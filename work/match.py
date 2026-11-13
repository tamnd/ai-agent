#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Match the canon and frontier inventories to the curated catalog (spec 14.3 step 4).

Usage: python3 match.py [--model claude-sonnet-5-5]
Writes:
  catalog/canon_refs.json        slug to the sorted list of works that name it
  catalog/additions.jsonl        canon items that the catalog does not have (in the curated format)
  catalog/match_decisions.json   the cache of the model decisions
A canon item matches by name, alias or same_as id. A model decides the doubtful pairs.
"""
import argparse
import json
import re
from collections import defaultdict

from curate import key, slugify, tree, valid_areas
from kinds import DEFAULT_FORM, IDEA_KINDS, KINDS, LEVELS
from lib import REPO, WORK, extract_json, front_matter, log, models, run_model, split_ref

CANON = WORK / "catalog" / "canon"
CUR = WORK / "catalog" / "curated.jsonl"
REFS_OUT = WORK / "catalog" / "canon_refs.json"
ADD = WORK / "catalog" / "additions.jsonl"
DEC = WORK / "catalog" / "match_decisions.json"
STOP = {"the", "a", "an", "of", "for", "with", "and", "in", "on", "to", "by", "via", "using", "s"}


def tokens(s):
    return {t for t in re.split(r"[^a-z0-9]+", s.lower().replace("'s", "")) if t and t not in STOP}


def targets():
    out = {}
    if CUR.exists():
        for line in CUR.read_text(encoding="utf-8").splitlines():
            if line.strip():
                it = json.loads(line)
                out[it["slug"]] = it
    for p in (REPO / "nodes").glob("*/*/*/node.md"):
        fm = front_matter(p)
        s = fm.get("slug") or p.parent.name
        if s not in out:
            out[s] = {"slug": s, "name": fm.get("name", ""), "kind": fm.get("kind", ""), "aliases": fm.get("aliases") or [],
                      "area": str(p.parent.parent.relative_to(REPO / "nodes")), "same_as": fm.get("same_as") or {}}
    return out


def canon_items():
    rows = []
    for f in sorted(CANON.glob("*.json")):
        ref = f.stem.split("--")[0]
        for it in json.loads(f.read_text(encoding="utf-8")):
            if isinstance(it, dict) and it.get("name"):
                it["ref"] = ref
                rows.append(it)
    return rows


ASK = """You match items from the tables of contents of the canon works and the frontier sources of AI to the catalog of ai.tamnd.com.

For each item below, the candidates are catalog entries with similar names (slug | kind | area | name). Decide one outcome:
- "match": the item is the same thing as a candidate (give its slug). A different name for the same thing is a match. A small variant that a page would cover in its "Variants" section is also a match to the main item.
- "new": the catalog does not have it, and it is a real item of AI. Give slug (kebab-case common English name), name, kind ({kinds}), area (top/sub from the tree) and level (intro, core, advanced, research).
- "skip": not an item (a chapter heading, a person, an example, a vague word).
Do not judge which item is better. Decide only identity and home.

The area tree:
{tree}

Items:
{items}

