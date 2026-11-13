#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Ship one group of pages to tamnd/ai (spec 14.10): write provenance when asked, run pr.py --merge with up to 3 tries on a fresh checkout, run deploy.sh when the deploy inside pr.py failed, and on success tick the pages in the issue.
#
# Usage: bash ship.sh <issue> <branch> <title> <bodyfile> <prov:0|1> <slug>...
# Environment: MS (the milestone, for example "Area: Deep learning training"), AI_PR_TREE (default: ai-pr next to the content checkout), AI_REPO (default ../../ai).
# The credentials of the deploy come only from AI_ENV_FILE (default $HOME/data/.env). This script never prints them.
set -u
cd "$(dirname "$0")" || exit 1
if [ $# -lt 6 ]; then
  echo "usage: bash ship.sh <issue> <branch> <title> <bodyfile> <prov:0|1> <slug>..." >&2
  exit 2
fi
ISSUE=$1 BR=$2 TI=$3 BODY=$4 PV=$5
shift 5
REPO_DIR=$(cd "${AI_REPO:-../../ai}" && pwd) || exit 1
PRTREE=${AI_PR_TREE:-$(dirname "$REPO_DIR")/ai-pr}
unset GH_TOKEN GITHUB_TOKEN
if [ "$PV" = 1 ]; then
  python3 prov.py --note "full page, fixed after the audit and checked by a second audit" "$@" > /dev/null || exit 1
fi
MSARGS=()
if [ -n "${MS:-}" ]; then MSARGS=(--milestone "$MS"); fi
PAGES=()
for s in "$@"; do PAGES+=("/$s/"); done
for try in 1 2 3; do
  if [ -d "$PRTREE" ]; then
    (cd "$PRTREE" && git checkout -q -- . && git clean -fdq && git fetch -q && git checkout -q --detach origin/main)
  fi
  out=$(python3 pr.py --branch "$BR" --title "$TI" --body-file "$BODY" --slugs "$@" --labels "type: node" ${MSARGS[@]+"${MSARGS[@]}"} --merge 2>&1)
  echo "$out" | tail -3
  if echo "$out" | grep -q "^DEPLOY FAILED"; then
    bash deploy.sh | tail -1
    out="deployed"
  fi
  if echo "$out" | grep -q "^deployed"; then
    python3 report.py tick "$ISSUE" "${PAGES[@]}" | tail -1
    exit 0
  fi
  if [ -d "$PRTREE" ]; then
    (cd "$PRTREE" && git push -q origin --delete "$BR" 2>/dev/null)
  fi
  echo "ship try $try failed" >&2
  sleep 30
done
echo "SHIP FAILED $BR"
exit 1
