# REST Generator

[![CI](https://github.com/repomz/rest_generator/actions/workflows/ci.yml/badge.svg)](https://github.com/repomz/rest_generator/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![sqlc](https://img.shields.io/badge/sqlc-supported-5C6BC0)](https://sqlc.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

`rest_generator`: CLI на Go для генерации REST-приложения поверх SQLC/PostgreSQL.

Генератор читает SQL-схемы, SQLC-запросы и Go-код, созданный `sqlc`, а затем собирает приложение со слоями `domain`, `repository`, `service`, `transport`, OpenAPI, Dockerfile, logging, metrics, тестами и curl-примерами.

## Быстрый Старт

```bash
make build-rest
./bin/rest init --example
./bin/rest generate
go test ./...
```

Если SQLC уже настроен:

```bash
./bin/rest init
# Укажите enable: enable и корректный sqlc_path в rest_config/sqlc_rest.yaml.
./bin/rest generate
```

## Команды

| Команда | Что делает |
| --- | --- |
| `rest init` | Создает `rest_config/*.yaml` |
| `rest init --sqlc` | Создает пользовательский `sqlc/`-каркас и удаляет `sqlc_example/` |
| `rest init --example` | Создает автономный `sqlc_example/` с примером `study` |
| `rest init --out path` | Создает конфигурацию и выбранный SQLC-режим в другом каталоге |
| `rest generate` | Генерирует REST-приложение |
| `rest generate -config path` | Использует другой каталог конфигурации |
| `rest update` | Обновляет бинарник `rest` из GitHub Releases |
| `rest version` | Показывает текущую версию |

`rest generate` по умолчанию выполняет под капотом:

```bash
sqlc generate -f <sqlc_path>
go mod tidy
```

Для этого должен быть установлен `sqlc`:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Если нужно запускать SQLC вручную, установите `auto_sqlc: disable` в `rest_config/rest.yaml` и перед `rest generate` выполните `sqlc generate -f <sqlc_path>` самостоятельно. Значение `<sqlc_path>` берется из `rest_config/sqlc_rest.yaml`.

## Что Генерируется

```text
cmd/main.go
internal/app/domain
internal/app/repository/pgrepo
internal/app/services
internal/app/transport/httpmodels
internal/app/transport/httpserver
```

Опционально генерируются `Dockerfile`, `.env.example`, `Makefile`, `docs/swagger.yaml`, `.github/workflows/*.yaml`, `curl/*.md`, metrics, logging и Goose init migration.

Файлы в `internal/app/*` пересоздаются при каждом `rest generate`. При включенном `safe_reload` генератор сравнивает их с последним снимком и по каждому измененному файлу предлагает сохранить пользовательскую версию либо перезаписать ее.

## Документация

- [Архитектура проекта](docs/architecture.md)
- [Деплой и self-update](docs/deployment.md)
- [Production readiness](docs/production-readiness.md)

## Разработка

```bash
gofmt -w .
go test ./...
go build -trimpath -ldflags="-s -w" -o bin/rest ./cmd/rest
```

Коротко для PR:

- держите изменения маленькими и сфокусированными;
- добавляйте тесты на новое поведение генератора;
- обновляйте docs, если меняются CLI, config или generated output.

## Статус

Готово: SQLC/PostgreSQL generation, OpenAPI, Docker, zap logging, metrics, handler tests, curl docs, graceful shutdown, CI/CD workflow templates, safe reload, self-update.

Пока не готово: MongoDB generator, auth generator, plugin system, dry-run/doctor и полноценные migration-инструменты для уже сгенерированных проектов.

## License

Проект распространяется по лицензии [Apache-2.0](LICENSE).
