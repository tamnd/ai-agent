#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Write a stub for each curated item with the outcome node, sync the cards, then mint the tags (spec 14.3 step 7).

Usage: python3 stub.py [--dry-run]

For a node outcome with no node yet, `aik new -area <top/sub> -name <name> -level <level> <kind> <slug>` writes the directory with the sections of its kind. stub.py then puts the catalog note in the summary and adds the aliases.
For a card outcome, `aik card sync` writes or updates the card. aik does not have this command yet, so stub.py stops before it writes anything when there is a card outcome.
A bridge outcome needs no file here: the bridge card is already in cards/bridge/.
Then `aik fmt` and `aik mint` run in the content checkout. Set AI_AUTHOR to pass -author to aik mint.
Writes catalog/slugs.tsv: slug, kind, area and name of each node, card and bridge card.
"""
import argparse
import json
import os
import re

from lib import REPO, WORK, aik, front_matter, need_aik

CATALOG = WORK / "catalog" / "curated.jsonl"
SLUGS = WORK / "catalog" / "slugs.tsv"
MARKER = "Write one sentence that says what this is."


def sentences(text):
    text = re.sub(r"\s+", " ", text.replace("\u2014", ",").replace("\u2013", "-")).strip()
    if text and text[-1] not in ".!?":
        text += "."
    if text:
        text = text[0].upper() + text[1:]
    return text


def jsonl(path):
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]


def fill(path, it):
    """Put the note in the summary and add the aliases in the front matter that aik new wrote."""
    t = path.read_text(encoding="utf-8")
    t = t.replace(f"  {MARKER}", "  " + sentences(it.get("note") or it["name"]), 1)
    aliases = [a for a in it.get("aliases", []) if a and a != it["name"]][:8]
    if aliases:
        line = "aliases: [" + ", ".join(json.dumps(a, ensure_ascii=False) for a in aliases) + "]"
        t = re.sub(r"^(name: .*)$", lambda m: m.group(1) + "\n" + line, t, count=1, flags=re.M)
    path.write_text(t, encoding="utf-8")


def write_slugs():
    rows = []
    for p in sorted((REPO / "nodes").rglob("node.md")):
        fm = front_matter(p)
        area = str(p.parent.parent.relative_to(REPO / "nodes"))
        rows.append(f"{fm.get('slug', p.parent.name)}\t{fm.get('kind', '')}\t{area}\t{fm.get('name', '')}")
    for p in sorted((REPO / "cards").glob("*/*.jsonl")):
        if p.parent.name == "bridge":
            continue
        rows += [f"{r['slug']}\t{r.get('kind', p.parent.name)}\t{r.get('area', '')}\t{r.get('name', '')}" for r in jsonl(p) if r.get("slug")]
    for p in sorted((REPO / "cards" / "bridge").glob("*/*.jsonl")):
        rows += [f"{r['site']}:{r['slug']}\t{r.get('kind', '')}\t{r.get('area', '')}\t{r.get('name', '')}" for r in jsonl(p) if r.get("slug")]
    SLUGS.parent.mkdir(parents=True, exist_ok=True)
    SLUGS.write_text("\n".join(rows) + "\n", encoding="utf-8")
    return len(rows)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--dry-run", action="store_true", help="count the stubs and the cards and change no file")
    a = ap.parse_args()
    items = jsonl(CATALOG)
    if not items:
        raise SystemExit(f"{CATALOG} is empty or missing; run curate.py and finalize.py merge first")
    nodes = [it for it in items if it.get("outcome", "node") == "node" and not (REPO / "nodes" / it["area"] / it["slug"]).exists()]
    cards = [it for it in items if it.get("outcome") == "card"]
    bridge = [it for it in items if it.get("outcome") == "bridge"]
    print(f"{len(nodes)} stubs, {len(cards)} cards, {len(bridge)} bridge items")
    if a.dry_run:
        return
    if cards:
        need_aik("card sync", "14.3")
    made = 0
    for it in nodes:
        rc, out = aik("new", "-area", it["area"], "-name", it["name"], "-level", it.get("level", "core"), it["kind"], it["slug"])
        if rc != 0:
            print(f"{it['slug']}: aik new failed: {out.strip()[-300:]}")
            continue
        fill(REPO / "nodes" / it["area"] / it["slug"] / "node.md", it)
        made += 1
    if cards:
        path = WORK / "catalog" / "cards.jsonl"
        with open(path, "w", encoding="utf-8") as f:
            for it in cards:
                f.write(json.dumps(it, ensure_ascii=False) + "\n")
        rc, out = aik("card", "sync", str(path))
        print(out.strip()[-1000:])
    if made:
        rc, out = aik("fmt")
        if rc != 0:
            raise SystemExit(f"aik fmt failed:\n{out[-2000:]}")
        mint = ["mint"] + (["-author", os.environ["AI_AUTHOR"]] if os.environ.get("AI_AUTHOR") else [])
        rc, out = aik(*mint)
        if rc != 0:
            raise SystemExit(f"aik mint failed:\n{out[-2000:]}")
    print(f"{made} stubs written; {write_slugs()} slugs in {SLUGS}")


if __name__ == "__main__":
    main()
