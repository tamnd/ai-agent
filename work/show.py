#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Print a short view of raw catalog files: the count, then each slug with its kind, a star for the core and a mark for one-run items.

Usage: python3 show.py catalog/raw/<top>--<sub>.json [...]
"""
import json
import sys

for f in sys.argv[1:]:
    with open(f, encoding="utf-8") as fh:
        d = json.load(fh)
    print(f, len(d))
    print(", ".join(x["slug"] + "(" + x["kind"][:4] + ("*" if x.get("core") else "") + ("?" if x.get("flag") else "") + ")" for x in d))
