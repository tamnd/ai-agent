#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Report progress on GitHub in tamnd/ai: tick checklist lines in an issue, post a comment, cut a release (spec 14.10).

Usage:
  python3 report.py tick 39 /dropout/ /lora/              # tick the checklist lines that contain the text
  python3 report.py comment 39 --body-file c.md
  python3 report.py release 2027.03 --notes-file n.md [--latest]
  python3 report.py next-patch                            # print the next patch version, for example 2027.03.1
The repository is AI_GH_REPO (default tamnd/ai). Releases use calendar versions (spec 20): YYYY.MM, and YYYY.MM.N for a patch.
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import tempfile

from lib import GH_REPO

CALVER = re.compile(r"v?(\d{4})\.(\d{2})(?:\.(\d+))?")


def gh(*args, input=None):
    p = subprocess.run(["gh", *args], capture_output=True, text=True, input=input)
    if p.returncode:
        raise SystemExit(p.stderr)
    return p.stdout.strip()


def check_text(text):
    """The house style: no em dash, no en dash used as a dash."""
    bad = [c for c in ("\u2014", "\u2013") if c in text]
    if bad:
        raise SystemExit(f"text has a dash character {bad}; rewrite it")
    return text


def tick(num, needles):
    body = json.loads(gh("issue", "view", str(num), "-R", GH_REPO, "--json", "body"))["body"]
    n = 0
    for needle in needles:
        pat = re.compile(r"^(\s*[-*] )\[ \](.*" + re.escape(needle) + r".*)$", re.M)
        body, k = pat.subn(r"\1[x]\2", body)
        n += k
    with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as f:
        f.write(body)
    gh("issue", "edit", str(num), "-R", GH_REPO, "--body-file", f.name)
    os.unlink(f.name)
    print(f"ticked {n} lines in #{num}")


def next_patch(tags, today=None):
    """The next patch of the latest calendar release: 2027.03 gives 2027.03.1, and 2027.03.1 gives 2027.03.2.
    With no release yet, the version of the current month."""
    vers = []
    for t in tags:
        m = CALVER.fullmatch(t)
        if m:
            vers.append((int(m.group(1)), int(m.group(2)), int(m.group(3) or 0)))
    if not vers:
        today = today or datetime.date.today()
        return f"{today.year}.{today.month:02d}"
    y, mo, p = max(vers)
    return f"{y}.{mo:02d}.{p + 1}"


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = ap.add_subparsers(dest="cmd", required=True)
    t = sub.add_parser("tick")
    t.add_argument("issue", type=int)
    t.add_argument("needles", nargs="+")
    c = sub.add_parser("comment")
    c.add_argument("issue", type=int)
    c.add_argument("--body-file", required=True)
    r = sub.add_parser("release")
    r.add_argument("tag")
    r.add_argument("--notes-file", required=True)
    r.add_argument("--latest", action="store_true")
    sub.add_parser("next-patch")
    a = ap.parse_args()
    if a.cmd == "tick":
        tick(a.issue, a.needles)
    elif a.cmd == "comment":
        with open(a.body_file, encoding="utf-8") as f:
            check_text(f.read())
        print(gh("issue", "comment", str(a.issue), "-R", GH_REPO, "--body-file", a.body_file))
    elif a.cmd == "release":
        if not CALVER.fullmatch(a.tag) or a.tag.startswith("v"):
            raise SystemExit(f"{a.tag!r} is not a calendar version such as 2027.03 or 2027.03.1 (spec 20)")
        with open(a.notes_file, encoding="utf-8") as f:
            check_text(f.read())
        args = ["release", "create", a.tag, "-R", GH_REPO, "--target", "main", "--title", a.tag, "--notes-file", a.notes_file]
        if a.latest:
            args.append("--latest")
        print(gh(*args))
    else:
        tags = json.loads(gh("release", "list", "-R", GH_REPO, "--limit", "100", "--json", "tagName") or "[]")
        print(next_patch([x["tagName"] for x in tags]))


if __name__ == "__main__":
    main()
