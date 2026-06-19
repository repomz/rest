#!/usr/bin/env bash

set -euo pipefail

message_file="${1:-}"
if [[ -z "$message_file" || ! -f "$message_file" ]]; then
  echo "usage: $0 <commit-message-file>" >&2
  exit 2
fi

subject="$(head -n 1 "$message_file")"

if [[ "$subject" =~ ^Merge\  ]] || [[ "$subject" =~ ^Revert\ \" ]]; then
  exit 0
fi

pattern='^(feat|fix|docs|test|refactor|perf|build|ci|chore|style|revert)(\([a-z0-9][a-z0-9._/-]*\))?!?: .+'
if [[ "$subject" =~ $pattern ]]; then
  exit 0
fi

cat >&2 <<EOF
Invalid Conventional Commit:

  $subject

Expected:

  <type>(<scope>): <description>

Examples:

  feat(update): show release notes
  fix(generator): preserve handler parameters
  docs(readme): document installation
  feat(config)!: remove deprecated option

Allowed types:
  feat fix docs test refactor perf build ci chore style revert
EOF
exit 1
