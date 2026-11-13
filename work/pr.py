#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Open a pull request in tamnd/ai from files of the content checkout, merge it on request, and deploy (spec 14.10).

Usage:
  python3 pr.py --branch content/regularization --title "..." --body-file b.md --paths registers/core.yaml \
      [--labels "type: node"] [--milestone "Area: Deep learning training"] [--merge]
  python3 pr.py --slugs dropout label-smoothing ... (copies the node directories, their cards and their prov files)

The PR tree is a clean worktree of origin/main at AI_PR_TREE (default: ai-pr next to the content checkout), made from AI_MAIN (default: the content checkout).
`aik fmt` and `aik check` run in the PR tree before the commit. The body gets the gate and audit summary table of the nodes.
Labels: type: node, area: <top>, kind: <kind> and priority: p0 for core items are added from --slugs.
--merge squash-merges after the local check, then deploys with `aik site deploy`, unless NODEPLOY is set. The Cloudflare credentials come only from the environment file AI_ENV_FILE (default $HOME/data/.env), and are never printed.
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from lib import AIK, GH_REPO, REPO, WORK, front_matter

PRTREE = Path(os.environ.get("AI_PR_TREE", REPO.parent / "ai-pr")).resolve()
MAIN = Path(os.environ.get("AI_MAIN", REPO)).resolve()
ENV_FILE = os.environ.get("AI_ENV_FILE", str(Path.home() / "data" / ".env"))
STATE = WORK / "state"


def sh(cmd, cwd=PRTREE, check=True):
    p = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if check and p.returncode:
        raise SystemExit(f"{' '.join(cmd)}\n{p.stdout}\n{p.stderr}")
    return p.stdout.strip()


def prepare(branch):
    if not PRTREE.exists():
        sh(["git", "worktree", "add", "--detach", str(PRTREE), "origin/main"], cwd=MAIN)
    sh(["git", "fetch", "-q", "origin"])
    sh(["git", "checkout", "-q", "-B", branch, "origin/main"])
    sh(["git", "clean", "-fdq"])


def copy(rel):
    src, dst = REPO / rel, PRTREE / rel
    if src.is_dir():
        shutil.copytree(src, dst, dirs_exist_ok=True, ignore=shutil.ignore_patterns("__pycache__", ".pytest_cache", "*.pyc"))
    else:
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, dst)


def nodes(slugs):
    """[(slug, node.md path, front matter)] for the slugs that have a node."""
    out = []
    for s in slugs:
        hits = list((REPO / "nodes").glob(f"*/*/{s}/node.md"))
        if hits:
            out.append((s, hits[0], front_matter(hits[0])))
        else:
            print(f"warning: no node {s}", file=sys.stderr)
    return out


def node_paths(found):
    out = []
    for _, p, fm in found:
        out.append(str(p.parent.relative_to(REPO)))
        tag = fm.get("tag", "")
        if tag and (REPO / "prov" / f"{tag}.jsonl").exists():
            out.append(f"prov/{tag}.jsonl")
    for f in sorted((REPO / "cards").glob("*/*.jsonl")):
        if f.parent.name == "bridge":
            continue
        rows = [json.loads(ln) for ln in f.read_text(encoding="utf-8").splitlines() if ln.strip()]
        if any(r.get("slug") in {s for s, _, _ in found} for r in rows):
            out.append(str(f.relative_to(REPO)))
    return out


def labels_for(found, core):
    out = set()
    for s, p, fm in found:
        out.add(f"area: {str(p.parent.relative_to(REPO / 'nodes')).split('/')[0]}")
        if fm.get("kind"):
            out.add(f"kind: {fm['kind']}")
        if s in core:
            out.add("priority: p0")
    return sorted(out)


def core_slugs():
    p = WORK / "catalog" / "curated.jsonl"
    if not p.exists():
        return set()
    return {json.loads(ln)["slug"] for ln in p.read_text(encoding="utf-8").splitlines() if ln.strip() and json.loads(ln).get("core")}


def ladders(slug, tree):
    """The ladder labels of a node in a tree, from `aik status -json`, as one short string such as "W3 H0 V1"."""
    try:
        p = subprocess.run([AIK, "status", "-json", slug], cwd=tree, capture_output=True, text=True, timeout=120)
        d = json.loads(p.stdout[p.stdout.find("{"):]) if p.returncode == 0 and "{" in p.stdout else {}
    except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError):
        return "none"
    out = []
    for k, v in d.items():
        if isinstance(v, dict) and "label" in v:
            out.append(v["label"])
        elif k == "i" and isinstance(v, dict):
            out += [f"{lang}:{r.get('label')}" for lang, r in sorted(v.items()) if isinstance(r, dict)]
    return " ".join(out) or "none"


