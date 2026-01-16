#!/usr/bin/env bash
set -euo pipefail
if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <ISO-8601-timestamp> <commit-message>" >&2
  exit 1
fi
export GIT_AUTHOR_DATE="$1"
export GIT_COMMITTER_DATE="$1"
shift
git add -A
git commit -m "$*"
