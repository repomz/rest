# Contributing to rest

This project uses Conventional Commits and generates `CHANGELOG.md`
automatically with `git-cliff`.

## One-time setup

Install `git-cliff`:

```bash
brew install git-cliff
```

Then enable the repository commit hook:

```bash
make setup
```

## Everyday workflow

Create a branch, change the code, run checks, and commit:

```bash
git switch -c feature/update-progress
make check
git add .
git commit -m "feat(update): show download progress"
git push -u origin feature/update-progress
```

Do not edit `CHANGELOG.md` manually. Preview the next release notes with:

```bash
make changelog
```

## Commit format

```text
<type>(<scope>): <description>
```

Examples:

```bash
feat(generator): add pagination templates
fix(update): handle releases without notes
docs(readme): explain SQLC setup
refactor(config): simplify validation
feat(config)!: rename authentication options
```

The `!` marks a breaking change. A detailed breaking change may also be placed
in the commit body:

```text
feat(config)!: rename authentication options

BREAKING CHANGE: authentication.jwt was renamed to authentication.tokens.
```

### Main types

| Type | Meaning | SemVer effect |
| --- | --- | --- |
| `feat` | New user-visible behavior | Minor |
| `fix` | Bug fix | Patch |
| `perf` | Performance improvement | Patch |
| `docs` | Documentation only | None |
| `refactor` | Internal change without new behavior | None |
| `test` | Tests only | None |
| `build` | Build or dependencies | None |
| `ci` | CI/CD configuration | None |
| `chore` | Repository maintenance | None |
| `style` | Formatting without behavior changes | None |
| `revert` | Revert an earlier commit | Depends on reverted change |

Any type with `!` or a `BREAKING CHANGE:` footer causes a major version bump.

## Project scopes

The scope tells readers where the change was made.

| Scope | Area |
| --- | --- |
| `cli` | CLI commands and argument parsing in `cmd/rest` |
| `update` | Self-update and release-note retrieval |
| `generator` | Core REST code generation |
| `appgen` | Application generation orchestration and feature registry |
| `config` | YAML configuration types, loading, validation, and defaults |
| `sqlc` | SQLC configuration and source integration |
| `templates` | Generated Go, Docker, CI, and project templates |
| `openapi` | OpenAPI document generation |
| `release` | Versioning, changelog generation, and publishing |
| `ci` | GitHub Actions and automated checks |
| `docs` | Project documentation |
| `tests` | Shared test infrastructure |
| `deps` | Go or automation dependencies |

Use the narrowest meaningful scope. If a change truly spans several areas,
omit the scope:

```text
refactor: reorganize generator packages
```

## Releasing

Prepare the next version automatically from commit history:

```bash
make release
```

`git-cliff` chooses the version:

- breaking change → major;
- `feat` → minor;
- `fix` or `perf` → patch.

If the repository has no release tags yet, `git-cliff` starts at `v0.1.0`.

To choose the version explicitly:

```bash
make release VERSION=v0.3.0
```

Review the generated commit and tag, then publish:

```bash
git show --stat
git show v0.3.0:CHANGELOG.md
make publish-release VERSION=v0.3.0
```

Pushing the tag starts GitHub Actions, which tests the project, builds release
archives, extracts that version from `CHANGELOG.md`, and creates GitHub Release.
