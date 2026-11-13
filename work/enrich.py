#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Review the canon additions: drop the ones that are not items, and write a summary for the others (spec 14.3 step 5).

Usage: python3 enrich.py [--model claude-sonnet-5-5]
Rewrites catalog/additions.jsonl in place, and keeps a copy as catalog/additions.raw.jsonl.
"""
import argparse
import json
import shutil

from lib import WORK, extract_json, log, models, run_model, split_ref

ADD = WORK / "catalog" / "additions.jsonl"
RAW = WORK / "catalog" / "additions.raw.jsonl"

PROMPT = """You review new entries for ai.tamnd.com, a reference of artificial intelligence with one page or one card per concept, definition, method, architecture, task, metric, theorem, finding, argument, open problem or risk.

Each entry below came from the table of contents of a canon work or a frontier source. For each one decide:
- keep: true when the entry is a real, named item that deserves a page or a card of its own. false when it is only a book section, a comparison, a discussion, a tip, an example, an exercise without a standard name, or a topic that belongs to a neighbour field (mathematics, algorithms or programming) with no AI view.
- summary: for a kept entry, one plain English sentence of at most 25 words that says what the item is or does. Use no dash as punctuation. Do not start with the name. Give no number, no rank and no judgement of quality.

Entries (index | slug | kind | name | where):
{items}

Reply with ONLY a JSON array: [{{"i": <index>, "keep": true|false, "summary": "..."}}]
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", default="", help="the default is the curator role of models.yaml")
    args = ap.parse_args()
    ref = args.model or models().role("curator")[0]
    backend, model = split_ref(ref)
    if not ADD.exists():
        raise SystemExit(f"{ADD} does not exist; run match.py first")
    if not RAW.exists():
        shutil.copy(ADD, RAW)
    items = [json.loads(ln) for ln in RAW.read_text(encoding="utf-8").splitlines() if ln.strip()]
    out = []
    for start in range(0, len(items), 60):
        chunk = items[start:start + 60]
        txt = "\n".join(f"{n} | {a['slug']} | {a['kind']} | {a['name']} | {a['note']}" for n, a in enumerate(chunk))
        r = run_model(PROMPT.format(items=txt), backend, model, job="enrich", role="curator", write=False, timeout=1200, max_turns=2)
        try:
            ans = {x["i"]: x for x in extract_json(r["text"]) if isinstance(x, dict) and "i" in x}
        except ValueError:
            print("no JSON at", start)
            continue
        for n, a in enumerate(chunk):
            d = ans.get(n)
            if d and d.get("keep") and d.get("summary"):
                a["note"] = d["summary"].replace("\u2014", ",").replace("\u2013", "-").strip()
                out.append(a)
        log({"job": "enrich", "n": len(chunk), "model": ref})
        print(f"{start + len(chunk)}/{len(items)}: kept {len(out)}", flush=True)
    with open(ADD, "w", encoding="utf-8") as f:
        for a in out:
            f.write(json.dumps(a, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
