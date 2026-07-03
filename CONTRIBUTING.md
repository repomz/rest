# Contributing to rest

Thanks for considering a contribution. `rest` is a Go CLI that generates layered REST applications from SQLC/PostgreSQL files or MongoDB contracts, so most changes affect either the CLI, configuration, generator logic, generated templates, or tests.

## Local setup

Install Go 1.25.11 or newer, then prepare the repository:

```bash
make setup
make check
```

For SQL-related generator work, install SQLC:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0
```

Some optional checks require extra tools:

- Docker, for generated Dockerfile smoke checks.
- PostgreSQL and MongoDB, for live runtime e2e checks.
- `govulncheck`, for vulnerability checks.

## Workflow

Create a focused branch, make the change, run the relevant checks, and open a pull request:

```bash
git switch -c feat/my-change
make check
git add .
git commit -m "feat(generator): describe the change"
git push -u origin feat/my-change
```

Run broader checks when the change affects generated output, templates, auth, Docker, OpenAPI, or runtime behavior:

```bash
make golden
make race
make vuln
make generated-examples
```

Docker and live database checks are useful before larger pull requests:

```bash
REST_DOCKER_SMOKE=1 make docker-smoke
REST_RUNTIME_E2E=1 make runtime-e2e
```

## Commit format

Use Conventional Commits:

```text
<type>(<scope>): <description>
```

Examples:

```bash
feat(generator): add pagination templates
fix(cli): improve YAML validation errors
docs(readme): clarify SQLC setup
test(auth): cover missing roles
refactor(config): simplify defaults
```

Common types:

- `feat` — new user-visible behavior.
- `fix` — bug fix.
- `docs` — documentation only.
- `test` — tests only.
- `refactor` — internal change without new behavior.
- `build` — build or dependency changes.
- `ci` — GitHub Actions or CI changes.
- `chore` — repository maintenance.

Use a scope when it makes the affected area clear, for example `cli`, `config`, `generator`, `templates`, `auth`, `openapi`, `mongo`, `sqlc`, `docs`, or `ci`.

Mark breaking changes with `!`:

```text
feat(config)!: rename authentication options
```

## Pull requests

Good pull requests are small, focused, and easy to review.

Please include:

- a short explanation of the user-facing change;
- tests for new generator behavior or bug fixes;
- updated golden snapshots when generated output changes;
- documentation updates when commands, configuration, or generated behavior change.

Avoid mixing unrelated refactors with feature work. If a change touches generated code, run `make golden` and review the diff carefully before opening the pull request.
