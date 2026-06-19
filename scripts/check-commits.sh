#!/usr/bin/env bash

set -euo pipefail

base="${1:-}"
head="${2:-HEAD}"

if [[ -z "$base" ]]; then
  echo "usage: $0 <base-ref-or-sha> [head-ref-or-sha]" >&2
  exit 2
fi

failed=0
while IFS= read -r commit; do
  [[ -z "$commit" ]] && continue
  message_file="$(mktemp)"
  git show -s --format=%B "$commit" > "$message_file"
  if ! ./scripts/check-commit-message.sh "$message_file"; then
    echo "Commit: $commit" >&2
    failed=1
  fi
  rm -f "$message_file"
done < <(git rev-list --no-merges "${base}..${head}")

exit "$failed"
