#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Make the catalog: for each sub-area, the cataloguer models list the ideas whose primary area is that sub-area and that the universe does not have yet (spec 14.3 step 1).

Usage: python3 catalog.py [--backend claude --model claude-opus-5-5] [--jobs 8] [--only top/sub ...] [--force]
Writes catalog/raw/<top>--<sub>.json, and one file per model run in catalog/raw/runs/.

The input is the universe rows of the sub-area (registers/universe/<top>.jsonl), the existing nodes, cards and bridge cards, and the area brief when it exists.
The model adds only idea kinds, and each addition must name a source. Artifacts come only from harvest (spec 14.1).
Two runs with models of different families (the cataloguer role of models.yaml) must agree: an item that only one run lists gets the flag one-run, and the curator decides it.
"""
import argparse
import json
from concurrent.futures import ThreadPoolExecutor

from kinds import IDEA_KINDS, LEVELS
from lib import REPO, WORK, Wait, available, extract_json, load_yaml, log, models, run_model, split_ref

RAW = WORK / "catalog" / "raw"
RUNS = RAW / "runs"
BRIEFS = WORK / "prompts" / "areas"

PROMPT = """You are a cataloguer of ai.tamnd.com, a reference that must cover all of artificial intelligence with no item missing.

The full area tree of the site (top area: sub-areas) is:
{tree}

Your job: list every idea whose PRIMARY home is the sub-area **{top}/{sub}** ("{subname}" in "{topname}") and that is NOT in the lists below.
An idea is one of these kinds: {kinds}.
Do not list models, datasets, benchmarks, systems, software, hardware, policies, works, people, organizations or events. They come from harvested sources, not from recall.
Items that belong more naturally to another sub-area of the tree must not be listed here.
Do not invent items. Each item must be a real, named thing in the literature, and you must name a source for it: a canon work with a chapter or section (for example canon:goodfellow-2016:7.12), an arXiv id (arxiv:1207.0580), a DOI (doi:10.1000/x), or a URL.
Do not give numbers, scores, dates of releases or rankings. The catalog has no facts.

{brief}The universe rows of this sub-area (already known, do not repeat):
{universe}

Already on the site as nodes or cards (do not repeat): {existing}

Owned by a neighbour site, as bridge cards (do not repeat, unless the AI view of the item differs; then say so in the note): {bridge}

For each item give:
- slug: lowercase kebab-case, at most 60 characters, the common English name.
- name: the display name.
- kind: one of {kinds}.
- level: one of {levels}.
- aliases: other names (a list, it may be empty).
- prereqs: the slugs of the items a reader must know first (any area; use the common slug).
- note: one short line that says what it is.
- source: the source, in one of the forms above.

