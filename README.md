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
./bin/rest init --sqlc --example
sqlc generate -f sqlc/sqlc.yaml
./bin/rest generate
go test ./...
```

Если SQLC уже настроен:

```bash
./bin/rest init
sqlc generate -f sqlc/sqlc.yaml
./bin/rest generate
```

## Команды

| Команда | Что делает |
| --- | --- |
| `rest init` | Создает `rest_config/*.yaml` |
| `rest init --sqlc` | Добавляет минимальный SQLC-каркас |
| `rest init --sqlc --example` | Добавляет SQLC-каркас и рабочий пример |
| `rest generate` | Генерирует REST-приложение |
| `rest generate -config path` | Использует другой каталог конфигурации |
| `rest update` | Обновляет бинарник `rest` из GitHub Releases |
| `rest version` | Показывает текущую версию |

## Что Генерируется

```text
cmd/main.go
internal/app/domain
internal/app/repository/pgrepo
internal/app/services
internal/app/transport/httpmodels
internal/app/transport/httpserver
```

Опционально генерируются `Dockerfile`, `.env.example`, `Makefile`, `docs/swagger.yaml`, `curl/*.md`, metrics, logging и Goose init migration.

Файлы в `internal/app/*` пересоздаются при каждом `rest generate`. Пользовательскую бизнес-логику не стоит писать прямо в этих файлах.

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

Готово: SQLC/PostgreSQL generation, OpenAPI, Docker, zap logging, metrics, handler tests, curl docs, self-update.

Пока не готово: MongoDB generator, auth generator, plugin system, safe upgrade tooling для уже сгенерированных проектов.

## License

Проект распространяется по лицензии [Apache-2.0](LICENSE).
