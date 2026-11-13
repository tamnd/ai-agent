#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Tests of lib.py and kinds.py: reading models.yaml, the auditor_for rule with its wait case, the event log, and the kind table.

Usage: python3 -m unittest discover -s work -p 'test_*.py'
The tests use a temporary models.yaml. They call no model and no network.
"""
import datetime
import json
import re
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import lib
import kinds
import report

MODELS = """version: m-test
classes:
  large:  [claude-opus-5-5, claude-fable-5-2]
  medium: [claude-sonnet-5-5]
  small:  [claude-haiku-5-5]
  other:
    writer: codex:gpt-6-luna
    auditor: codex:gpt-6-sol
writer_ladder: [other.writer, claude-haiku-5-5, claude-sonnet-5-5, claude-opus-5-5]
writer_ladder_hard: [claude-sonnet-5-5, claude-opus-5-5, claude-fable-5-2]
auditor_for:
  claude-*: other.auditor
  codex:*: claude-sonnet-5-5
  fallback_if_other_unavailable: claude-opus-5-5
verifier: other.auditor
cataloguer: [claude-opus-5-5, other.auditor]
curator: claude-sonnet-5-5
"""

SOL = ("codex", "gpt-6-sol")
SONNET = ("claude", "claude-sonnet-5-5")
OPUS = ("claude", "claude-opus-5-5")


def everything(*_):
    return True


def without(*down):
    """An availability function where the given backends are down."""
    return lambda backend, model="": backend not in down


@unittest.skipIf(lib.yaml is None, "PyYAML is not installed")
class ModelsTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.path = Path(self.dir.name) / "models.yaml"
        self.path.write_text(MODELS, encoding="utf-8")
        self.m = lib.models(self.path)

    def tearDown(self):
        lib._models_cache.clear()
        self.dir.cleanup()

    def test_version(self):
        self.assertEqual(self.m.version, "m-test")

    def test_resolve(self):
        self.assertEqual(self.m.resolve("other.writer"), ("codex", "gpt-6-luna"))
        self.assertEqual(self.m.resolve("claude-haiku-5-5"), ("claude", "claude-haiku-5-5"))
        self.assertEqual(self.m.resolve("codex:gpt-6-sol"), SOL)
        self.assertEqual(self.m.resolve("large"), OPUS)
        with self.assertRaises(SystemExit):
            self.m.resolve("other.critic")
        with self.assertRaises(SystemExit):
            self.m.resolve("gemini:x")

    def test_ladders(self):
        self.assertEqual(self.m.ladder(), ["codex:gpt-6-luna", "claude:claude-haiku-5-5", "claude:claude-sonnet-5-5", "claude:claude-opus-5-5"])
        self.assertEqual(self.m.ladder(hard=True), ["claude:claude-sonnet-5-5", "claude:claude-opus-5-5", "claude:claude-fable-5-2"])

    def test_roles(self):
        self.assertEqual(self.m.role("cataloguer"), ["claude:claude-opus-5-5", "codex:gpt-6-sol"])
        self.assertEqual(self.m.role("curator"), ["claude:claude-sonnet-5-5"])
        self.assertEqual(self.m.role("verifier"), ["codex:gpt-6-sol"])
        with self.assertRaises(SystemExit):
            self.m.role("translator")

    def test_claude_writer_gets_the_other_family(self):
        self.assertEqual(self.m.auditor_for("claude:claude-haiku-5-5", everything), SOL)
        self.assertEqual(self.m.auditor_for(["claude:claude-sonnet-5-5", "claude:claude-opus-5-5"], everything), SOL)

    def test_codex_writer_gets_claude(self):
        self.assertEqual(self.m.auditor_for("codex:gpt-6-luna", everything), SONNET)

    def test_claude_writer_waits_when_codex_is_down(self):
        # algorithm-work gave this audit to Claude Opus; spec 14.4 says the item waits
        with self.assertRaises(lib.Wait):
            self.m.auditor_for("claude:claude-sonnet-5-5", without("codex"))

    def test_codex_writer_falls_back_when_claude_primary_is_down(self):
        down = lambda b, m="": (b, m) != SONNET  # noqa: E731
        self.assertEqual(self.m.auditor_for("codex:gpt-6-luna", down), OPUS)

    def test_codex_writer_waits_when_claude_is_down(self):
        with self.assertRaises(lib.Wait):
            self.m.auditor_for("codex:gpt-6-luna", without("claude"))

    def test_mixed_writers_wait(self):
        # a climb from the codex rung to a Claude rung leaves no auditor of another family (spec 15.4, Q-08)
        with self.assertRaises(lib.Wait):
            self.m.auditor_for(["codex:gpt-6-luna", "claude:claude-haiku-5-5"], everything)

    def test_no_writer_waits(self):
        with self.assertRaises(lib.Wait):
            self.m.auditor_for([], everything)

    def test_never_the_writer_family(self):
        for hard in (False, True):
            for rung in self.m.ladder(hard):
                for avail in (everything, without("codex"), without("claude")):
                    try:
                        b, _ = self.m.auditor_for(rung, avail)
                    except lib.Wait:
                        continue
                    self.assertNotEqual(b, lib.family(rung), f"{rung} audited by its own family")

    def test_missing_file_stops(self):
        with self.assertRaises(SystemExit):
            lib.models(Path(self.dir.name) / "none.yaml")

    def test_no_classes_stops(self):
        p = Path(self.dir.name) / "bad.yaml"
        p.write_text("version: x\n", encoding="utf-8")
        with self.assertRaises(SystemExit):
            lib.models(p)


class YamlTest(unittest.TestCase):
    def test_need_yaml_stops_without_pyyaml(self):
        with mock.patch.object(lib, "yaml", None):
            with self.assertRaises(SystemExit) as e:
                lib.need_yaml()
            self.assertIn("PyYAML", str(e.exception))


class LogTest(unittest.TestCase):
    def test_run_model_logs_tokens_and_cost(self):
        with tempfile.TemporaryDirectory() as d:
            reply = {"ok": True, "text": "[]", "secs": 3, "tokens_in": 1200, "tokens_out": 300, "cost": 0.042, "limited": False}
            with mock.patch.object(lib, "LOG", Path(d)), mock.patch.object(lib, "run_claude", return_value=reply), \
                    mock.patch.object(lib, "models", side_effect=SystemExit("no register")):
                lib.run_model("p", "claude", "claude-haiku-5-5", job="catalog", slug="x", role="cataloguer")
            ev = json.loads((Path(d) / "events.jsonl").read_text(encoding="utf-8").splitlines()[-1])
        self.assertEqual((ev["tokens_in"], ev["tokens_out"], ev["cost"]), (1200, 300, 0.042))
        self.assertEqual((ev["job"], ev["role"], ev["model"]), ("catalog", "cataloguer", "claude-haiku-5-5"))

    def test_unknown_backend_stops(self):
        with self.assertRaises(SystemExit):
            lib.run_model("p", "gemini", "x")


class AikTest(unittest.TestCase):
    HELP = "Usage:\n\n  aik <command> [flags]\n\nCommands:\n\n  new       make a node\n  check     run the rules\n  test      run tests\n\nPlanned, not built yet: agent, audit.\n"

    def fake(self, *args):
        if args == ("help",):
            return self.HELP
        if args == ("test", "-h"):
            return "Usage: aik test\n\nFlags:\n  -lang string\n    \tlanguages\n"
        return ""

    def test_missing_command_names_the_section(self):
        with mock.patch.object(lib, "_aik_text", side_effect=self.fake):
            self.assertTrue(lib.aik_has("check"))
            self.assertFalse(lib.aik_has("gate"))
            with self.assertRaises(lib.MissingCommand) as e:
                lib.need_aik("gate", "14.6")
            self.assertIn("aik gate", str(e.exception))
            self.assertIn("14.6", str(e.exception))

    def test_missing_flag(self):
        with mock.patch.object(lib, "_aik_text", side_effect=self.fake):
            self.assertTrue(lib.aik_has("test", "lang"))
            with self.assertRaises(lib.MissingCommand) as e:
                lib.need_aik("test", "14.6", flag="framework")
            self.assertIn("-framework", str(e.exception))


class KindsTest(unittest.TestCase):
    def test_28_kinds(self):
        self.assertEqual(len(kinds.KINDS), 28)
        self.assertEqual(kinds.HARD, {"theorem", "argument", "open-problem", "policy", "finding"})
        self.assertTrue(kinds.HARD <= set(kinds.KINDS))
        self.assertEqual(kinds.CARD_ONLY, {"result", "exercise"})

    def test_every_kind_has_gold_and_tasks(self):
        for k in kinds.KINDS:
            self.assertIn(k, kinds.EXTRA)
            self.assertTrue(kinds.sections_for(k))
            if k not in kinds.CARD_ONLY:
                self.assertTrue(kinds.GOLD_M0.get(k), k)

    def test_sections_match_aik(self):
        src = Path(__file__).resolve().parent.parent / "node" / "kinds.go"
        if not src.exists():
            self.skipTest("node/kinds.go is not in this checkout")
        text = src.read_text(encoding="utf-8")
        go = {}
        for m in re.finditer(r'ID: "([a-z-]+)".*?Form: "(\w+)"(.*?)\n\t\}', text, re.S):
            body = m.group(3)

            def field(name):
                f = re.search(name + r":\s*\[\]string\{(.*?)\}", body, re.S)
                return re.findall(r'"([^"]*)"', f.group(1)) if f else []

            go[m.group(1)] = (field("Sections"), field("Required"), field("Generated"), m.group(2))
        self.assertEqual(set(go), set(kinds.KINDS))
        for k, (secs, req, gen, form) in go.items():
            self.assertEqual(kinds.SECTIONS[k], (secs, req, gen), k)
            self.assertEqual(kinds.DEFAULT_FORM[k], form, k)


class ReportTest(unittest.TestCase):
    def test_next_patch(self):
        self.assertEqual(report.next_patch(["2026.12", "2027.03"]), "2027.03.1")
        self.assertEqual(report.next_patch(["2027.03", "2027.03.1", "2026.12.4"]), "2027.03.2")
        self.assertEqual(report.next_patch([], datetime.date(2026, 11, 13)), "2026.11")
        self.assertEqual(report.next_patch(["v1.2.3"], datetime.date(2026, 11, 13)), "2026.11")


if __name__ == "__main__":
    unittest.main()
