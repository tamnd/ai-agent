#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Run the writer gate on nodes (spec 14.6): the local checks of writer.py, then `aik gate -json <slug>`.

Usage: python3 gate.py [--final] <slug> [<slug> ...]
--final also runs `aik cite <slug>`, as writer.py does after the audit passes.
Exit 1 when a node has a problem.
"""
import argparse
import sys

from lib import need_aik
from writer import gate, load_catalog


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--final", action="store_true", help="also run the citation check")
    ap.add_argument("slugs", nargs="+")
    a = ap.parse_args()
    need_aik("gate", "14.6")
    if a.final:
        need_aik("cite", "14.6")
    items = load_catalog()
    bad = 0
    for s in a.slugs:
        if s not in items:
            print(f"{s}: FAIL")
            print("  - no node with this slug in the catalog or the content checkout")
            bad += 1
            continue
        probs = gate(items[s], final=a.final)
        print(f"{s}: {'PASS' if not probs else 'FAIL'}")
        for p in probs:
            print("  -", p)
        bad += bool(probs)
    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
