#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Write node pages with a ladder of models, the AI gate and a cross-family audit (spec 14.6).

Usage:
  python3 writer.py --slugs dropout transformer ...       # these items
  python3 writer.py --queue --limit 50 --jobs 6           # the next items of the queue (core first)
Options:
  --writers codex:gpt-6-luna,claude:claude-haiku-5-5      # override the ladder of registers/models.yaml
State: state/<slug>.json. Logs: logs/events.jsonl, logs/runs/<slug>/*.txt. Provenance: prov/<tag>.jsonl in the content checkout.

The ladder is writer_ladder of models.yaml, or writer_ladder_hard for the hard kinds of kinds.py.
The auditor is never of the family of a writer of the node. When no such auditor is available, the item waits with the status blocked (spec 14.4, Q-08).
The gate is `aik gate -json <slug>`, plus the cheap local checks of algorithm-work. `aik cite <slug>` runs once, after the audit passes.
AIK_GATE_FLAGS adds flags to the gate, as -unsafe-local on a machine without docker. A gate that fails for a reason of
the machine (docker down, a full disk) blocks the item instead of asking a model to fix the page.
"""
import argparse
import hashlib
import json
import os
import re
import threading
import time
from concurrent.futures import ThreadPoolExecutor

from kinds import CARD_ONLY, HARD, gold_paths, sections_for
from lib import (LOG, REPO, SPEC, WORK, Wait, aik, available, extract_json, front_matter, log, models, need_aik,
                 need_yaml, run_model, split_ref)

STATE = WORK / "state"
RUNS = LOG / "runs"
CATALOG = WORK / "catalog" / "curated.jsonl"
REFS = WORK / "catalog" / "canon_refs.json"
SLUGS = WORK / "catalog" / "slugs.tsv"
PROMPTS = WORK / "prompts"
BRIEFS = PROMPTS / "areas"
VERSION = os.environ.get("AI_PROMPT_VERSION", "v1")
RUN_ID = os.environ.get("AI_RUN_ID") or time.strftime("w-%Y%m%d-%H%M%S-") + str(os.getpid())
STUB_MARKERS = ["Write one sentence that says what this is"]
WRITE_ROLES = ("write", "fix-gate", "fix-audit", "fix-minor")
_gate_lock = threading.Semaphore(3)  # aik gate runs the framework tests and the cpu-small experiments


def node_dir(item):
    return f"nodes/{item['area']}/{item['slug']}"


def jsonl(path):
    if not path.exists():
        return []
    return [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]


def load_catalog():
    items = {it["slug"]: it for it in jsonl(CATALOG)}
    # nodes in the content checkout that the catalog does not list (gold pages, pilots)
    for p in (REPO / "nodes").glob("*/*/*/node.md"):
        try:
            fm = front_matter(p)
        except Exception:  # noqa: BLE001
            continue
        s = fm.get("slug") or p.parent.name
        if s in items:
            items[s]["area"] = str(p.parent.parent.relative_to(REPO / "nodes"))
            continue
        items[s] = {"slug": s, "name": fm.get("name", s), "kind": fm.get("kind", "concept"), "level": fm.get("level", "core"),
                    "area": str(p.parent.parent.relative_to(REPO / "nodes")), "core": False,
                    "aliases": fm.get("aliases") or [], "prereqs": fm.get("prereqs") or [], "sources": [],
                    "note": fm.get("summary", "")}
    return items


def tag_of(item):
    p = REPO / node_dir(item) / "node.md"
    m = re.search(r"^tag: (\S+)", p.read_text(encoding="utf-8"), re.M) if p.exists() else None
    return m.group(1) if m else ""


def state(slug):
    p = STATE / f"{slug}.json"
    return json.loads(p.read_text(encoding="utf-8")) if p.exists() else {"slug": slug, "status": "todo", "history": []}


def save(st):
    STATE.mkdir(parents=True, exist_ok=True)
    (STATE / f"{st['slug']}.json").write_text(json.dumps(st, indent=1, ensure_ascii=False), encoding="utf-8")


def written_sections(path):
    """The number of ## sections that hold text. aik new writes every heading of the kind, so a heading alone does not count."""
    if not path.exists():
        return 0
    body = path.read_text(encoding="utf-8").split("\n---", 1)[-1]
    parts = re.split(r"^## .*$", body, flags=re.M)[1:]
    return sum(1 for p in parts if p.strip())


# The gate


def plain_words_len(body):
    m = re.search(r"^## In plain words\n(.*?)(?=^## |\Z)", body, re.S | re.M)
    return len(m.group(1).split()) if m else 0


def null_keys(x):
    """Keys with no value whose text holds a space: the sign of a comma that split a flow mapping value."""
    out = []
    if isinstance(x, dict):
        for k, v in x.items():
            if v is None and isinstance(k, str) and " " in k.strip():
                out.append(k)
            out += null_keys(v)
    elif isinstance(x, list):
        for i in x:
            out += null_keys(i)
    return out


def findings(out):
    """The problems in the JSON output of aik gate or aik check: a list of strings or of finding objects."""
    i = min([j for j in (out.find("["), out.find("{")) if j != -1], default=-1)
    if i == -1:
        return None
    try:
        d = json.loads(out[i:])
    except json.JSONDecodeError:
        return None
    if isinstance(d, dict):
        d = d.get("problems", d.get("findings", []))
    probs = []
    for f in d or []:
        if isinstance(f, str):
            probs.append(f)
        elif isinstance(f, dict) and f.get("severity", "error") not in ("warn", "warning", "info"):
            where = f.get("path") or f.get("file") or ""
            probs.append(f"{f.get('step', 'aik gate')}: {where}: {f.get('rule', '')} {f.get('severity', '')}: {f.get('msg') or f.get('message', '')}".strip())
    return probs


def local_checks(item):
    """The cheap checks of algorithm-work that need no aik command (spec 14.6 steps 3 to 5)."""
    yaml = need_yaml()
    d = REPO / node_dir(item)
    probs = []
    md = (d / "node.md").read_text(encoding="utf-8")
    try:
        nk = null_keys(yaml.safe_load(md.split("---")[1]))
    except yaml.YAMLError as e:
        nk, probs = [], probs + [f"front matter is not valid YAML: {e}"]
    if nk:
        probs.append("front matter: a comma inside a flow mapping value split it, so these became keys with no value: " + "; ".join(repr(k) for k in nk) + ". Quote the value, for example {meaning: \"the rate, a value in [0, 1)\"}")
    files = [d / "node.md", *sorted((d / "derivations").glob("*.md")), *sorted(d.glob("*.yaml"))]
    for f in files:
        txt = f.read_text(encoding="utf-8")
        for m in STUB_MARKERS:
            if m in txt:
                probs.append(f"{f.relative_to(d)} still holds the stub text {m!r}")
        for ch, name in (("\t", "tab"), ("\f", "form feed"), ("\r", "carriage return"), ("\u2014", "em dash"), ("\u2013", "en dash")):
            if ch in txt:
                ln = txt[: txt.index(ch)].count("\n") + 1
                probs.append(f"{f.relative_to(d)}: line {ln} holds a {name} character. A tab or form feed in math is often a broken escape such as \\times or \\frac written through a string")
    n = plain_words_len(md.split("\n---", 1)[-1])
    if n > 150:
        probs.append(f"'In plain words' has {n} words; the limit is 150")
    if not (d / "claims.yaml").exists():
        probs.append("claims.yaml is missing (spec 07.3)")
    return probs


# ENV_FAULTS are gate failures of the machine, not of the page: no model edit can fix them, so the item waits.
ENV_FAULTS = ("Cannot connect to the Docker daemon", "docker.sock", "the tests run in docker containers",
              "no space left on device")


def gate_flags():
    """The extra flags of aik gate, from AIK_GATE_FLAGS, as "-unsafe-local" on a machine without docker."""
    return os.environ.get("AIK_GATE_FLAGS", "").split()


def gate(item, final=False):
    """Run the gate. Returns a list of problem strings; an empty list means pass. Raises Wait when the gate fails
    for a reason of the machine (ENV_FAULTS), as docker being down."""
    slug = item["slug"]
    probs = local_checks(item)
    with _gate_lock:
        rc, out = aik("gate", "-json", *gate_flags(), slug, timeout=1800)
    if rc:
        for f in ENV_FAULTS:
            if f in out:
                raise Wait(f"aik gate fails for a reason of the machine, not the page ({f!r}); fix it, or set AIK_GATE_FLAGS=-unsafe-local")
    fs = findings(out)
    if fs is None:
        if rc:
            probs.append(f"`aik gate -json {slug}` failed:\n{out[-2500:]}")
    else:
        probs += fs
        if rc and not fs:
            probs.append(f"`aik gate -json {slug}` exited with {rc}:\n{out[-1500:]}")
    if final and not probs:
        rc, out = aik("cite", slug, timeout=1800)
        if rc:
            probs.append(f"`aik cite {slug}` failed: a quoted claim is not in its source snapshot (G05, spec 07.5):\n{out[-2500:]}")
    return probs


# The prompts


def prompt_text(role):
    p = PROMPTS / role / f"{VERSION}.md"
    if not p.exists():
        raise SystemExit(f"the prompt {p} does not exist; set AI_PROMPT_VERSION to a version that exists")
    return p.read_text(encoding="utf-8")


def gold_text(kind):
    lines = []
    for s, path in gold_paths(kind):
        if path:
            lines.append(f"   - `{path}/` (every file: node.md, claims.yaml, derivations/, and the code and experiments when present)")
        else:
            lines.append(f"   - `{s}` is not written yet. Follow spec 06.3 for the {kind} kind")
    return "\n".join(lines)


def sources_of(item):
    """The sources that the writer may cite: those of the catalog item and the canon works that name it."""
    refs = json.loads(REFS.read_text(encoding="utf-8")).get(item["slug"], []) if REFS.exists() else []
    out = list(dict.fromkeys([*(item.get("sources") or []), *(f"canon:{r}" for r in refs)]))
    return out


def context_of(item):
    """The cards and the fact records of the item, as paths and rows."""
    d = REPO / node_dir(item)
    lines = []
    if (d / "facts.yaml").exists():
        lines.append(f"   - the fact records: `{node_dir(item)}/facts.yaml`")
    for p in sorted((REPO / "cards").glob("*/*.jsonl")):
        if p.parent.name == "bridge":
            continue
        for r in jsonl(p):
            if r.get("slug") == item["slug"]:
                lines.append(f"   - the card in `{p.relative_to(REPO)}`: {json.dumps(r, ensure_ascii=False)[:1500]}")
    lines.append(f"   - the bridge cards that you may link: the `al:`, `m2:` and `pg:` rows of `{SLUGS}`")
    return "\n".join(lines)


