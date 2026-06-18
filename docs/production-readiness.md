# Production Readiness

Это честная оценка того, что в `rest` уже выглядит крепко, а что стоит реализовать до уровня зрелого production-ready open-source проекта.

## Что Уже Хорошо

| Область | Статус |
| --- | --- |
| CLI basics | Есть команды init, generate, update, version |
| Config loading | Строгий YAML parsing с запретом unknown fields |
| SQLC/PostgreSQL path | Реализован end-to-end |
| Generated app structure | Слои domain/repository/service/transport |
| OpenAPI | Генерируется из той же endpoint-модели, что и handlers |
| Docker | Генерируется multi-stage Dockerfile |
| Observability | zap logging и optional Prometheus-compatible metrics |
| Runtime | Переиспользование соединений через `database/sql` и graceful shutdown |
| Orchestration | Optional auto SQLC перед генерацией и обязательный `go mod tidy` после нее |
| Safety | `rest init` не перезаписывает config-файлы, `safe_reload` защищает ручные изменения generated files |
| CI/CD | GitHub Actions для test/build и release artifacts |
| Self-update | Обновление binary из GitHub Releases |
| License | Apache-2.0 добавлена |

## Главные Пробелы До Зрелого Production Уровня

### 1. Project Governance

Лицензия уже добавлена: Apache-2.0. Для зрелого публичного проекта остается оформить governance и правила сопровождения.

Рекомендовано:

- добавить `CODE_OF_CONDUCT.md`, если проект будет публично принимать community contributions;
- добавить issue и PR templates;
- описать support policy и versioning policy.

### 2. Проверка Release Trust Для Self-Update

`rest update` скачивает release asset и заменяет binary. Перед mature production-использованием нужно проверять целостность.

Рекомендовано:

- проверять `checksums.txt`;
- подписывать releases через cosign, minisign или GPG;
- научить updater проверять signature или минимум SHA-256;
- документировать поведение при permission errors;
- добавить check-only режим:

```bash
rest update --check
```

### 3. Semantic Versioning И Changelog

Release workflow публикует artifacts, но пользователю нужен upgrade context.

Рекомендовано:

- добавить `CHANGELOG.md`;
- явно соблюдать SemVer;
- документировать breaking changes в config и generated output;
- генерировать release notes из PR labels или conventional commits.

### 4. Compatibility Для Generated Projects

Самая сложная production-задача для code generator: безопасное обновление уже сгенерированных проектов.

Рекомендовано:

- добавить `rest doctor` для проверки состояния проекта;
- добавить `rest diff`, чтобы показать будущие изменения генерации;
- добавить `rest gen --dry-run`;
- проверять совместимость config version и CLI version;
- добавить manifest generated files;
- писать migration guides между версиями генератора.

### 5. Template Stability

Templates: нормальная основа для генератора, но изменения templates должны иметь сильные гарантии.

Рекомендовано:

- добавить golden tests для representative generated projects;
- компилировать generated app в CI;
- запускать `go test` внутри generated examples;
- стабилизировать fixtures для OpenAPI output;
- покрыть все поддерживаемые SQL type mappings.

### 6. Более Широкая SQL Поддержка

Текущий parser покрывает реализованный путь, но production SQL бывает сложнее.

Рекомендовано:

- расширить PostgreSQL type mapping;
- лучше обрабатывать complex defaults;
- улучшить schema parsing для constraints и indexes;
- протестировать multiple schemas;
- явно документировать unsupported SQL patterns.

### 7. Auth И MongoDB

Config contracts уже есть, но генерация пока намеренно не реализована.

Рекомендовано:

- реализовать auth/MongoDB generators;
- либо явно пометить их как future/experimental во всех публичных документах;
- не создавать впечатление production-поддержки до реализации.

### 8. Security Defaults

Сгенерированные приложения должны быть консервативными по умолчанию.

Рекомендовано:

- пересмотреть CORS defaults для production;
- добавить tests для request size limits;
- документировать secret handling;
- генерировать `.env` только при явном запросе;
- добавить security headers middleware option;
- добавить rate limit middleware option.

### 9. Installation Story

Сейчас есть локальная сборка и release binaries. Для зрелого CLI нужны дополнительные installation paths.

Рекомендовано:

- опубликовать Homebrew tap;
- документировать `go install`, когда version package strategy будет стабильной;
- добавлять install script только после release verification;
- описать checksums/signature verification.

### 10. Examples И Troubleshooting

Документация теперь описывает структуру, но adoption сильно выигрывает от полных примеров.

Рекомендовано:

- добавить complete generated example project в `examples/`;
- добавить пример rendered OpenAPI;
- добавить "from SQL to endpoint" examples;
- добавить troubleshooting docs.

## Suggested Roadmap

### Short Term

- добавить `CHANGELOG.md`;
- добавить checksum verification в `rest update`;
- добавить `rest update --check`;
- добавить generated-example CI job;
- добавить issue/PR templates.

### Medium Term

- добавить `rest doctor`;
- добавить `rest gen --dry-run`;
- добавить generated file manifest;
- расширить SQL type support;
- добавить golden tests для representative projects.

### Long Term

- реализовать auth generator;
- реализовать MongoDB generator или убрать его из public-facing default config до готовности;
- добавить plugin/template extension system;
- добавить package manager installers;
- подписывать releases и проверять signatures в updater.

## Практическое Определение Production Ready

Для этого проекта production ready означает:

- releases воспроизводимы и проверяемы;
- generated code компилируется и тестируется в CI на representative projects;
- upgrades предсказуемы и inspectable;
- destructive file operations жестко ограничены и покрыты тестами;
- unsupported SQL/config patterns падают с понятными ошибками;
- docs четко отделяют реализованные features от future contracts.
