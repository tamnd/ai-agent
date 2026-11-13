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
