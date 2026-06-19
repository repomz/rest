#!/usr/bin/env bash

set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: make publish-release VERSION=vMAJOR.MINOR.PATCH" >&2
  exit 2
fi

if ! git rev-parse "$version" >/dev/null 2>&1; then
  echo "Local tag does not exist: $version" >&2
  exit 1
fi

branch="$(git branch --show-current)"
if [[ "$branch" != "main" && "$branch" != "master" ]]; then
  echo "Release publishing is only allowed from main or master." >&2
  exit 1
fi

git push origin "$branch"
git push origin "$version"

echo "Published $version. GitHub Actions will build and publish the release."
