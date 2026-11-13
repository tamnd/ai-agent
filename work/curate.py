#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Merge the raw catalog and the pending universe rows into catalog/curated.jsonl (spec 14.3 step 2).

Usage: python3 curate.py [--model claude-sonnet-5-5]

1. Normalize each item. Drop bad kinds and reserved slugs.
2. Group exact and near duplicates: the same normalized key, an alias of another item, or the same same_as id.
3. A model decides each group: one item survives with its primary area, the others become aliases. Items with the flag one-run are decided the same way.
4. For each survivor: the form (node, card or bridge, by the defaults of spec 04.2 and the border test of spec 5.9), the level, and the outcome for the universe register (spec 18.3).
5. Remove items that are already nodes or cards, and drop prereq targets that are not in the catalog.
Decisions are cached in catalog/decisions.json.
"""
import argparse
import json
import re
import sys
from collections import defaultdict

from kinds import DEFAULT_FORM, KINDS, LEVELS
from lib import REPO, WORK, extract_json, front_matter, load_yaml, log, models, run_model, split_ref

RAW = WORK / "catalog" / "raw"
OUT = WORK / "catalog" / "curated.jsonl"
DEC = WORK / "catalog" / "decisions.json"
SITES = {"al": "algorithm", "m2": "math", "pg": "programming"}


def slugify(s):
    s = s.lower().replace("'", "").replace("’", "")
    s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
    return s[:60].rstrip("-")


def key(s):
    k = slugify(s)
    k = re.sub(r"-(method|methods|model|models|algorithm|technique|problem|task|metric|theorem|hypothesis|principle|effect)$", "", k)
    k = re.sub(r"s$", "", k)
    return k.replace("-", "")


def jsonl(path):
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]


def areas():
    return load_yaml(REPO / "topics" / "areas.yaml")


def valid_areas():
    return {f"{t['id']}/{c['id']}" for t in areas() for c in t.get("children", [])}


def reserved():
    langs = [x["id"] for x in load_yaml(REPO / "registers" / "languages.yaml")]
    ids = []
    for t in areas():
        ids.append(t["id"])
        ids += [c["id"] for c in t.get("children", [])]
    words = []
    p = REPO / "registers" / "reserved.yaml"
    if p.exists():
        r = load_yaml(p)
        if isinstance(r, dict):
            for v in r.values():
                if isinstance(v, list):
                    words += [str(x) for x in v]
        elif isinstance(r, list):
            words += [str(x) for x in r]
    return set(langs) | set(ids) | set(words)


def existing():
    """The slugs of the nodes and the cards, with their kinds."""
    out = {}
    for p in (REPO / "nodes").rglob("node.md"):
        fm = front_matter(p)
        out[fm.get("slug") or p.parent.name] = fm.get("kind")
    for p in (REPO / "cards").glob("*/*.jsonl"):
        if p.parent.name == "bridge":
            continue
        for r in jsonl(p):
            if r.get("slug"):
                out[r["slug"]] = r.get("kind") or p.parent.name
    return out


def bridges():
    """key of the name or an alias -> the bridge card, for the border test of spec 5.9."""
    out = {}
    for p in sorted((REPO / "cards" / "bridge").glob("*/*.jsonl")):
        for r in jsonl(p):
            for n in [r.get("slug", ""), r.get("name", ""), *(r.get("aliases") or [])]:
                if n:
                    out.setdefault(key(n), r)
    return out


def load():
    """The raw catalog items and the pending universe rows, normalized."""
    items = []
    for f in sorted(RAW.glob("*.json")):
        items += json.loads(f.read_text(encoding="utf-8"))
    for f in sorted((REPO / "registers" / "universe").glob("*.jsonl")):
        for r in jsonl(f):
            if r.get("outcome", "pending") != "pending":
                continue
            items.append({"slug": r.get("name", ""), "name": r.get("name", ""), "kind": r.get("kind"), "area": r.get("area", ""),
                          "aliases": r.get("aliases") or [], "same_as": r.get("same_as") or {}, "sources": r.get("sources") or [],
                          "universe": r.get("id"), "core": bool(r.get("core")), "note": r.get("reason", "")})
    out = []
    for it in items:
        if not isinstance(it, dict) or it.get("kind") not in KINDS or not it.get("slug"):
            continue
        it["slug"] = slugify(it["slug"])
        it["level"] = it.get("level") if it.get("level") in LEVELS else "advanced"
        for k in ("aliases", "prereqs", "sources"):
            it[k] = [x for x in (it.get(k) or []) if isinstance(x, str)]
        it["prereqs"] = [slugify(x) for x in it["prereqs"]]
        it.setdefault("same_as", {})
        out.append(it)
    return out


def groups(items):
    by = defaultdict(list)
    for i, it in enumerate(items):
        by[key(it["slug"])].append(i)
    parent = list(range(len(items)))

    def find(x):
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    for ids in by.values():
        for j in ids[1:]:
            parent[find(j)] = find(ids[0])
    for i, it in enumerate(items):
        for a in it["aliases"]:
            for j in by.get(key(a), []):
                if j != i and items[j]["kind"] == it["kind"]:
                    parent[find(j)] = find(i)
    ids = defaultdict(list)
    for i, it in enumerate(items):
        for src, v in (it.get("same_as") or {}).items():
            ids[f"{src}:{v}"].append(i)
    for v in ids.values():
        for j in v[1:]:
            parent[find(j)] = find(v[0])
    g = defaultdict(list)
    for i in range(len(items)):
        g[find(i)].append(i)
    return [v for v in g.values() if len(v) > 1 or items[v[0]].get("flag")]


DECIDE = """You curate the catalog of ai.tamnd.com. Each group below lists catalog entries that may be the same item, written by different cataloguers or harvested from different sources. A group of one entry has the flag one-run: only one of two cataloguer models listed it, so decide whether it is a real item of the field (keep it) or not (give it an empty "nodes" list).