def writer_prompt(item):
    slug, kind = item["slug"], item["kind"]
    brief = BRIEFS / f"{item['area'].split('/')[0]}.md"
    srcs = sources_of(item)
    return prompt_text("writer").format(
        slug=slug, name=item["name"], kind=kind, area=item["area"], level=item.get("level", "core"),
        core_note=" (core: the canon names it, so it must be excellent)" if item.get("core") else "",
        tag=tag_of(item), note=item.get("note", ""), aliases=item.get("aliases", []), prereqs=item.get("prereqs", []),
        node_dir=node_dir(item), spec=SPEC, gold=gold_text(kind), kind_tasks=sections_for(kind), slugs_file=SLUGS,
        brief=f"   - `{brief}`" if brief.exists() else "   - (none)", context=context_of(item),
        sources="\n".join(f"- {s}" for s in srcs) if srcs else "- (no source beyond registers/canon.yaml)")


def fix_prompt(item, problems, writer_file):
    return prompt_text("fix").format(slug=item["slug"], kind=item["kind"], node_dir=node_dir(item), problems=problems,
                                     writer_prompt_file=writer_file)


def audit_prompt(item):
    refs = json.loads(REFS.read_text(encoding="utf-8")).get(item["slug"], []) if REFS.exists() else []
    return prompt_text("audit").format(slug=item["slug"], kind=item["kind"], node_dir=node_dir(item), spec=SPEC,
                                       gold=gold_text(item["kind"]), canon_refs=", ".join(refs) or "(none)",
                                       sources=", ".join(sources_of(item)) or "(registers/canon.yaml only)")


