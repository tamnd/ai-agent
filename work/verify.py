#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Judge the claims that the citation check found with a verifier of another model family (G3, spec 7.5, 15.4).

Usage: python3 verify.py [--model codex:gpt-6-sol] [--writers claude-opus-5-5] [--redo] [--dry-run] <slug> ...

For each node it asks `aik cite passages` for the claims whose source passed the citation check and has no verdict
yet, sends the claims and their passages to the verifier in one prompt, and writes the verdicts into
prov/cite/<tag>.jsonl with `aik cite verdict`. The verifier sees only the claim, its type and scope, and the passage
with its section; it never sees the page.

The verifier is `verifier` of registers/models.yaml, unless it is of the family of a writer of the node, in which
case it is auditor_for of the writers. The writers come from prov/<tag>.jsonl, or from --writers when the provenance
does not name them. When no verifier of another family is available, the node waits.

The quotes and notes go to the run record prov/runs/<run>.yaml (14.7), never to the citation records.
Exit 1 when a claim is not supported or a node waits.
"""
import argparse
import datetime
import json
import os
import subprocess
import tempfile
import time

from lib import AIK, REPO, WORK, MissingCommand, Wait, _aik_text, extract_json, family, log, models, run_model, split_ref

PROMPT_VERSION = "verify/v1"
RUN_ID = os.environ.get("AI_RUN_ID") or time.strftime("v-%Y%m%d-%H%M%S-") + str(os.getpid())
VERDICTS = ("supported", "unsupported", "contradicted", "unreadable")
MAX_PASSAGE = 24000  # bytes of passage and section per item


def aik(sub, *args):
    """Run `aik cite <sub>` on the content repository and give its standard output."""
    if sub not in _aik_text("cite", "-h").split("Flags:")[0]:
        raise MissingCommand(f"aik has no command `aik cite {sub}` yet (spec 15.4); run a newer aik (now {AIK!r})")
    p = subprocess.run([AIK, "cite", sub, "-root", str(REPO), *args], capture_output=True, text=True)
    if p.returncode != 0:
        raise SystemExit(f"aik cite {sub} {' '.join(args)}: {p.stderr.strip() or p.stdout.strip()}")
    return p.stdout


def writers_of(tag):
    """Every model that the provenance of the node names as a writer, as backend:model."""
    p = REPO / "prov" / f"{tag}.jsonl"
    out = set()
    if p.exists():
        for ln in p.read_text(encoding="utf-8").splitlines():
            if not ln.strip():
                continue
            a = json.loads(ln).get("author")
            if isinstance(a, dict):
                out |= {m if ":" in m else ":".join(split_ref(m)) for m in a.get("models") or []}
    return sorted(out)


def verifier_for(ws, override):
    """The verifier (backend, model): never of the family of a writer (spec 15.4)."""
    if not ws:
        raise Wait("the provenance names no writer, so the cross-family rule cannot be checked; give --writers")
    fams = {family(w) for w in ws}
    if override:
        b, m = split_ref(override)
        if b in fams:
            raise Wait(f"{override} is of the family of the writers {ws}; the verifier must be of another family")
        return b, m
    reg = models()
    ref = reg.data.get("verifier")
    if ref:
        b, m = reg.resolve(ref)
        if b not in fams:
            return b, m
    return reg.auditor_for(ws)


def item_text(i, p):
    scope = p.get("scope") or {}
    sc = "; ".join(f"{k}: {v if isinstance(v, str) else ', '.join(v)}" for k, v in scope.items() if v)
    passage = p["passage"]
    section = p.get("section", "")
    if len(passage) > MAX_PASSAGE:
        passage = passage[:MAX_PASSAGE] + "\n[the passage is cut here]"
        section = ""
    elif len(passage) + len(section) > MAX_PASSAGE:
        section = section[:MAX_PASSAGE - len(passage)] + "\n[the section is cut here]"
    out = [f"## Item {i}", f"Claim ({p['type']}{', scope: ' + sc if sc else ''}): {p['text']}",
           f"Source: {p['work']}, {p['version']}, at {p['locator']}", "Passage:", "<<<", passage, ">>>"]
    if section:
        out += ["The section that holds the passage:", "<<<", section, ">>>"]
    return "\n".join(out)


def verify(slug, override, writers, redo, dry):
    ps = json.loads(aik("passages", *(["-redo"] if redo else []), slug) or "[]")
    if not ps:
        print(f"{slug}: no claim found by the citation check is waiting for a verdict")
        return 0, []
    tag = ps[0]["tag"]
    try:
        backend, model = verifier_for(writers or writers_of(tag), override)
    except Wait as w:
        print(f"{slug}: waits: {w}")
        return 1, []
    prompt = (WORK / "prompts" / "verify" / "v1.md").read_text(encoding="utf-8")
    prompt = prompt.format(items="\n\n".join(item_text(i + 1, p) for i, p in enumerate(ps)))
    if dry:
        print(prompt)
        return 0, []
    r = run_model(prompt, backend, model, job="verify", slug=slug, role="verifier", write=False, timeout=1800,
                  max_turns=4, effort="high")
    try:
        got = extract_json(r["text"])
    except ValueError:
        got = None
    if not isinstance(got, list):
        print(f"{slug}: the verifier {backend}:{model} gave no JSON list: {r.get('text', '')[-400:]}{r.get('err', '')[-400:]}")
        return 1, []
    by_id = {int(g["id"]): g for g in got if isinstance(g, dict) and str(g.get("id", "")).isdigit()}
    verdicts, items, bad = [], [], 0
    for i, p in enumerate(ps, 1):
        g = by_id.get(i) or {}
        v = g.get("verdict")
        if v not in VERDICTS:
            print(f"  {p['claim']} {p['work']}: no verdict from the verifier")
            bad += 1
            continue
        verdicts.append({"tag": tag, "key": p["key"], "verdict": v})
        items.append({"tag": tag, "claim": p["claim"], "work": p["work"], "locator": p["locator"], "key": p["key"],
                      "spans": p["spans"], "verdict": v, "quote": str(g.get("quote", ""))[:300],
                      "note": str(g.get("note", ""))[:600]})
        bad += v != "supported"
        print(f"  {v:<12} {p['claim']} ({p['work']}, {p['locator']})")
        if v != "supported" and g.get("note"):
            print(f"               {g['note']}")
    ver = f"{backend}:{model}"
    with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
        json.dump(verdicts, f)
    try:
        print(" ", aik("verdict", "-file", f.name, "-verifier", ver, "-run", RUN_ID).strip())
    finally:
        os.unlink(f.name)
    log({"job": "verify", "slug": slug, "verifier": ver, "items": len(ps), "not_supported": bad})
    return bad, [{"slug": slug, "verifier": ver, **it} for it in items]


def write_run(items):
    """The run record of 14.7, with the verifier's quotes and notes."""
    try:
        import yaml
    except ImportError:
        raise SystemExit("verify.py needs PyYAML for the run record") from None
    d = REPO / "prov" / "runs"
    d.mkdir(parents=True, exist_ok=True)
    rec = {"run": RUN_ID, "date": datetime.date.today().isoformat(), "role": "verifier", "prompt": PROMPT_VERSION,
           "models_version": models().version, "harness": "ai-agent/work/verify.py", "items": items}
    (d / f"{RUN_ID}.yaml").write_text(yaml.safe_dump(rec, sort_keys=False, allow_unicode=True, width=120), encoding="utf-8")
    print(f"run record prov/runs/{RUN_ID}.yaml")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", default="", help="backend:model of the verifier; the default is verifier of models.yaml")
    ap.add_argument("--writers", default="", help="a comma list of the writers, when the provenance does not name them")
    ap.add_argument("--redo", action="store_true", help="judge again the claims that have a verdict")
    ap.add_argument("--dry-run", action="store_true", help="print the prompt and stop")
    ap.add_argument("slugs", nargs="+")
    a = ap.parse_args()
    ws = [w if ":" in w else ":".join(split_ref(w)) for w in a.writers.split(",") if w]
    bad, items = 0, []
    for s in a.slugs:
        print(s)
        b, its = verify(s, a.model, ws, a.redo, a.dry_run)
        bad += b
        items += its
    if items:
        write_run(items)
    raise SystemExit(1 if bad else 0)


if __name__ == "__main__":
    main()
