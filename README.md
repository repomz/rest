# REST

<p align="center"><strong>Write queries. Get an application. Add business logic.</strong></p>

[![CI](https://github.com/repomz/rest/actions/workflows/ci.yml/badge.svg)](https://github.com/repomz/rest/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![sqlc](https://img.shields.io/badge/sqlc-supported-5C6BC0)](https://sqlc.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

`rest` is a Go CLI for generating REST applications on top of SQLC/PostgreSQL and MongoDB contracts.

For SQL projects, it reads SQL schemas, SQLC queries, and generated Go code, then creates a layered application with domain models, repositories, services, HTTP transport, OpenAPI, Docker, logging, metrics, tests, and curl examples. For MongoDB projects, it reads `rest_mongo/*.yaml` contracts and generates a layered MongoDB HTTP API with custom methods, OpenAPI documentation, auth middleware, and Docker output.

## Installation

Go 1.24 or newer is required:

```bash
go install github.com/repomz/rest/cmd/rest@latest
```

The binary is installed into `$(go env GOPATH)/bin`. Make sure this directory is included in your `PATH`.

Verify the installation:

```bash
rest version
```

Install a specific release:

```bash
go install github.com/repomz/rest/cmd/rest@v0.1.0
```

Update an existing installation:

```bash
rest update
```

After installing the release, the command prints its GitHub Release notes,
including breaking changes, features, fixes, and documentation updates.
Release entries in [`CHANGELOG.md`](CHANGELOG.md) are generated automatically
from Conventional Commits.

## Quick Start

Generate a standalone SQL example project:

```bash
rest init --example sql
rest gen
go test ./...
```

Generate a standalone MongoDB example project:

```bash
rest init --example mongo
rest gen
go test ./...
```

Use an existing SQLC project:

```bash
rest init
# Set enable: enable and a valid sqlc_path in rest_config/rest_sqlc.yaml.
rest gen
```

## Commands

| Command | Description |
| --- | --- |
| `rest init` | Create `rest_config/*.yaml` and a customizable `rest_sqlc/` project skeleton |
| `rest init --example sql` | Create a standalone `rest_sqlc_example/` project |
| `rest init --example mongo` | Create a standalone MongoDB example contract |
| `rest gen` | Generate the REST application |
| `rest doctor` | Validate configs, generated files, tooling, Docker/OpenAPI/auth readiness |
| `rest update` | Update the CLI from GitHub Releases |
| `rest changelog` | Print the latest GitHub Release notes |
| `rest changelog --version vX.Y.Z` | Print notes for a specific release |
| `rest version` | Print the installed version |

For SQL projects, `rest gen` runs:

```bash
sqlc generate -f <sqlc_path>
go mod tidy
```

Install SQLC with:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

To run SQLC manually, set `auto_sqlc: disable` in `rest_config/rest.yaml`, execute `sqlc generate -f <sqlc_path>`, and then run `rest gen`.

For MongoDB projects, `rest gen` reads active `rest_config/rest_mongo/*.yaml` contracts, ignoring system files that start with `rest_`, and generates a layered MongoDB API: domain documents, Mongo repositories, services, HTTP handlers, custom method routes, OpenAPI, and optional Docker/Docker Compose output.

Use `rest doctor` before or after `rest gen` to catch common setup issues: invalid YAML, missing enabled configs, broken SQLC/Mongo contract paths, auth policy conflicts, Docker/OpenAPI output gaps, and generated-project readiness.

### Authentication and authorization

Set `auth: enable` in `rest_config/rest.yaml`. The first `rest gen` generates the application and creates `rest_config/auth_rest.yaml` with every discovered SQL and Mongo endpoint. Choose JWT authentication or Basic Auth, configure `public`, `require_auth`, and `roles`, then run `rest gen` again to generate authentication and RBAC route guards. For SQL JWT projects, generated auth handlers issue HS256 tokens. For Mongo projects, generated middleware validates JWT bearer tokens or Basic Auth credentials and applies endpoint role policies. New endpoints are merged into the auth file without losing existing policies.

When REST, SQLC, Mongo, schema, query, and auth configuration inputs have not changed, `rest gen` exits without regenerating code.

## Generated Application

```text
cmd/main.go
internal/app/domain
internal/app/repository/pgrepo
internal/app/services
internal/app/transport/httpmodels
internal/app/transport/httpserver
```

MongoDB projects generate the same application layout with generic BSON document domain types, collection-specific repositories, services, HTTP handlers, custom method handlers, swagger routes, and optional auth middleware.

Optional output includes `Dockerfile`, `docker-compose.yml`, `.env.example`, `Makefile`, `docs/swagger.yaml`, GitHub Actions workflows, curl examples, logging, metrics, and a Goose initialization migration where supported by the selected backend.

Files under `internal/app/*` are regenerated by `rest gen`. With `safe_reload` enabled, REST detects user changes and asks whether each modified file should be kept or overwritten.

## Development

```bash
make setup
gofmt -w .
make check
make generated-examples
REST_RUNTIME_E2E=1 make runtime-e2e # requires live Postgres, MongoDB, and sqlc
```

Use Conventional Commit messages such as
`feat(update): show download progress`. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the project workflow and
[`RELEASE_WORKFLOW.md`](RELEASE_WORKFLOW.md) for the complete reusable setup.

Run the representative 10/50-table generator benchmark:

```bash
make benchmark
```

Pull requests should stay focused, include tests for new generator behavior, and update documentation when CLI, configuration, or generated output changes.

## Status

Available: SQLC/PostgreSQL generation, MongoDB example generation, layered MongoDB CRUD/custom-method generation, OpenAPI, Docker/Docker Compose for SQL and Mongo projects, SQL JWT auth handlers, Mongo JWT/Basic Auth middleware, zap logging, metrics, handler tests, curl documentation, graceful shutdown, CI/CD workflow templates, safe reload, `rest doctor`, and self-update.

Planned: plugin support, dry-run/diff commands, generated manifests, and migration tooling for existing generated projects.

## License

Licensed under the [Apache License 2.0](LICENSE).