Reply with ONLY a JSON array of objects with exactly these keys. No prose.
"""


def tree():
    d = load_yaml(REPO / "topics" / "areas.yaml")
    lines, subs = [], []
    for t in d:
        kids = t.get("children", [])
        lines.append(f"- {t['id']} ({t['name']}): " + ", ".join(c["id"] for c in kids))
        for c in kids:
            subs.append((t["id"], t["name"], c["id"], c["name"]))
    return "\n".join(lines), subs


def jsonl(path):
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]


def universe(top, sub):
    """The universe rows of the sub-area, as short lines."""
    rows = [r for r in jsonl(REPO / "registers" / "universe" / f"{top}.jsonl") if r.get("area") == f"{top}/{sub}"]
    return "\n".join(f"- {r.get('name')} ({r.get('kind')}, {r.get('outcome', 'pending')})" for r in rows) or "(none yet)"


def existing():
    """The slugs of the nodes and the cards of the site."""
    out = {p.parent.name for p in (REPO / "nodes").rglob("node.md")}
    for p in (REPO / "cards").glob("*/*.jsonl"):
        if p.parent.name == "bridge":
            continue
        out |= {r.get("slug") for r in jsonl(p) if r.get("slug")}
    return ", ".join(sorted(out)) or "(none yet)"


def bridge():
    out = []
    for p in sorted((REPO / "cards" / "bridge").glob("*/*.jsonl")):
        out += [f"{r['site']}:{r['slug']}" for r in jsonl(p) if r.get("slug")]
    return ", ".join(sorted(out)) or "(none)"


def brief(top):
    p = BRIEFS / f"{top}.md"
    return f"The area brief:\n{p.read_text(encoding='utf-8')}\n\n" if p.exists() else ""


def clean(items, area):
    out = []
    for it in items if isinstance(items, list) else []:
        if not isinstance(it, dict) or it.get("kind") not in IDEA_KINDS or not it.get("slug"):
            continue
        if not str(it.get("source") or "").strip():
            continue  # spec 14.3: each addition must name a source
        it["area"] = area
        if it.get("level") not in LEVELS:
            it["level"] = "advanced"
        out.append(it)
    return out


def run_one(prompt, ref, area):
    backend, model = split_ref(ref)
    if not available(backend, model):
        raise Wait(f"{ref} is not available")
    for _ in range(3):
        r = run_model(prompt, backend, model, job="catalog", slug=area, role="cataloguer",
                      write=False, timeout=900, max_turns=3, effort="medium")
        if r.get("limited"):
            raise Wait(f"{ref} is rate-limited")
        try:
            items = clean(extract_json(r["text"]), area)
            if items or r.get("ok"):
                return items
        except ValueError as e:
            log({"job": "catalog", "area": area, "model": ref, "error": str(e), "err": r.get("err", "")[:500]})
    raise RuntimeError(f"{ref}: no valid JSON after 3 tries")


def merge(runs):
    """Join the runs by slug. An item of only one run gets the flag one-run (the two-model rule)."""
    by = {}
    for ref, items in runs.items():
        for it in items:
            cur = by.setdefault(it["slug"], dict(it, runs=[]))
            cur["runs"].append(ref)
            cur["aliases"] = sorted(set(cur.get("aliases") or []) | set(it.get("aliases") or []))
            srcs = {str(s) for s in ([cur.get("source")] + list(cur.get("sources") or []) + [it.get("source")]) if s}
            cur["sources"] = sorted(srcs)
    for it in by.values():
        it.pop("source", None)
        if len(runs) > 1 and len(it["runs"]) < 2:
            it["flag"] = "one-run"
    return sorted(by.values(), key=lambda x: x["slug"])


def one(args, tr, ex, br, s):
    top, topname, sub, subname = s
    area = f"{top}/{sub}"
    dest = RAW / f"{top}--{sub}.json"
    if dest.exists() and not args.force:
        return
    prompt = PROMPT.format(tree=tr, top=top, sub=sub, topname=topname, subname=subname, existing=ex, bridge=br,
                           universe=universe(top, sub), brief=brief(top), kinds=", ".join(IDEA_KINDS), levels=", ".join(LEVELS))
    refs = [f"{args.backend}:{args.model}"] if args.model else models().role("cataloguer")
    runs = {}
    for i, ref in enumerate(refs):
        part = RUNS / f"{top}--{sub}--{i}.json"
        if part.exists() and not args.force:
            runs[ref] = json.loads(part.read_text(encoding="utf-8"))
            continue
        try:
            items = run_one(prompt, ref, area)
        except Wait as w:
            print(f"{area}: WAIT ({w})", flush=True)
            log({"job": "catalog", "area": area, "model": ref, "wait": str(w)})
            return
        except RuntimeError as e:
            print(f"{area}: FAILED ({e})", flush=True)
            return
        part.write_text(json.dumps(items, indent=1, ensure_ascii=False), encoding="utf-8")
        runs[ref] = items
    items = merge(runs)
    dest.write_text(json.dumps(items, indent=1, ensure_ascii=False), encoding="utf-8")
    flagged = sum(1 for x in items if x.get("flag"))
    log({"job": "catalog", "area": area, "models": list(runs), "n": len(items), "one_run": flagged})
    print(f"{area}: {len(items)} items, {flagged} in one run only ({', '.join(runs)})", flush=True)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--backend", default="claude", help="the backend of --model")
    ap.add_argument("--model", default="", help="one model for a single run; the default is the cataloguer role of models.yaml")
    ap.add_argument("--jobs", type=int, default=8)
    ap.add_argument("--only", nargs="*", help="top/sub ids")
    ap.add_argument("--force", action="store_true", help="run again when the output exists")
    args = ap.parse_args()
    RUNS.mkdir(parents=True, exist_ok=True)
    tr, subs = tree()
    ex, br = existing(), bridge()
    if args.only:
        subs = [s for s in subs if f"{s[0]}/{s[2]}" in args.only or s[0] in args.only]
    with ThreadPoolExecutor(args.jobs) as pool:
        list(pool.map(lambda s: one(args, tr, ex, br, s), subs))


if __name__ == "__main__":
    main()