For each group, decide which entries are the same thing. Entries that are the same thing become ONE item: choose the best slug (the common English name, kebab-case), the kind, the ONE primary area where a reader expects it, and the form:
- "node" for an item that gets a written page, "card" for a row with facts and no page. The default form of each kind is: {forms}.
- An item that a neighbour site owns (an algorithm, a data structure, a mathematical object or a language feature, with no AI view that adds three facts with sources) is a bridge card; give the form "bridge".
Entries that are different things stay separate.
Do not judge which method or model is better. Decide only identity, home and form.

The area tree:
{tree}

Groups (each entry: index | slug | kind | area | name | note | sources):
{groups}

Reply with ONLY a JSON array, one object per group, in order: {{"group": <n>, "nodes": [{{"keep": <index of the entry whose data to keep>, "slug": "...", "kind": "...", "area": "top/sub", "form": "node|card|bridge", "merge": [<indices that are the same thing>]}}]}}. Every index of the group must be in exactly one "merge" list (the keep index is also in its own merge list), unless the group is dropped.
"""


def tree():
    return "\n".join(f"- {t['id']}: " + ", ".join(c["id"] for c in t.get("children", [])) for t in areas())


def decide(items, gs, ref):
    decisions = json.loads(DEC.read_text(encoding="utf-8")) if DEC.exists() else {}
    todo = [g for g in gs if ",".join(map(str, g)) not in decisions]
    tr = tree()
    forms = ", ".join(f"{k} {v}" for k, v in DEFAULT_FORM.items())
    backend, model = split_ref(ref)
    for start in range(0, len(todo), 60):
        chunk = todo[start:start + 60]
        txt = []
        for n, g in enumerate(chunk):
            txt.append(f"Group {n}:")
            for i in g:
                it = items[i]
                txt.append(f"  {i} | {it['slug']} | {it['kind']} | {it['area']} | {it.get('name', '')} | {it.get('note', '')[:120]} | {', '.join(it['sources'][:3])}")
        r = run_model(DECIDE.format(tree=tr, forms=forms, groups="\n".join(txt)), backend, model,
                      job="curate", role="curator", write=False, timeout=1200, max_turns=2)
        try:
            ans = extract_json(r["text"])
        except ValueError:
            print("decide: no JSON", r.get("err", "")[:300], file=sys.stderr)
            continue
        for a in ans:
            if isinstance(a, dict) and isinstance(a.get("group"), int) and a["group"] < len(chunk):
                decisions[",".join(map(str, chunk[a["group"]]))] = a.get("nodes") or []
        DEC.write_text(json.dumps(decisions, indent=1), encoding="utf-8")
        log({"job": "curate", "groups": len(chunk), "model": ref})
        print(f"decided {start + len(chunk)}/{len(todo)}", flush=True)
    return decisions


def border(it, br):
    """The border test of spec 5.9: an item whose name is a bridge card of a neighbour site."""
    for n in [it["slug"], it.get("name", ""), *it["aliases"]]:
        r = br.get(key(n)) if n else None
        if r:
            return r
    return None


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", default="", help="the curator model; the default is the curator role of models.yaml")
    args = ap.parse_args()
    ref = args.model or models().role("curator")[0]
    items = load()
    res = reserved()
    valid = valid_areas()
    gs = groups(items)
    print(f"{len(items)} raw items, {len(gs)} groups to decide", flush=True)
    decisions = decide(items, gs, ref)
    drop, out, form = set(), [], {}
    for g in gs:
        d = decisions.get(",".join(map(str, g)))
        if d is None:
            continue
        if not d:
            drop |= set(g)
            continue
        for node in d:
            if not isinstance(node.get("keep"), int) or node["keep"] >= len(items):
                continue
            keep = items[node["keep"]]
            names = set()
            for j in node.get("merge", []):
                if j == node["keep"] or j >= len(items):
                    continue
                o = items[j]
                drop.add(j)
                names |= {o.get("name", ""), *o["aliases"]}
                keep["prereqs"] = sorted(set(keep["prereqs"]) | set(o["prereqs"]))
                keep["sources"] = sorted(set(keep["sources"]) | set(o["sources"]))
                keep["same_as"] = {**o.get("same_as", {}), **keep.get("same_as", {})}
                keep["core"] = keep.get("core") or o.get("core")
                if o.get("universe"):
                    keep.setdefault("merged", []).append(o["universe"])
            keep["slug"] = slugify(node.get("slug", keep["slug"]))
            if node.get("kind") in KINDS:
                keep["kind"] = node["kind"]
            if node.get("area") in valid:
                keep["area"] = node["area"]
            if node.get("form") in ("node", "card", "bridge"):
                form[node["keep"]] = node["form"]
            keep["aliases"] = sorted((set(keep["aliases"]) | names) - {keep.get("name", ""), ""})
            keep.pop("flag", None)
    ex = existing()
    br = bridges()
    seen = {}
    for i, it in enumerate(items):
        if i in drop or it.get("flag"):
            continue
        s = it["slug"]
        if s in ex or s in res or s in seen or it["area"] not in valid:
            continue
        b = border(it, br)
        f = form.get(i) or ("bridge" if b else DEFAULT_FORM[it["kind"]])
        it["form"] = f
        it["outcome"] = f
        if f == "bridge":
            if not b:
                it["form"] = it["outcome"] = DEFAULT_FORM[it["kind"]]
            else:
                it["elsewhere"] = b["site"]
                it["target"] = f"{b['site']}:{b['slug']}"
                it["reason"] = f"border test 5.9: {SITES.get(b['site'], b['site'])} owns it"
        if it["outcome"] == "node":
            it["target"] = f"ai://NEW-{s}"
        it["decided_by"] = f"curator:{ref}@{models().version}"
        seen[s] = it
    bykey = {key(s): s for s in list(seen) + list(ex)}
    known = set(seen) | set(ex)
    for s, it in seen.items():
        pre = []
        for x in it["prereqs"]:
            x = bykey.get(key(x), x)
            if x in known and x != s and x not in pre:
                pre.append(x)
        it["prereqs"] = pre
        out.append(it)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT, "w", encoding="utf-8") as f:
        for it in sorted(out, key=lambda x: (x["area"], x["slug"])):
            f.write(json.dumps(it, ensure_ascii=False) + "\n")
    n = defaultdict(int)
    for it in out:
        n[it["outcome"]] += 1
    print(f"{len(out)} curated items ({', '.join(f'{k} {v}' for k, v in sorted(n.items()))}), "
          f"{sum(1 for x in out if x.get('core'))} core; {len(drop)} merged or dropped")


if __name__ == "__main__":
    main()