Reply with ONLY a JSON array, one object per item, in order: {{"i": <n>, "outcome": "match|new|skip", "slug": "...", "name": "...", "kind": "...", "area": "...", "level": "..."}} (name, kind, area and level only for "new").
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", default="", help="the default is the curator role of models.yaml")
    args = ap.parse_args()
    ref = args.model or models().role("curator")[0]
    backend, model = split_ref(ref)
    tg = targets()
    idx, same = {}, {}
    tok_idx = defaultdict(set)
    for s, it in tg.items():
        for n in [s, it.get("name", ""), *it.get("aliases", [])]:
            if n:
                idx.setdefault(key(n), s)
                for t in tokens(n):
                    tok_idx[t].add(s)
        for src, v in (it.get("same_as") or {}).items():
            same[f"{src}:{v}"] = s
    refs = defaultdict(set)
    unmatched = {}
    for c in canon_items():
        hit = next((same[f"{k}:{v}"] for k, v in (c.get("same_as") or {}).items() if f"{k}:{v}" in same), None)
        for n in [c["name"], *(c.get("aliases") or [])]:
            if hit:
                break
            if isinstance(n, str) and key(n) in idx:
                hit = idx[key(n)]
        if hit:
            refs[hit].add(c["ref"])
        else:
            k = key(c["name"])
            u = unmatched.setdefault(k, {"name": c["name"], "kind": c.get("kind"), "refs": set(), "where": []})
            u["refs"].add(c["ref"])
            u["where"].append(f"{c['ref']}: {c.get('where', '')}")
    print(f"{sum(len(v) for v in refs.values())} ref links on {len(refs)} slugs; {len(unmatched)} unmatched names")
    dec = json.loads(DEC.read_text(encoding="utf-8")) if DEC.exists() else {}
    todo = [k for k in unmatched if k not in dec]
    tr = tree()
    for start in range(0, len(todo), 40):
        chunk = todo[start:start + 40]
        lines = []
        for n, k in enumerate(chunk):
            u = unmatched[k]
            score = defaultdict(int)
            for t in tokens(u["name"]):
                if len(tok_idx[t]) < 400:
                    for s in tok_idx[t]:
                        score[s] += 1
            cands = sorted(score, key=lambda s: (-score[s], len(s)))[:10]
            lines.append(f"Item {n}: {u['name']} ({u['kind']}; {'; '.join(u['where'][:2])})")
            for s in cands:
                lines.append(f"    {s} | {tg[s].get('kind', '')} | {tg[s].get('area', '')} | {tg[s].get('name', '')}")
        r = run_model(ASK.format(tree=tr, kinds=", ".join(IDEA_KINDS), items="\n".join(lines)), backend, model,
                      job="match", role="curator", write=False, timeout=1800, max_turns=2)
        try:
            ans = extract_json(r["text"])
        except ValueError:
            print("no JSON", r.get("err", "")[:200])
            continue
        for a in ans:
            if isinstance(a, dict) and isinstance(a.get("i"), int) and a["i"] < len(chunk):
                dec[chunk[a["i"]]] = a
        DEC.write_text(json.dumps(dec, indent=1, ensure_ascii=False), encoding="utf-8")
        log({"job": "match", "n": len(chunk), "model": ref})
        print(f"decided {start + len(chunk)}/{len(todo)}", flush=True)
    adds = {}
    for k, u in unmatched.items():
        d = dec.get(k)
        if not d:
            continue
        if d.get("outcome") == "match" and d.get("slug") in tg:
            refs[d["slug"]] |= u["refs"]
        elif d.get("outcome") == "new":
            s = slugify(d.get("slug") or u["name"])
            if s in tg:
                refs[s] |= u["refs"]
                continue
            kind = d.get("kind") if d.get("kind") in KINDS else (u["kind"] if u["kind"] in KINDS else "concept")
            a = adds.setdefault(s, {"slug": s, "name": d.get("name") or u["name"], "kind": kind,
                                    "level": d.get("level") if d.get("level") in LEVELS else "core",
                                    "area": d.get("area"), "core": False, "aliases": [], "prereqs": [],
                                    "form": DEFAULT_FORM[kind], "outcome": DEFAULT_FORM[kind],
                                    "sources": [f"canon:{r}" for r in sorted(u["refs"])],
                                    "note": f"Covered in {'; '.join(u['where'][:3])}."})
            if u["name"] != a["name"] and u["name"] not in a["aliases"]:
                a["aliases"].append(u["name"])
            refs[s] |= u["refs"]
    valid = valid_areas()
    adds = {s: a for s, a in adds.items() if a["area"] in valid}
    ADD.parent.mkdir(parents=True, exist_ok=True)
    with open(ADD, "w", encoding="utf-8") as f:
        for a in adds.values():
            f.write(json.dumps(a, ensure_ascii=False) + "\n")
    REFS_OUT.write_text(json.dumps({s: sorted(v) for s, v in sorted(refs.items())}, indent=1), encoding="utf-8")
    print(f"{len(adds)} additions; {sum(1 for v in refs.values() if len(v) >= 2)} slugs in two or more works")


if __name__ == "__main__":
    main()
