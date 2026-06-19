#!/usr/bin/env bash

set -euo pipefail

requested_version="${1:-}"

if ! command -v git-cliff >/dev/null 2>&1; then
  echo "git-cliff is required. Install it with: brew install git-cliff" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "The working tree must be clean before preparing a release." >&2
  exit 1
fi

branch="$(git branch --show-current)"
if [[ "$branch" != "main" && "$branch" != "master" ]]; then
  echo "Releases must be prepared from main or master; current branch: $branch" >&2
  exit 1
fi

git fetch origin "$branch" --tags

if [[ "$(git rev-parse HEAD)" != "$(git rev-parse "origin/$branch")" ]]; then
  echo "Local $branch must exactly match origin/$branch before a release." >&2
  echo "Pull or push your commits, then run the release again." >&2
  exit 1
fi

latest_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
if [[ -n "$latest_tag" ]]; then
  ./scripts/check-commits.sh "$latest_tag" HEAD
fi

if [[ -n "$requested_version" ]]; then
  version="$requested_version"
else
  version="$(git-cliff --bumped-version)"
fi

if [[ "$version" != v* ]]; then
  version="v${version}"
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must match vMAJOR.MINOR.PATCH; got: $version" >&2
  exit 1
fi

if git rev-parse "$version" >/dev/null 2>&1; then
  echo "Tag already exists: $version" >&2
  exit 1
fi

echo "Preparing $version"
make check
git-cliff --tag "$version" --output CHANGELOG.md

if ! grep -Fq "## [$version]" CHANGELOG.md; then
  echo "Generated CHANGELOG.md does not contain ## [$version]" >&2
  exit 1
fi

git add CHANGELOG.md
git commit -m "chore(release): prepare $version"
git tag -a "$version" -m "$version"

cat <<EOF

Release $version is prepared locally.

Review it:
  git show --stat
  git show $version:CHANGELOG.md

Publish it:
  make publish-release VERSION=$version
EOF
