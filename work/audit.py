#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Audit pages with a reader of another model family (spec 14.6, 15.4).

Usage: python3 audit.py [--model codex:gpt-6-sol] [--jobs 4] <slug> ...
Writes state/audits/<slug>.json and prints the issues. Exit 1 when a page fails or waits.

The default auditor comes from auditor_for in registers/models.yaml, for the writers of the node. The writers are read from state/<slug>.json, else from prov/<tag>.jsonl.
--model is refused when it is of the family of a writer of the node. When no auditor of another family is available, the page waits.
"""
import argparse
import json
from concurrent.futures import ThreadPoolExecutor

from lib import REPO, WORK, Wait, extract_json, family, log, models, run_model, split_ref
from writer import audit_prompt, fmt_issues, load_catalog, state, tag_of, writers

OUT = WORK / "state" / "audits"


def writers_of(item):
    """Every model that wrote a file of the node, as backend:model."""
    w = writers(state(item["slug"]))
    if w:
        return w
    p = REPO / "prov" / f"{tag_of(item)}.jsonl"
    out = set()
    if p.exists():
        for ln in p.read_text(encoding="utf-8").splitlines():
            if ln.strip():
                a = json.loads(ln).get("author") or {}
                out |= {m if ":" in m else ":".join(split_ref(m)) for m in a.get("models") or []}
    return sorted(out)


def auditor(item, override):
    ws = writers_of(item)
    if override:
        b, m = split_ref(override)
        bad = [w for w in ws if family(w) == b]
        if bad:
            raise Wait(f"{override} is of the family of the writers {bad}; the auditor must be of another family (spec 15.4)")
        return b, m
    return models().auditor_for(ws)


def one(item, override):
    try:
        backend, model = auditor(item, override)
    except Wait as w:
        a = {"verdict": "wait", "issues": [], "reason": str(w)}
        (OUT / f"{item['slug']}.json").write_text(json.dumps(a, indent=1, ensure_ascii=False), encoding="utf-8")
        return item["slug"], a | {"auditor": "none"}
    r = run_model(audit_prompt(item), backend, model, job="audit", slug=item["slug"], role="audit",
                  write=False, timeout=1800, max_turns=40, effort="high")
    try:
        a = extract_json(r["text"])
        if isinstance(a, list):
            a = next(x for x in a if isinstance(x, dict) and "verdict" in x)
    except (ValueError, StopIteration):
        a = {"verdict": "error", "issues": [], "raw": r.get("text", "")[-800:] + r.get("err", "")[-400:]}
    a["auditor"] = f"{backend}:{model}"
    a["models_version"] = models().version
    (OUT / f"{item['slug']}.json").write_text(json.dumps(a, indent=1, ensure_ascii=False), encoding="utf-8")
    log({"job": "audit", "slug": item["slug"], "verdict": a.get("verdict"), "score": a.get("score")})
    return item["slug"], a


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", default="", help="backend:model; the default is auditor_for of models.yaml")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("slugs", nargs="+")
    a = ap.parse_args()
    OUT.mkdir(parents=True, exist_ok=True)
    items = load_catalog()
    missing = [s for s in a.slugs if s not in items]
    if missing:
        raise SystemExit(f"not in the catalog or the content checkout: {', '.join(missing)}")
    bad = 0
    with ThreadPoolExecutor(a.jobs) as pool:
        for slug, res in pool.map(lambda s: one(items[s], a.model), a.slugs):
            print(f"{slug}: {res.get('verdict')} score={res.get('score')} ({res['auditor']})")
            if res.get("reason"):
                print(f"  waits: {res['reason']}")
            if res.get("issues"):
                print(fmt_issues(res["issues"]))
            if res.get("raw"):
                print(res["raw"])
            bad += res.get("verdict") != "pass"
    raise SystemExit(1 if bad else 0)


if __name__ == "__main__":
    main()
