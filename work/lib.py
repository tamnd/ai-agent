#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Shared helpers of the ai.tamnd.com content harness: paths, model backends, the model register, the event log, JSON extraction and calls to aik.

This is a module, not a command. The other scripts import it.
The model names come from registers/models.yaml of the content repository (spec 14.4).
Each model call appends one line with its tokens and cost to logs/events.jsonl (spec 14.7).
"""
import fnmatch
import json
import os
import re
import subprocess
import tempfile
import threading
import time
from pathlib import Path

try:
    import yaml
except ImportError:  # the scripts that need YAML call need_yaml(), which stops with a clear message
    yaml = None

WORK = Path(__file__).resolve().parent
REPO = Path(os.environ.get("AI_REPO", WORK / ".." / ".." / "ai")).resolve()
SPEC = Path(os.environ.get("AI_SPEC", WORK.parent / "spec")).resolve()
AIK = os.environ.get("AIK", "aik")
SITE = os.environ.get("AI_SITE", "https://ai.tamnd.com").rstrip("/")
GH_REPO = os.environ.get("AI_GH_REPO", "tamnd/ai")
LOG = WORK / "logs"
BACKENDS = ("claude", "codex")
LIMIT_WAIT = 900  # seconds that a backend rests after it reports a rate limit

_lock = threading.Lock()
_blocked_until = {b: 0.0 for b in BACKENDS}


def need_yaml():
    """Return the PyYAML module, or stop with a clear message when it is not installed."""
    if yaml is None:
        raise SystemExit("PyYAML is not installed. The harness reads YAML registers. Install it with: python3 -m pip install pyyaml")
    return yaml


def load_yaml(path):
    """Read a YAML file. Stop with a clear message when the file is not there."""
    y = need_yaml()
    p = Path(path)
    if not p.exists():
        raise SystemExit(f"{p} does not exist. Set AI_REPO to the checkout of tamnd/ai (now {REPO}).")
    with open(p, encoding="utf-8") as f:
        return y.safe_load(f)


def front_matter(path):
    """The front matter of a node.md file as a dict."""
    text = Path(path).read_text(encoding="utf-8")
    m = re.match(r"---\n(.*?)\n---\n", text, re.S)
    return need_yaml().safe_load(m.group(1)) if m else {}


# The event log


def log(event: dict) -> None:
    event = {"t": time.strftime("%Y-%m-%dT%H:%M:%S"), **event}
    with _lock:
        LOG.mkdir(exist_ok=True)
        with open(LOG / "events.jsonl", "a", encoding="utf-8") as f:
            f.write(json.dumps(event, ensure_ascii=False) + "\n")


# Availability of the backends


def available(backend: str, model: str = "") -> bool:
    """True when the backend did not report a rate limit in the last LIMIT_WAIT seconds."""
    return time.time() >= _blocked_until.get(backend, 0.0)


def codex_available() -> bool:
    return available("codex")


def _block(backend: str) -> None:
    _blocked_until[backend] = time.time() + LIMIT_WAIT


# The model register (spec 14.4)


class Wait(Exception):
    """No model can do this role now without a break of the cross-family rule. The item waits."""


def family(ref: str) -> str:
    """The family of a model reference: claude or the name of the other backend."""
    return ref.split(":", 1)[0] if ":" in ref else ("claude" if ref.startswith("claude") else ref)


def register_id(backend: str, model: str) -> str:
    """The form that models.yaml uses: a bare id for Claude, backend:model for the other family."""
    return model if backend == "claude" else f"{backend}:{model}"


class Models:
    """The content of registers/models.yaml, with the rules of spec 14.4."""

    def __init__(self, data: dict, path: str = "models.yaml"):
        if not isinstance(data, dict) or "classes" not in data:
            raise SystemExit(f"{path} has no classes block; see spec 14.4")
        self.data = data
        self.path = path
        self.version = str(data.get("version", "unknown"))

    def resolve(self, ref: str):
        """Turn a reference of the register into (backend, model).

        A reference is a model id (claude-sonnet-5-5), a backend and a model (codex:gpt-6-sol), other.<role>, or a class name (large), which gives the first model of the class.
        """
        ref = str(ref).strip()
        classes = self.data["classes"]
        if ref.startswith("other."):
            other = classes.get("other") or {}
            role = ref.split(".", 1)[1]
            if role not in other:
                raise SystemExit(f"{self.path}: classes.other has no {role!r}")
            return self.resolve(other[role])
        if ref in classes and isinstance(classes[ref], list) and classes[ref]:
            return self.resolve(classes[ref][0])
        if ":" in ref:
            backend, model = ref.split(":", 1)
            if backend not in BACKENDS:
                raise SystemExit(f"{self.path}: unknown backend {backend!r} in {ref!r}")
            return backend, model
        if ref.startswith("claude-"):
            return "claude", ref
        raise SystemExit(f"{self.path}: cannot resolve the model reference {ref!r}")

    def ref(self, ref: str) -> str:
        """The reference as backend:model."""
        return ":".join(self.resolve(ref))

    def role(self, name: str) -> list:
        """The models of a role (curator, cataloguer, verifier, ...), each as backend:model."""
        v = self.data.get(name)
        if v is None:
            raise SystemExit(f"{self.path} has no role {name!r}")
        return [self.ref(x) for x in (v if isinstance(v, list) else [v])]

    def ladder(self, hard: bool = False) -> list:
        """The writer ladder, each rung as backend:model. The hard kinds use writer_ladder_hard."""
        key = "writer_ladder_hard" if hard else "writer_ladder"
        if key not in self.data:
            raise SystemExit(f"{self.path} has no {key}")
        return [self.ref(x) for x in self.data[key]]

    def auditor_for(self, writers, is_available=available):
        """The auditor (backend, model) for a node, or raise Wait.

        writers is one backend:model or a list of them: every model that wrote a file of the node.
        The auditor is never of the family of a writer (spec 14.2, 15.4).
        When the auditor of the other family is not available, the fallback of the register is used only when it is not of a writer's family. Else the item waits (spec 14.4, Q-08).
        """
        writers = [writers] if isinstance(writers, str) else [w for w in writers if w]
        if not writers:
            raise Wait("no writer is known, so the cross-family rule cannot be checked")
        rules = dict(self.data.get("auditor_for") or {})
        fallback = rules.pop("fallback_if_other_unavailable", None)
        fams = {family(w) for w in writers}
        primaries = []
        for w in writers:
            b, m = w.split(":", 1) if ":" in w else self.resolve(w)
            rid = register_id(b, m)
            hit = next((v for pat, v in rules.items() if fnmatch.fnmatchcase(rid, pat)), None)
            if hit is None:
                raise Wait(f"auditor_for in {self.path} has no rule for the writer {rid}")
            primaries.append(self.resolve(hit))
        allowed = [p for p in dict.fromkeys(primaries) if p[0] not in fams]
        if not allowed:
            raise Wait(f"the writers {sorted(set(writers))} are of the families {sorted(fams)}, so no auditor of another family is pinned (spec 15.4, Q-08)")
        for p in allowed:
            if is_available(*p):
                return p
        if fallback:
            fb = self.resolve(fallback)
            if fb[0] not in fams and is_available(*fb):
                return fb
        raise Wait(f"the auditor {':'.join(allowed[0])} is not available, and the fallback is of a writer's family; the item waits (spec 14.4, Q-08)")


_models_cache = {}


def models(path=None) -> Models:
    """Read registers/models.yaml of the content repository, or the file of AI_MODELS or path."""
    p = Path(path or os.environ.get("AI_MODELS") or REPO / "registers" / "models.yaml")
    key = str(p)
    if key not in _models_cache:
        _models_cache[key] = Models(load_yaml(p), path=key)
    return _models_cache[key]


def split_ref(ref: str):
    """backend:model to (backend, model). A bare Claude id is accepted."""
    if ":" in ref:
        b, m = ref.split(":", 1)
        return b, m
    return models().resolve(ref)


# The backends


def run_codex(prompt: str, model: str, cwd: Path = REPO, timeout: int = 1800,
              effort: str = "medium", write: bool = True) -> dict:
    """Run codex exec. Returns {ok, text, limited, secs, tokens_in, tokens_out, cost}."""
    fd, name = tempfile.mkstemp(prefix="codex-", suffix=".txt")
    os.close(fd)
    out = Path(name)
    cmd = ["codex", "exec", "-m", model, "-c", f"model_reasoning_effort={effort}",
           "--skip-git-repo-check", "--ephemeral", "-C", str(cwd),
           "--output-last-message", str(out)]
    cmd += ["--sandbox", "workspace-write"] if write else ["--sandbox", "read-only"]
    t0 = time.time()
    try:
        p = subprocess.run(cmd + ["-"], input=prompt, capture_output=True, text=True, timeout=timeout)
        err = p.stdout[-3000:] + p.stderr[-3000:]
    except subprocess.TimeoutExpired:
        out.unlink(missing_ok=True)
        return {"ok": False, "text": "", "limited": False, "secs": timeout, "err": "timeout", "cost": None}
    text = out.read_text() if out.exists() else ""
    out.unlink(missing_ok=True)
    limited = "usage limit" in err or "rate limit" in err.lower()
    if limited:
        _block("codex")
    m = re.search(r"tokens used[:\s]+([\d,]+)", err, re.I)
    tokens = int(m.group(1).replace(",", "")) if m else None
    # codex exec prints one total; the split into input and output is not known. The price of the other family is not in this harness (spec 14.8).
    return {"ok": p.returncode == 0 and not limited, "text": text, "limited": limited,
            "secs": round(time.time() - t0), "tokens_in": None, "tokens_out": None, "tokens": tokens,
            "cost": None, "err": "" if p.returncode == 0 else err[-1500:]}


def run_claude(prompt: str, model: str, cwd: Path = REPO, timeout: int = 1800,
               write: bool = True, max_turns: int = 80) -> dict:
    """Run claude -p. Returns {ok, text, cost, secs, tokens_in, tokens_out, limited}."""
    cmd = ["claude", "-p", "--model", model, "--output-format", "json", "--max-turns", str(max_turns)]
    if write:
        cmd += ["--permission-mode", "bypassPermissions"]
    else:
        cmd += ["--allowedTools", "Read,Grep,Glob"]
    t0 = time.time()
    try:
        p = subprocess.run(cmd, input=prompt, capture_output=True, text=True, timeout=timeout, cwd=cwd)
    except subprocess.TimeoutExpired:
        return {"ok": False, "text": "", "cost": 0, "secs": timeout, "err": "timeout"}
    try:
        d = json.loads(p.stdout)
    except json.JSONDecodeError:
        return {"ok": False, "text": p.stdout[-2000:], "cost": 0, "secs": round(time.time() - t0),
                "err": p.stderr[-1500:]}
    limited = bool("limit" in str(d.get("result", "")).lower() and d.get("is_error"))
    if limited:
        _block("claude")
    u = d.get("usage") or {}
    tin = sum(int(u.get(k) or 0) for k in ("input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"))
    return {"ok": not d.get("is_error"), "text": d.get("result", ""), "cost": d.get("total_cost_usd", 0),
            "secs": round(time.time() - t0), "limited": limited, "tokens_in": tin or None,
            "tokens_out": u.get("output_tokens"), "err": "" if not d.get("is_error") else str(d)[-1500:]}


def run_model(prompt: str, backend: str, model: str, job: str = "", slug: str = "", role: str = "", **kw) -> dict:
    """Run one model call and log it to logs/events.jsonl with its tokens and cost."""
    if backend == "codex":
        kw.pop("max_turns", None)
        r = run_codex(prompt, model=model, **kw)
    elif backend == "claude":
        kw.pop("effort", None)
        r = run_claude(prompt, model=model, **kw)
    else:
        raise SystemExit(f"unknown backend {backend!r}; use one of {', '.join(BACKENDS)}")
    ev = {"job": job, "slug": slug, "role": role, "backend": backend, "model": model,
          "ok": r.get("ok", False), "secs": r.get("secs"), "tokens_in": r.get("tokens_in"),
          "tokens_out": r.get("tokens_out"), "cost": r.get("cost"), "limited": r.get("limited", False)}
    if r.get("tokens") is not None:
        ev["tokens"] = r["tokens"]
    try:
        ev["models_version"] = models().version
    except SystemExit:
        pass
    log({k: v for k, v in ev.items() if v not in ("", None) or k in ("cost",)})
    return r


def extract_json(text: str):
    """Take the last JSON object or array in a model reply."""
    m = re.search(r"```(?:json)?\s*(.*?)```", text, re.S)
    if m:
        try:
            return json.loads(m.group(1))
        except json.JSONDecodeError:
            pass
    try:
        return json.loads(text.strip())
    except json.JSONDecodeError:
        pass
    pairs = [("[", "]"), ("{", "}")]
    pairs.sort(key=lambda p: text.find(p[0]) if text.find(p[0]) != -1 else len(text))
    for opener, closer in pairs:
        i = text.find(opener)
        j = text.rfind(closer)
        if i != -1 and j > i:
            try:
                return json.loads(text[i:j + 1])
            except json.JSONDecodeError:
                continue
    raise ValueError("no JSON in reply")


# Calls to aik


class MissingCommand(SystemExit):
    """aik does not have a command or a flag that the harness needs yet."""


_aik_help = {}


def _aik_text(*args) -> str:
    key = " ".join(args)
    if key not in _aik_help:
        try:
            p = subprocess.run([AIK, *args], capture_output=True, text=True, timeout=120)
            _aik_help[key] = p.stdout + p.stderr
        except FileNotFoundError:
            raise SystemExit(f"the binary {AIK!r} is not on PATH. Build it with `go install ./cmd/aik` in tamnd/ai-agent, or set AIK.") from None
    return _aik_help[key]


def aik_has(command: str, flag: str = "") -> bool:
    """True when aik has the command (for example gate, or card sync) and, when given, the flag."""
    words = command.split()
    listed = re.search(r"^Commands:\n(.*?)\n\n", _aik_text("help"), re.S | re.M)
    names = {ln.split()[0] for ln in listed.group(1).splitlines() if ln.strip()} if listed else set()
    if words[0] not in names:
        return False
    if len(words) > 1:
        usage = _aik_text(words[0])
        if f"unknown subcommand \"{words[1]}\"" in usage or re.search(rf"\b{re.escape(words[1])}\b", usage) is None:
            return False
    if flag:
        return re.search(rf"^\s+-{re.escape(flag.lstrip('-'))}\b", _aik_text(*words, "-h"), re.M) is not None
    return True


def need_aik(command: str, section: str, flag: str = "") -> None:
    """Stop with a clear message when aik does not have the command or the flag yet."""
    if not aik_has(command, flag):
        what = f"`aik {command}` has no flag -{flag.lstrip('-')}" if flag and aik_has(command) else f"aik has no command `aik {command}`"
        raise MissingCommand(f"{what} yet. The harness needs it (spec {section}). Add it to tamnd/ai-agent, or run a newer aik (now {AIK!r}).")


def aik(*args, cwd=REPO, timeout=600):
    """Run aik. Returns (exit code, stdout and stderr)."""
    try:
        p = subprocess.run([AIK, *args], cwd=cwd, capture_output=True, text=True, timeout=timeout)
        return p.returncode, p.stdout + p.stderr
    except subprocess.TimeoutExpired:
        return 1, f"timeout after {timeout}s: aik {' '.join(args)}"
