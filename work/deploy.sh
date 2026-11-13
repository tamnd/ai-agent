#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Deploy origin/main of tamnd/ai (spec 14.10): wait for 2 GB of free disk, run aik site deploy, then retry until the live gold page shows the commit.
#
# Usage: bash deploy.sh
# Environment: AI_PR_TREE (default: ai-pr next to the content checkout), AI_REPO (default ../../ai), AIK (default aik), AI_SITE (default https://ai.tamnd.com).
# The Cloudflare credentials come only from AI_ENV_FILE (default $HOME/data/.env), read with set -a. This script never prints them.
# It prints "deployed <hash>" or "DEPLOY FAILED".
set -u
cd "$(dirname "$0")" || exit 1
REPO_DIR=$(cd "${AI_REPO:-../../ai}" && pwd) || exit 1
PRTREE=${AI_PR_TREE:-$(dirname "$REPO_DIR")/ai-pr}
ENV_FILE=${AI_ENV_FILE:-$HOME/data/.env}
AIK=${AIK:-aik}
SITE=${AI_SITE:-https://ai.tamnd.com}
LIVE="$SITE/dropout/"
if [ ! -r "$ENV_FILE" ]; then
  echo "DEPLOY FAILED: the environment file $ENV_FILE is missing" >&2
  exit 1
fi
cd "$PRTREE" || { echo "DEPLOY FAILED: no PR tree at $PRTREE; run pr.py once" >&2; exit 1; }
for try in 1 2 3 4 5 6; do
  until [ "$(df -Pm / | tail -1 | awk '{print $4}')" -gt 2000 ]; do sleep 30; done
  git checkout -q -- . && git clean -fdq && git fetch -q && git checkout -q --detach origin/main
  H=$(git rev-parse HEAD)
  # The subshell reads the credentials into its own environment; only the last lines of the aik output are kept.
  # shellcheck source=/dev/null
  out=$(set -a; . "$ENV_FILE"; set +a; "$AIK" site deploy -out build/site -commit "$H" 2>&1 | tail -2)
  rm -rf build
  sleep 20
  if curl -fsS "$LIVE" | grep -q "commit/$H"; then
    echo "deployed $H"
    exit 0
  fi
  echo "deploy try $try did not go live: $out"
  sleep 60
done
echo "DEPLOY FAILED"
exit 1
