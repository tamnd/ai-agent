#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Merge the canon additions into the catalog, set the core flag by the rule of spec 5.10, and write registers/core.yaml (spec 14.3 step 6).

Usage:
  python3 finalize.py merge     # before stub.py: additions into curated.jsonl; core when two or more canon works name the item, or by the frontier-core rule
  python3 finalize.py core      # after stub.py: write registers/core.yaml in the content checkout, with the reason for each item

The frontier-core rule (spec 5.10): the AI Index of the current year names the item, or two or more surveys of registers/surveys.yaml do, or two or more courses do.
A row of surveys.yaml has a type: ai-index, survey (the default) or course.
"""
import json
import sys

from curate import key
from lib import REPO, WORK, front_matter, load_yaml, models

CUR = WORK / "catalog" / "curated.jsonl"
ADD = WORK / "catalog" / "additions.jsonl"
REFS = WORK / "catalog" / "canon_refs.json"


def jsonl(path):
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]


def sources():
    """(the canon work ids in register order, {frontier id: type})."""
    canon = [w["id"] for w in (load_yaml(REPO / "registers" / "canon.yaml").get("works") or [])]
    front = {}
    p = REPO / "registers" / "surveys.yaml"
    if p.exists():
        d = load_yaml(p)
        rows = d.get("works", d.get("surveys", [])) if isinstance(d, dict) else d
        for w in rows or []:
            if isinstance(w, dict) and w.get("id"):
                front[w["id"]] = w.get("type", "survey")
    return canon, front


def why(refs, canon, front):
    """The reason an item is core, or None (spec 5.10)."""
    c = [r for r in canon if r in refs]
    if len(c) >= 2:
        return f"core: {len(c)} canon works", c
    by = {}
    for r in refs:
        if r in front:
            by.setdefault(front[r], []).append(r)
    if by.get("ai-index"):
        return "frontier core: the AI Index", by["ai-index"]
    for t, label in (("survey", "surveys"), ("course", "courses")):
        if len(by.get(t, [])) >= 2:
            return f"frontier core: {len(by[t])} {label}", sorted(by[t])
    return None


def merge():
    items = jsonl(CUR)
    if not items:
        raise SystemExit(f"{CUR} is empty or missing; run curate.py first")
    have = {it["slug"] for it in items} | {key(it["slug"]) for it in items}
    n = 0
    for a in jsonl(ADD):
        if a["slug"] in have or key(a["slug"]) in have:
            continue
        items.append(a)
        have |= {a["slug"], key(a["slug"])}
        n += 1
    refs = json.loads(REFS.read_text(encoding="utf-8")) if REFS.exists() else {}
    canon, front = sources()
    for it in items:
        it["refs"] = refs.get(it["slug"], [])
        w = why(it["refs"], canon, front)
        it["core"] = bool(w) or bool(it.get("core") and it.get("universe"))
        if w:
            it["core_reason"] = w[0]
    items.sort(key=lambda x: (x["area"], x["slug"]))
    with open(CUR, "w", encoding="utf-8") as f:
        for it in items:
            f.write(json.dumps(it, ensure_ascii=False) + "\n")
    print(f"{n} additions merged; {len(items)} items; {sum(bool(it['core']) for it in items)} core")


def core():
    refs = json.loads(REFS.read_text(encoding="utf-8")) if REFS.exists() else {}
    canon, front = sources()
    rows = []
    for p in sorted((REPO / "nodes").glob("*/*/*/node.md")):
        fm = front_matter(p)
        slug, tag = fm.get("slug") or p.parent.name, fm.get("tag")
        w = why(refs.get(slug, []), canon, front)
        if w and tag:
            rows.append((slug, tag, w))
    out = ["# The core of spec 5.10: the items that two or more canon works name, and the frontier core.",
           "# finalize.py core of tamnd/ai-agent/work writes this file from the canon match. A person approves it at the end of M1 (spec 21).",
           f"version: core-{models().version.split('-', 1)[-1]}", "items:"]
    for slug, tag, (reason, r) in rows:
        out.append(f"  - node: {tag}  # {slug}")
        out.append(f"    reason: \"{reason}\"")
        out.append(f"    refs: [{', '.join(r)}]")
    if not rows:
        out[-1] = "items: []"
    (REPO / "registers" / "core.yaml").write_text("\n".join(out) + "\n", encoding="utf-8")
    print(f"{len(rows)} core rows in registers/core.yaml")


if __name__ == "__main__":
    cmds = {"merge": merge, "core": core}
    if len(sys.argv) != 2 or sys.argv[1] not in cmds:
        raise SystemExit("usage: python3 finalize.py merge|core")
    cmds[sys.argv[1]]()