def fmt_issues(issues):
    out = []
    for i, x in enumerate(issues, 1):
        out.append(f"{i}. [{x.get('severity')}] {x.get('file', '')}: \"{x.get('quote', '')}\": {x.get('problem', '')} Fix: {x.get('fix', '')}")
    return "\n".join(out)


# The loop


def pick(ladder, i):
    """The first writer of the ladder from rung i that is available now: (rung, backend, model) or None."""
    while i < len(ladder):
        b, m = split_ref(ladder[i])
        if available(b, m):
            return i, b, m
        i += 1
    return None


def climb(ladder, rung, st, sel):
    """The next writer above rung that is available and keeps an auditor of another family for the node: a model of a
    new family would leave no auditor that wrote none of it (spec 15.4, Q-08). The current writer when there is none."""
    i = rung + 1
    while (nxt := pick(ladder, i)) is not None:
        try:
            models().auditor_for(sorted(set(writers(st)) | {f"{nxt[1]}:{nxt[2]}"}))
            return nxt
        except Wait:
            i = nxt[0] + 1
    return sel


def writers(st):
    """Every model that wrote a file of the node, as backend:model."""
    return sorted({f"{h['backend']}:{h['model']}" for h in st["history"] if h["role"] in WRITE_ROLES})


def record(item, role, i, prompt, r):
    d = RUNS / item["slug"]
    d.mkdir(parents=True, exist_ok=True)
    (d / f"{int(time.time())}-{role}-{i}.txt").write_text(
        f"PROMPT\n{prompt}\n\nREPLY\n{r.get('text', '')}\n\nERR\n{r.get('err', '')}", encoding="utf-8")