def summary(found, before):
    """The gate and audit summary table of spec 14.10."""
    rows = ["| node | ladders before | ladders after | writers | auditor | audit score | status |", "|---|---|---|---|---|---|---|"]
    for s, _, _ in found:
        p = STATE / f"{s}.json"
        st = json.loads(p.read_text(encoding="utf-8")) if p.exists() else {}
        fin = st.get("final", {})
        rows.append(f"| {s} | {before.get(s, 'none')} | {ladders(s, PRTREE)} | {', '.join(fin.get('writers', [])) or fin.get('writer', '?')} | "
                    f"{fin.get('auditor', '?')} | {fin.get('score', '?')} | {st.get('status', '?')} |")
    return "\n".join(rows)


def deploy(head):
    """aik site deploy with the credentials of the environment file. The output is cut to its last lines and holds no secret."""
    script = 'set -a; . "$AI_ENV_FILE"; set +a; "$AIK" site deploy -out build/site -commit "$1" 2>&1 | tail -3'
    env = dict(os.environ, AI_ENV_FILE=ENV_FILE, AIK=AIK)
    p = subprocess.run(["bash", "-c", script, "deploy", head], cwd=PRTREE, capture_output=True, text=True, env=env)
    return p.stdout + p.stderr


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--branch", required=True)
    ap.add_argument("--title", required=True)
    ap.add_argument("--body-file", required=True)
    ap.add_argument("--paths", nargs="*", default=[])
    ap.add_argument("--slugs", nargs="*", default=[])
    ap.add_argument("--labels", nargs="*", default=["type: node"])
    ap.add_argument("--milestone")
    ap.add_argument("--merge", action="store_true")
    args = ap.parse_args()
    found = nodes(args.slugs)
    prepare(args.branch)
    before = {s: ladders(s, PRTREE) for s, _, _ in found}
    for p in args.paths + node_paths(found):
        copy(p)
    rc = subprocess.run([AIK, "fmt"], cwd=PRTREE, capture_output=True, text=True)
    if rc.returncode:
        raise SystemExit(f"aik fmt failed:\n{rc.stdout[-2000:]}{rc.stderr[-1000:]}")
    rc = subprocess.run([AIK, "check"], cwd=PRTREE, capture_output=True, text=True)
    last = rc.stdout.strip().splitlines()[-1] if rc.stdout.strip() else rc.stderr[-500:]
    print("check:", last)
    if rc.returncode:
        raise SystemExit(rc.stdout[-3000:])
    sh(["git", "add", "-A"])
    sh(["git", "commit", "-q", "-s", "-m", args.title])
    sh(["git", "push", "-q", "-f", "origin", args.branch])
    body = Path(args.body_file).read_text(encoding="utf-8")
    if found:
        body = body.rstrip() + "\n\n## Gate and audit\n\n" + summary(found, before) + "\n"
    with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
        f.write(body)
        body_file = f.name
    cmd = ["gh", "pr", "create", "-R", GH_REPO, "--base", "main", "--head", args.branch, "--title", args.title, "--body-file", body_file]
    for lb in sorted(set(args.labels) | set(labels_for(found, core_slugs()))):
        cmd += ["--label", lb]
    if args.milestone:
        cmd += ["--milestone", args.milestone]
    url = ""
    for _ in range(4):
        r = subprocess.run(cmd, cwd=PRTREE, capture_output=True, text=True)
        url = r.stdout.strip() if r.returncode == 0 else ""
        if not url:
            time.sleep(20)
            url = sh(["gh", "pr", "list", "-R", GH_REPO, "--head", args.branch, "--state", "open", "--json", "url", "-q", ".[0].url"], check=False)
        if url:
            break
    os.unlink(body_file)
    if not url:
        raise SystemExit("pull request create failed for " + args.branch)
    num = url.rstrip("/").split("/")[-1]
    print(url, flush=True)
    if not args.merge:
        return
    # CI is slow, so merge right away: the local aik check above is the gate, and CI confirms after the merge
    sh(["gh", "pr", "merge", num, "-R", GH_REPO, "--squash", "--admin", "--delete-branch=false"])
    print("merged", num)
    sh(["git", "fetch", "-q", "origin"])
    sh(["git", "checkout", "-q", "--detach", "origin/main"])
    head = sh(["git", "rev-parse", "HEAD"]).strip()
    if os.environ.get("NODEPLOY"):
        print("deployed-later", head)  # the queue deploys once, after the merges
        return
    dep = deploy(head)
    if "deployed to" not in dep:
        print("DEPLOY FAILED", head, re.sub(r"\s+", " ", dep)[-300:])
        sys.exit(2)
    print("deployed", head)


if __name__ == "__main__":
    main()
