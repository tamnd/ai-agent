#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""List the named items of each canon work and each frontier source, one model call per part (spec 14.3 step 3).

Usage: python3 canon.py [--jobs 6] [--only <work id>] [--force] [--model claude-sonnet-5-5]
Output: catalog/canon/<ref>--<n>.json, a list of {name, kind, where, aliases}.

The works come from registers/canon.yaml (the 19 canon works of spec 5.10) and registers/surveys.yaml when it exists.
A part is a chunk of the toc entries of the work. A work with no toc entries yet is one part.
The register holds names and locators only (spec 19.3), so the prompt asks for names and locators only.
"""
import argparse
import json
from concurrent.futures import ThreadPoolExecutor

from kinds import IDEA_KINDS
from lib import REPO, WORK, extract_json, load_yaml, log, models, run_model, split_ref

OUT = WORK / "catalog" / "canon"
CHUNK = 12  # toc entries per model call

PROMPT = """List every named item of artificial intelligence that {ref} covers in {part}.

An item is a concept, a definition, a method, an architecture, a task, a metric, a theorem or a lemma with a name, a finding, an argument, an open problem or a risk. Include the items that are only a named subsection or a named variant (for example "inverted dropout" or "label smoothing"). Do not include people, models, datasets, chapter titles that are not items, or general words such as "introduction".

Be complete: a reader will check your list against the table of contents and the index. Do not invent items that the source does not cover. Give names and locators only. Do not quote the text of the source.

Reply with ONLY a JSON array. Each element: {{"name": "<the common English name>", "kind": "{kinds}", "where": "<chapter or section number and title>", "aliases": ["<other names the source uses>"]}}
"""


def works():
    """[(id, citation, [part, ...]), ...] from canon.yaml and surveys.yaml."""
    out = []
    for name in ("canon.yaml", "surveys.yaml"):
        p = REPO / "registers" / name
        if not p.exists():
            continue
        d = load_yaml(p)
        rows = d.get("works", d.get("surveys", [])) if isinstance(d, dict) else d
        for w in rows or []:
            if not isinstance(w, dict) or not w.get("id"):
                continue
            authors = ", ".join(w.get("authors") or [])
            ed = f", edition {w['edition']}" if w.get("edition") else ""
            cite = f"{authors}, {w.get('title', w['id'])}{ed} ({w.get('year', 'n.d.')})"
            if w.get("url"):
                cite += f", {w['url']}"
            toc = [t if isinstance(t, str) else f"{t.get('locator', '')} {t.get('title', '')}".strip() for t in (w.get("toc") or [])]
            parts = [", ".join(toc[i:i + CHUNK]) for i in range(0, len(toc), CHUNK)] or ["the whole work"]
            out.append((w["id"], cite, parts))
    if not out:
        raise SystemExit(f"no works in {REPO / 'registers' / 'canon.yaml'}")
    return out


def job(ref, n, title, part, model_ref, force):
    f = OUT / f"{ref}--{n}.json"
    if f.exists() and not force:
        return f"{ref}--{n}: cached"
    backend, model = split_ref(model_ref)
    prompt = PROMPT.format(ref=title, part=part, kinds="|".join(IDEA_KINDS))
    r = run_model(prompt, backend, model, job="canon", slug=f"{ref}--{n}", role="curator", write=False, timeout=1800, max_turns=2)
    try:
        items = extract_json(r["text"])
        assert isinstance(items, list)
    except (ValueError, AssertionError):
        log({"job": "canon", "ref": ref, "n": n, "ok": False, "err": r.get("err", "")[:300]})
        return f"{ref}--{n}: FAILED"
    f.write_text(json.dumps(items, indent=1, ensure_ascii=False), encoding="utf-8")
    log({"job": "canon", "ref": ref, "n": n, "items": len(items)})
    return f"{ref}--{n}: {len(items)} items ({r['secs']}s)"


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--jobs", type=int, default=6)
    ap.add_argument("--only", help="one work id")
    ap.add_argument("--force", action="store_true")
    ap.add_argument("--model", default="", help="the default is the curator role of models.yaml")
    a = ap.parse_args()
    model_ref = a.model or models().role("curator")[0]
    OUT.mkdir(parents=True, exist_ok=True)
    tasks = [(ref, n, title, part) for ref, title, parts in works() for n, part in enumerate(parts)
             if not a.only or ref == a.only]
    with ThreadPoolExecutor(a.jobs) as pool:
        for line in pool.map(lambda t: job(*t, model_ref, a.force), tasks):
            print(line, flush=True)


if __name__ == "__main__":
    main()