def call(item, st, role, backend, model, prompt, write=True):
    r = run_model(prompt, backend, model, job="write", slug=item["slug"], role=role, write=write, timeout=2400, effort="medium")
    record(item, role, len(st["history"]), prompt, r)
    st["history"].append({"role": role, "backend": backend, "model": model, "ok": r["ok"], "secs": r.get("secs"),
                          "tokens_in": r.get("tokens_in"), "tokens_out": r.get("tokens_out"), "cost": r.get("cost"),
                          "limited": r.get("limited", False), "run": RUN_ID})
    if role in WRITE_ROLES:
        notes = [ln.split(":", 1)[1].strip() for ln in r.get("text", "").splitlines() if ln.strip().lower().startswith("needs-source:")]
        if notes:
            st["needs_source"] = sorted(set(st.get("needs_source", [])) | set(notes))
    save(st)
    return r


def block(st, why):
    st["status"] = "blocked"
    st["blocked"] = why
    save(st)
    log({"job": "write", "slug": st["slug"], "blocked": why})
    return "blocked"


def process(item, ladder, max_rounds=6):
    """Write one item. A gate that fails for a reason of the machine blocks the item (ENV_FAULTS)."""
    try:
        return _process(item, ladder, max_rounds)
    except Wait as w:
        return block(state(item["slug"]), str(w))


def _process(item, ladder, max_rounds=6):
    slug = item["slug"]
    if item["kind"] in CARD_ONLY:
        return "card-only"
    st = state(slug)
    if st["status"] == "done" or (st["status"] == "failed" and st.get("prompt") == VERSION):
        return st["status"]
    st.update(status="writing", prompt=VERSION, models_version=models().version)
    st.pop("blocked", None)
    save(st)
    wfile = RUNS / slug / "writer-prompt.md"
    wfile.parent.mkdir(parents=True, exist_ok=True)
    wprompt = writer_prompt(item)
    wfile.write_text(wprompt, encoding="utf-8")
    sel = pick(ladder, 0)
    if not sel:
        return block(st, "no writer of the ladder is available")
    rung, backend, model = sel
    if written_sections(REPO / node_dir(item) / "node.md") < 3:  # a resumed item keeps its draft
        r = call(item, st, "write", backend, model, wprompt)
        if r.get("limited"):
            sel = pick(ladder, rung + 1)
            if not sel:
                return block(st, "the writer is rate-limited and no higher rung is available")
            rung, backend, model = sel
            call(item, st, "write", backend, model, wprompt)
    audits, gate_fails = 0, 0
    for _ in range(max_rounds + 4):
        probs = gate(item)
        if probs:
            st["last_gate"] = probs
            save(st)
            gate_fails += 1
            if gate_fails > 4:
                break
            if gate_fails >= 3:  # two failed gate fixes: climb
                sel = climb(ladder, rung, st, sel)
                rung, backend, model = sel
                gate_fails = 1
            call(item, st, "fix-gate", backend, model, fix_prompt(item, "\n".join(f"- {p}" for p in probs), wfile))
            continue
        try:
            ab, am = models().auditor_for(writers(st))
        except Wait as w:
            return block(st, str(w))
        r = call(item, st, "audit", ab, am, audit_prompt(item), write=False)
        try:
            v = extract_json(r["text"])
        except ValueError:
            v = {"verdict": "fail", "issues": [], "score": 0, "parse_error": True}
        if isinstance(v, list):
            v = next((x for x in v if isinstance(x, dict) and "verdict" in x), {"verdict": "fail", "issues": [], "parse_error": True})
        audits += 1
        st.setdefault("audits", []).append({"by": f"{ab}:{am}", "verdict": v.get("verdict"), "score": v.get("score"),
                                            "n": len(v.get("issues", [])), "run": RUN_ID})
        if v.get("parse_error"):
            continue
        serious = [x for x in v.get("issues", []) if x.get("severity") in ("blocker", "major")]
        if v.get("verdict") == "pass" and not serious:
            minor = v.get("issues", [])
            if minor:  # one cheap round for minor issues, then the gate again
                call(item, st, "fix-minor", backend, model, fix_prompt(item, fmt_issues(minor), wfile))
                if gate(item):
                    continue
            cite = [p for p in gate(item, final=True) if "aik cite" in p]
            if cite:
                st["last_gate"] = cite
                save(st)
                call(item, st, "fix-gate", backend, model, fix_prompt(item, "\n".join(f"- {p}" for p in cite), wfile))
                continue
            st["status"] = "done"
            st["final"] = {"writer": f"{backend}:{model}", "writers": writers(st), "auditor": f"{ab}:{am}", "score": v.get("score")}
            save(st)
            write_prov(item, st)
            return "done"
        st["last_audit"] = v
        save(st)
        if audits >= 2:  # two failed audits: climb
            sel = climb(ladder, rung, st, sel)
            rung, backend, model = sel
        if audits >= 4:
            break
        call(item, st, "fix-audit", backend, model, fix_prompt(item, fmt_issues(v.get("issues", [])), wfile))
    st["status"] = "failed"
    save(st)
    return "failed"


