#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Create the milestone, the area issue and one issue per sub-area of a top area in tamnd/ai, with page checklists (spec 14.10).

Usage: python3 mkarea.py <top area id> [--dry]
The milestone is "Area: <Name>". Each checklist line is `- [ ] /<slug>/ (<kind>, <level>[, core])`, sorted with the core first, then by level, then by name.
The bar is the gold page of each kind (kinds.py). Set AI_PARENT_ISSUE to add "Part of #<n>." to the area issue.
--dry prints the sub-areas with their page counts and creates nothing.
"""
import argparse
import json
import os
import subprocess

from kinds import CARD_ONLY, gold_slugs
from lib import GH_REPO, REPO, SITE, front_matter, load_yaml

LEVELS = {"intro": 0, "core": 1, "advanced": 2, "research": 3}


def gh(*args, input=None):
    r = subprocess.run(["gh", *args], input=input, capture_output=True, text=True)
    if r.returncode:
        raise SystemExit(r.stderr)
    return r.stdout.strip()


def core_tags():
    p = REPO / "registers" / "core.yaml"
    if not p.exists():
        return set()
    d = load_yaml(p) or {}
    rows = d.get("items", []) if isinstance(d, dict) else d
    return {r["node"] for r in rows or [] if isinstance(r, dict) and r.get("node")}


def bar(kinds):
    """The gold pages of the kinds of a sub-area, as Markdown links."""
    out = []
    for k in sorted(kinds):
        g = gold_slugs(k)
        out.append(f"{k}: " + ", ".join(f"[{s}]({SITE}/{s}/)" for s in g))
    return "; ".join(out)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("area")
    ap.add_argument("--dry", action="store_true")
    a = ap.parse_args()
    areas = load_yaml(REPO / "topics" / "areas.yaml")
    top = next((x for x in areas if x["id"] == a.area), None)
    if not top:
        raise SystemExit(f"no top area {a.area!r} in topics/areas.yaml")
    core = core_tags()
    title = f"Area: {top['name']}"
    label = f"area: {a.area}"
    subs = []
    for s in top.get("children", []):
        rows = []
        for p in sorted((REPO / "nodes" / a.area / s["id"]).glob("*/node.md")):
            f = front_matter(p)
            if f.get("kind") in CARD_ONLY:
                continue
            c = f.get("tag") in core
            rows.append((0 if c else 1, LEVELS.get(f.get("level"), 9), f.get("name", ""), f["slug"], f.get("kind", ""), f.get("level", ""), c))
        rows.sort()
        subs.append((s, rows))
    total = sum(len(r) for _, r in subs)
    if a.dry:
        for s, r in subs:
            print(s["name"], len(r))
        print("total", total)
        return
    ms = json.loads(gh("api", f"repos/{GH_REPO}/milestones", "--jq", "[.[].title]") or "[]")
    if title not in ms:
        gh("api", f"repos/{GH_REPO}/milestones", "-f", f"title={title}", "-f",
           f"description=Write each page of the {top['name']} area to the bar of the gold page of its kind: claims with sources, facts from fact records, full derivations, checked examples and a neutral voice.")
    nums = []
    for s, rows in subs:
        lines = [f"- [ ] /{slug}/ ({k}, {lv}{', core' if c else ''})" for _, _, _, slug, k, lv, c in rows]
        body = (f"## Goal\n\nWrite each page of the sub-area {s['name']} to the bar of the gold page of its kind ({bar({r[4] for r in rows}) or 'spec 21, M0'}).\n\n"
                "## The bar\n\n"
                "- Each section of the kind (spec 06.3) is present, in order, and has real content.\n"
                "- Each claim sentence has a record in `claims.yaml` with a type and a source with a locator (spec 07).\n"
                "- Each number that changes is a fact reference, never plain text (spec 04.9).\n"
                "- Each derivation is full, and each example is checked by hand or by a script.\n"
                "- No position in the site voice on a contested question, and no rank or superlative (spec 06.7, 06.9).\n"
                "- `aik check` gives no error, and `aik gate` passes.\n\n"
                "## Pages\n\nThe pages are in writing order: the core first, then by level.\n\n" + "\n".join(lines) + "\n")
        url = gh("issue", "create", "-R", GH_REPO, "--title", f"Write the {top['name']} pages: {s['name']}", "--body", body,
                 "--label", "priority: p0", "--label", "type: node", "--label", label, "--label", "type: tracking", "--milestone", title)
        nums.append((int(url.rsplit("/", 1)[1]), s["name"], len(rows)))
        print(url)
    sublines = "\n".join(f"- [ ] #{n} {name} ({k} pages)" for n, name, k in nums)
    parent = os.environ.get("AI_PARENT_ISSUE")
    body = (f"## Goal\n\nWrite all {total} pages of the [{top['name']}]({SITE}/{a.area}/) area to the bar of the gold page of each kind. Quality is more important than speed.\n\n"
            "## Method\n\n"
            "- One issue for each sub-area, with a checklist of its pages.\n"
            "- One pull request for each sub-area. Each pull request deploys the site when it merges.\n"
            "- A comment on the sub-area issue when a group is done, and on this issue when a sub-area is done.\n"
            "- A patch release when a fix cannot wait, and the monthly release when the area is done.\n\n"
            f"## Sub-areas\n\n{sublines}\n\n## Done when\n\n- [ ] Each page of the area passes the gate.\n- [ ] The monthly release holds the area.\n"
            + (f"\nPart of #{parent}.\n" if parent else ""))
    url = gh("issue", "create", "-R", GH_REPO, "--title", f"Write the {top['name']} area", "--body", body,
             "--label", "priority: p0", "--label", "type: node", "--label", label, "--label", "type: tracking", "--milestone", title)
    print("area", url)


if __name__ == "__main__":
    main()
