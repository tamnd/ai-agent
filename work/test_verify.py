#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Tests of verify.py: the white space of a passage is collapsed before the cut, and the items are sent in batches that
fit the verifier's input limit, with the same prompt, and their verdicts written to the same records.

Usage: python3 -m unittest discover -s work -p 'test_*.py'
The tests call no model and no aik.
"""
import json
import re
import unittest
from unittest import mock

import verify


def item(n, passage="The passage says it.", section=""):
    return {"tag": "t", "claim": f"c-{n}", "type": "fact", "text": f"Claim {n}.", "work": "w", "version": "v1",
            "locator": f"Article {n}", "key": f"k{n}", "spans": [[0, 1]], "passage": passage, "section": section}


class CollapseTest(unittest.TestCase):
    def test_collapse(self):
        self.assertEqual(verify.collapse("  a \t  b  \n\n\n\n  c  d \n"), "a b\n\nc d")

    def test_cut_after_collapse(self):
        # The white space of a web page does not use up the room of the passage.
        words = ("word" + " " * 20) * 2000
        text = verify.item_text(1, item(1, passage=words))
        self.assertNotIn("is cut here", text)
        self.assertIn("word word", text)
        long = "word " * 6000
        text = verify.item_text(1, item(1, passage=long, section="the section"))
        self.assertIn("[the passage is cut here]", text)
        self.assertNotIn("the section", text)
        text = verify.item_text(1, item(1, passage="short", section="sect   ion " * 5000))
        self.assertIn("[the section is cut here]", text)
        self.assertIn("sect ion", text)


class BatchTest(unittest.TestCase):
    template = "Judge these.\n\n{items}\n\nReply [{{\"id\": 1}}]"

    def test_one_batch(self):
        ps = [item(i) for i in range(5)]
        self.assertEqual(verify.batches(ps, self.template), [ps])

    def test_batches_fit(self):
        ps = [item(i, passage=f"passage {i} " + "x" * 20000) for i in range(12)]
        limit = 70_000
        runs = verify.batches(ps, self.template, limit)
        self.assertGreater(len(runs), 1)
        self.assertEqual([p for b in runs for p in b], ps)  # every item once, in order
        for b in runs:
            prompt = self.template.format(items="\n\n".join(verify.item_text(i + 1, p) for i, p in enumerate(b)))
            self.assertLessEqual(len(prompt), limit)

    def test_verify_in_batches(self):
        ps = [item(i, passage=f"passage {i} " + "y" * 23000) for i in range(100)]  # 2.3 million characters
        prompts, written = [], []

        def run_model(prompt, backend, model, **kw):
            prompts.append(prompt)
            n = len(re.findall(r"^## Item \d+$", prompt, re.M))
            return {"text": json.dumps([{"id": i, "verdict": "supported", "quote": "q", "note": ""} for i in range(1, n + 1)])}

        def aik(sub, *args):
            if sub == "passages":
                return json.dumps(ps)
            if sub == "verdict":
                f = args[args.index("-file") + 1]
                with open(f, encoding="utf-8") as fh:
                    written.append((json.load(fh), args[args.index("-run") + 1]))
                return "wrote verdicts"
            raise AssertionError(sub)

        with mock.patch.object(verify, "aik", aik), mock.patch.object(verify, "run_model", run_model), \
                mock.patch.object(verify, "log"), mock.patch("builtins.print"):
            bad, items = verify.verify("eu-ai-act", "codex:gpt-6-sol", ["claude:claude-opus-5-5"], False, False)
        self.assertEqual(bad, 0)
        self.assertGreater(len(prompts), 1)
        for p in prompts:
            self.assertLess(len(p), 1_048_576)
            self.assertIn("You are the verifier", p)
        self.assertEqual([it["key"] for it in items], [p["key"] for p in ps])
        self.assertEqual([v["key"] for w, _ in written for v in w], [p["key"] for p in ps])
        self.assertEqual({run for _, run in written}, {verify.RUN_ID})

    def test_failed_batch_keeps_the_others(self):
        ps = [item(i, passage="z" * 20000) for i in range(6)]
        calls = []

        def run_model(prompt, backend, model, **kw):
            calls.append(prompt)
            if len(calls) == 1:
                return {"text": "no JSON here"}
            n = len(re.findall(r"^## Item \d+$", prompt, re.M))
            return {"text": json.dumps([{"id": i, "verdict": "supported"} for i in range(1, n + 1)])}

        written = []

        def aik(sub, *args):
            if sub == "passages":
                return json.dumps(ps)
            with open(args[args.index("-file") + 1], encoding="utf-8") as fh:
                written.extend(json.load(fh))
            return ""

        with mock.patch.object(verify, "aik", aik), mock.patch.object(verify, "run_model", run_model), \
                mock.patch.object(verify, "log"), mock.patch.object(verify, "MAX_PROMPT", 50_000), \
                mock.patch("builtins.print"):
            bad, items = verify.verify("x", "codex:gpt-6-sol", ["claude:claude-opus-5-5"], False, False)
        self.assertGreater(len(calls), 1)
        first = len(verify.batches(ps, (verify.WORK / "prompts" / "verify" / "v1.md").read_text(encoding="utf-8"), 50_000)[0])
        self.assertEqual(bad, first)
        self.assertEqual(len(items), len(ps) - first)
        self.assertEqual(len(written), len(ps) - first)


if __name__ == "__main__":
    unittest.main()