def write_prov(item, st):
    """One line per file of the node directory in prov/<tag>.jsonl (spec 04.8, 14.7)."""
    d = REPO / node_dir(item)
    tag = tag_of(item)
    lines = []
    for f in sorted(d.rglob("*")):
        if f.is_file() and "__pycache__" not in f.parts and f.suffix != ".pyc":
            h = hashlib.sha256(f.read_bytes()).hexdigest()
            lines.append(json.dumps({
                "date": time.strftime("%Y-%m-%d"), "file": str(f.relative_to(d)), "op": "create", "hash": f"sha256:{h}",
                "author": {"agent": "writer", "models": writers(st), "harness": "ai-agent/work/writer.py",
                           "prompt": f"writer/{VERSION}", "run": RUN_ID, "models_version": models().version},
                "audit": st.get("final", {}).get("auditor"), "reviewer": None, "sources": sources_of(item)}, ensure_ascii=False))
    (REPO / "prov").mkdir(exist_ok=True)
    with open(REPO / "prov" / f"{tag}.jsonl", "a", encoding="utf-8") as fh:
        fh.write("\n".join(lines) + "\n")


def queue(all_items, limit):
    """Core first, then by level, then the kinds that others link to first (spec 14.5). aik queue replaces this in M1."""
    order = {"intro": 0, "core": 1, "advanced": 2, "research": 3}
    kord = {k: i for i, k in enumerate(["area", "definition", "concept", "task", "metric", "method", "architecture",
                                        "theorem", "finding", "argument", "open-problem", "risk"])}

    def is_stub(it):
        p = REPO / node_dir(it) / "node.md"
        return p.exists() and written_sections(p) < 3

    todo = [it for it in all_items.values() if it["kind"] not in CARD_ONLY and state(it["slug"])["status"] in ("todo", "blocked") and is_stub(it)]
    todo.sort(key=lambda it: (not it.get("core"), order.get(it.get("level"), 2), kord.get(it["kind"], 99), it["slug"]))
    return todo[:limit]


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--slugs", nargs="*")
    ap.add_argument("--queue", action="store_true")
    ap.add_argument("--limit", type=int, default=20)
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--writers", default="", help="a comma list of backend:model; the default is the ladder of models.yaml")
    args = ap.parse_args()
    if not args.slugs and not args.queue:
        ap.error("give --slugs or --queue")
    need_aik("gate", "14.6")
    need_aik("cite", "14.6")
    all_items = load_catalog()
    missing = [s for s in args.slugs or [] if s not in all_items]
    if missing:
        raise SystemExit(f"not in the catalog or the content checkout: {', '.join(missing)}")
    items = [all_items[s] for s in args.slugs] if args.slugs else queue(all_items, args.limit)
    override = [x for x in args.writers.split(",") if x]

    def one(it):
        ladder = override or models().ladder(hard=it["kind"] in HARD)
        try:
            res = process(it, ladder)
        except SystemExit:
            raise
        except Exception as e:  # noqa: BLE001
            res = f"error: {e}"
            log({"job": "write", "slug": it["slug"], "error": str(e)})
        print(f"{it['slug']}: {res}", flush=True)
        return res

    with ThreadPoolExecutor(args.jobs) as pool:
        list(pool.map(one, items))


if __name__ == "__main__":
    main()
