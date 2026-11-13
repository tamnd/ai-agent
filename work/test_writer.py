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


if __name__ == "__main__":
    unittest.main()


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
