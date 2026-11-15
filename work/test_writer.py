#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Tests of writer.py: a gate that fails for a reason of the machine blocks the item, and AIK_GATE_FLAGS.

Usage: python3 -m unittest discover -s work -p 'test_*.py'
The tests call no model and no aik.
"""
import os
import unittest
from unittest import mock

import writer
from lib import Wait


class GateTest(unittest.TestCase):
    item = {"slug": "q-learning", "kind": "method"}

    def run_gate(self, rc, out, env=None):
        calls = []

        def fake_aik(*args, **kw):
            calls.append(args)
            return rc, out
        with mock.patch.object(writer, "aik", fake_aik), mock.patch.object(writer, "local_checks", lambda item: []), \
                mock.patch.dict(os.environ, env or {}, clear=False):
            return writer.gate(self.item), calls

    def test_docker_down_waits(self):
        with self.assertRaises(Wait):
            self.run_gate(1, "aik test fails:\nthe tests run in docker containers with the images of manifests/images.yaml")

    def test_page_failure_is_a_problem(self):
        probs, _ = self.run_gate(1, "not json: the section Setting is missing")
        self.assertTrue(probs)

    def test_flags(self):
        _, calls = self.run_gate(0, "[]", {"AIK_GATE_FLAGS": "-unsafe-local -no-exp"})
        self.assertEqual(calls[0], ("gate", "-json", "-unsafe-local", "-no-exp", "q-learning"))


class ClimbTest(unittest.TestCase):
    """A climb to a writer of a new family would leave no auditor of another family; it is skipped."""
    ladder = ["codex:gpt-6-luna", "claude:claude-haiku-5-5", "codex:gpt-6-sol"]
    st = {"history": [{"role": "write", "backend": "codex", "model": "gpt-6-luna"}]}

    def climb(self, auditor_ok):
        class M:
            def auditor_for(self, ws):
                if not auditor_ok({w.split(":")[0] for w in ws}):
                    raise Wait("no auditor")
                return "claude", "claude-sonnet-5-5"
        with mock.patch.object(writer, "available", lambda b, m: True), mock.patch.object(writer, "models", lambda: M()):
            return writer.climb(self.ladder, 0, self.st, (0, "codex", "gpt-6-luna"))

    def test_skips_a_new_family(self):
        self.assertEqual(self.climb(lambda fams: len(fams) < 2), (2, "codex", "gpt-6-sol"))

    def test_stays_when_no_rung_fits(self):
        self.assertEqual(self.climb(lambda fams: False), (0, "codex", "gpt-6-luna"))

    def test_takes_the_next_rung_when_it_fits(self):
        self.assertEqual(self.climb(lambda fams: True), (1, "claude", "claude-haiku-5-5"))


class RunRecordTest(unittest.TestCase):
    """Each gate is kept in the state, and a batch writes its run record with the totals of its calls."""

    def test_gated_records(self):
        st, saved = {"slug": "q", "history": []}, []
        with mock.patch.object(writer, "gate", lambda item, final=False: ["E01 a problem"] if not final else []), \
                mock.patch.object(writer, "save", saved.append):
            self.assertEqual(writer.gated({"slug": "q"}, st), ["E01 a problem"])
            self.assertEqual(writer.gated({"slug": "q"}, st, final=True), [])
        self.assertEqual(st["gates"], [{"ok": False, "n": 1, "run": writer.RUN_ID},
                                       {"ok": True, "n": 0, "run": writer.RUN_ID, "final": True}])
        self.assertEqual(len(saved), 2)

    def test_totals_count_this_run(self):
        sts = [{"history": [
            {"role": "write", "backend": "codex", "model": "m1", "tokens_in": 10, "tokens_out": 5, "cost": 0.1, "run": writer.RUN_ID},
            {"role": "audit", "backend": "claude", "model": "m2", "tokens_in": 3, "tokens_out": None, "cost": None, "run": writer.RUN_ID},
            {"role": "write", "backend": "codex", "model": "m1", "tokens_in": 99, "tokens_out": 99, "cost": 9.0, "run": "an-old-run"}]},
            {"history": [{"role": "fix-gate", "backend": "codex", "model": "m1", "tokens_in": 1, "tokens_out": 1, "cost": 0.05,
                          "run": writer.RUN_ID}]}]
        self.assertEqual(writer.run_totals(sts), {
            "claude:m2": {"calls": 1, "tokens_in": 3, "tokens_out": 0, "cost": 0.0},
            "codex:m1": {"calls": 2, "tokens_in": 11, "tokens_out": 6, "cost": 0.15}})

    def test_rates_skip_an_old_aik(self):
        with mock.patch.object(writer, "aik_has", lambda cmd, flag="": False):
            self.assertIsNone(writer.run_rates())

    def test_rates_parse(self):
        calls = []

        def fake_aik(*args, **kw):
            calls.append(args)
            return 0, '{"run": "r", "kinds": [], "total": {"kind": "total", "items": 0}}'
        with mock.patch.object(writer, "aik_has", lambda cmd, flag="": True), mock.patch.object(writer, "aik", fake_aik):
            self.assertEqual(writer.run_rates()["total"]["kind"], "total")
        self.assertEqual(calls[0][:2], ("report", "rates"))
        self.assertIn("-run", calls[0])



class BudgetTest(unittest.TestCase):
    """The caps of 14.8: by kind, the override, the batch, and the estimate of a call with tokens and no cost."""

    def test_caps(self):
        b = writer.Budget()
        self.assertEqual(b.cap({"kind": "method"}), 3.0)
        self.assertEqual(b.cap({"kind": "theorem"}), 8.0)
        self.assertEqual(b.cap({"kind": "model", "promoted": True}), 1.0)
        self.assertEqual(writer.Budget(item=5).cap({"kind": "theorem"}), 5)

    def test_estimate(self):
        b = writer.Budget(prices={"gpt-6-luna": {"in": 2.0, "out": 8.0}})
        self.assertAlmostEqual(b.cost({"model": "gpt-6-luna", "cost": None, "tokens_in": 1_000_000, "tokens_out": 500_000}), 6.0)
        self.assertEqual(b.cost({"model": "m", "cost": 0.5}), 0.5)
        self.assertIsNone(b.cost({"model": "other", "cost": None, "tokens_in": 10}))
        b.add({"model": "other", "cost": None})
        b.add({"model": "m", "cost": 0.25})
        self.assertEqual((b.spent, b.unpriced), (0.25, 1))

    def test_item_over(self):
        b = writer.Budget()
        st = {"history": [{"model": "m", "cost": 2.0}, {"model": "m", "cost": 1.5}]}
        with self.assertRaises(writer.OverBudget):
            b.check({"kind": "method"}, st)
        b.check({"kind": "theorem"}, st)

    def test_batch_over(self):
        b = writer.Budget(batch=1.0)
        b.add({"model": "m", "cost": 1.2})
        self.assertTrue(b.over())
        with self.assertRaises(writer.OverBudget):
            b.check({"kind": "theorem"}, {"history": []})

    def test_over_budget_blocks(self):
        st, saved = {"slug": "q", "status": "todo", "history": [{"model": "m", "cost": 9.0}]}, []
        with mock.patch.object(writer, "state", lambda slug: st), mock.patch.object(writer, "save", saved.append), \
                mock.patch.object(writer, "log", lambda ev: None), mock.patch.object(writer, "BUDGET", writer.Budget()):
            self.assertEqual(writer.process({"slug": "q", "kind": "method"}, []), "blocked")
        self.assertTrue(st["blocked"].startswith("budget:"))


if __name__ == "__main__":
    unittest.main()
