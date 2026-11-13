#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Append a provenance line for each file of a node in the content checkout, for interactive sessions (spec 04.8, 14.7).

Usage: python3 prov.py [--models claude-opus-5-5,codex:gpt-6-luna] [--audit codex:gpt-6-sol] [--reviewer LOGIN] [--prompt writer/v1] [--note TEXT] <slug> [<slug> ...]
The default of --models is the first model of the large class of registers/models.yaml.
The default of --audit is auditor_for of models.yaml for those models. When that auditor is not known, prov.py stops.
The default of --reviewer is AI_REVIEWER, else null.
"""
import argparse
import datetime
import hashlib
import json
import os
import re
import time

from lib import REPO, Wait, models

RUN_ID = os.environ.get("AI_RUN_ID") or time.strftime("s-%Y%m%d-%H%M%S-") + str(os.getpid())


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--models", default="", help="a comma list of backend:model or model ids")
    ap.add_argument("--audit", default="", help="backend:model of the auditor")
    ap.add_argument("--reviewer", default=os.environ.get("AI_REVIEWER") or None)
    ap.add_argument("--prompt", default="writer/v1", help="the prompt version, role/v<n>")
    ap.add_argument("--note", default="full page, fixed after the audit and checked by a second audit")
    ap.add_argument("slugs", nargs="+")
    a = ap.parse_args()
    reg = models()
    ms = [reg.ref(x) for x in a.models.split(",") if x] or [reg.ref("large")]
    if a.audit:
        audit = reg.ref(a.audit)
    else:
        try:
            audit = ":".join(reg.auditor_for(ms, is_available=lambda *_: True))
        except Wait as w:
            raise SystemExit(f"give --audit: {w}") from None
    today = datetime.date.today().isoformat()
    for s in a.slugs:
        hits = list((REPO / "nodes").glob(f"*/*/{s}/node.md"))
        if not hits:
            raise SystemExit(f"no node {s}")
        d = hits[0].parent
        m = re.search(r"^tag: (\S+)", hits[0].read_text(encoding="utf-8"), re.M)
        if not m or m.group(1).startswith("NEW-"):
            raise SystemExit(f"{s} has no minted tag; run aik mint first")
        tag = m.group(1)
        files = [p for p in sorted(d.rglob("*")) if p.is_file() and "__pycache__" not in p.parts and p.suffix != ".pyc"]
        (REPO / "prov").mkdir(exist_ok=True)
        with (REPO / "prov" / f"{tag}.jsonl").open("a", encoding="utf-8") as f:
            for p in files:
                h = hashlib.sha256(p.read_bytes()).hexdigest()
                line = {"date": today, "file": str(p.relative_to(d)), "op": "edit", "hash": f"sha256:{h}",
                        "author": {"agent": "writer", "models": ms, "harness": "ai-agent/work/prov.py", "prompt": a.prompt,
                                   "run": RUN_ID, "models_version": reg.version},
                        "audit": audit, "reviewer": a.reviewer, "note": a.note, "sources": []}
                f.write(json.dumps(line, separators=(",", ":"), ensure_ascii=False) + "\n")
        print(s, tag, len(files))


if __name__ == "__main__":
    main()
