# REST Generator

`rest_generator` сначала создает конфигурацию проекта, а затем генерирует REST-приложение на Go из выбранного стека и первичных файлов источников.

Основной файл `rest_config/rest.yaml` включает или отключает подсистемы. Настройки конкретных источников вынесены в `sqlc_rest.yaml`, `mongo_rest.yaml` и `auth_rest.yaml`. Команда `rest config generate` копирует канонические YAML целиком, включая разделители, пояснения и комментарии с допустимыми значениями.

В исходниках генератора файл `rest_config/templates.go` через `go:embed` упаковывает эти YAML внутрь устанавливаемого бинарника `rest`. Поэтому после `go install` или сборки бинарник самодостаточен: пользователю не нужны исходники генератора, а команда создаёт только редактируемые YAML-файлы.

Переключатели `sql`, `mongo` и `auth` задаются только в `rest.yaml`. Feature-файлы не дублируют состояние включения и содержат настройки, необходимые соответствующему генератору. Пока генерация MongoDB и auth не реализована, эти файлы являются проверяемыми контрактами будущих генераторов.

На текущем этапе полностью реализована SQL/HTTP-ветка на основе `sqlc`, включая настраиваемые HTTP middleware, logging на `zap`, OpenAPI, Docker, Prometheus-compatible metrics, тестовые артефакты и проектные файлы.

Отдельные YAML-файлы для ручного описания HTTP endpoint-ов не используются. Маршруты, параметры и результаты по-прежнему выводятся из SQL-схем, SQL-запросов и Go-файлов, созданных `sqlc`.

## Рабочие сценарии

```text
SQLC уже подготовлен:
rest config generate -> настройка rest_config -> rest app generate

Пустой проект:
rest config sqlc generate -> настройка файлов -> sqlc generate -f sqlc/sqlc.yaml -> rest app generate

Готовый пример без настроек:
rest config sqlc generate -> rest sqlc example generate -> sqlc generate -f sqlc/sqlc.yaml -> rest app generate
```

## Архитектура генератора

```text
cmd/rest                 разбор CLI-команд
internal/config          создание и чтение конфигурации
internal/appgen          оркестрация и реестр feature-генераторов
internal/generator       реализованный SQLC/HTTP-генератор
rest_config              YAML-шаблоны и templates.go для встраивания их в бинарник
```

Реестр использует общий интерфейс `Feature` с методами `Name`, `Enabled` и `Generate`. Текущая SQL/HTTP-ветка интегрирует logging, OpenAPI, Docker, curl, metrics и управляемые проектные файлы. Для MongoDB и auth подготовлены отдельные проверяемые конфигурационные контракты, но их генераторы пока не реализованы.

Каждая опциональная секция подчиняется одному правилу: `enabled: true` создаёт и интегрирует соответствующий код или файл, `enabled: false` не добавляет интеграцию и удаляет ранее созданный генератором артефакт. Исключение составляет пользовательское содержимое `.gitignore`: генератор управляет только блоком между собственными маркерами. Неизвестные поля YAML считаются ошибкой, поэтому опечатки в настройках не игнорируются.

При `sqlc_rest.yaml:init_migration: enable` из SQLC schema создаётся управляемая Goose-миграция `00001_rest_generator_init.sql`. При отключении удаляется только файл с маркером генератора; пользовательская миграция с таким именем не перезаписывается.

## Тестовые артефакты

Раздел `testing` в `rest.yaml` независимо управляет двумя видами файлов:

```yaml
testing:
  handler_tests: enabled
  curl: enabled
```

- `handler_tests` создает `*_handlers_test.go` для каждого инстанса;
- `curl` создает каталог `curl` и отдельный Markdown-файл для каждого инстанса, например `curl/study.md` и `curl/agent_record.md`;
- каждый curl-файл содержит команды для всех endpoint-ов своего инстанса;
- при отключении опции ранее сгенерированные файлы соответствующего типа удаляются при следующей генерации.

## OpenAPI

При `openapi.enabled: true` спецификация строится из той же внутренней модели, что handlers, HTTP DTO и маршруты приложения. Она включает:

- системный route `/`, все маршруты всех инстансов и Swagger routes при включенном UI;
- path/query-параметры с обязательностью и форматами `uuid`, `date`, `date-time`, integer и boolean;
- JSON request body для создания и дополнительных endpoint-ов;
- response schemas для объектов, массивов и scalar-результатов SQL-запросов;
- реальные служебные ответы `{ok: true}` и `{deleted: true}`;
- `400`, `404` и `500` только для тех обработчиков, которые могут вернуть соответствующий ответ;
- общую фактическую модель ошибки с полями `slug` и опциональным `error`;
- компоненты request/response для всех генерируемых HTTP-моделей.

Спецификация и встроенный Swagger endpoint используют один и тот же сгенерированный документ. При отключении OpenAPI ранее созданный файл спецификации удаляется.

## Требования

- Go 1.24 или новее;
- PostgreSQL;
- `sqlc`;
- корректный `go.mod`;
- `sqlc/sqlc.yaml` с настроенной Go-генерацией;
- SQL-файлы схемы и запросов.

Пример минимальной конфигурации `sqlc/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries"
    schema: "schema"
    gen:
      go:
        package: "db"
        out: "../internal/app/db"
        emit_json_tags: true
        emit_prepared_queries: true
        emit_interface: true
```

## Быстрый старт

Соберите бинарник генератора:

```bash
make build-rest
```

Если SQLC уже настроен и DB-код сгенерирован, создайте только конфиги REST Generator:

```bash
./bin/rest config generate
```

При необходимости настройте `rest_config/*.yaml`, включите `sqlc.enable` и выполните:

```bash
./bin/rest app generate
```

Для пустого проекта одной командой создайте конфиги REST Generator и SQLC-каркас:

```bash
./bin/rest config sqlc generate
```

Команда создаёт `rest_config/*.yaml`, единственный `sqlc/sqlc.yaml`, `sqlc/schema/item.sql` и `sqlc/queries/item.sql`. В bootstrap-конфиге `sqlc.enable` сразу включён.

Чтобы получить полностью рабочее приложение без ручной настройки, добавьте пример `study`:

```bash
./bin/rest sqlc example generate
```

Команда создаёт `sqlc_example/schema/studies.sql` и `sqlc_example/queries/studies.sql`, если в `sqlc_rest.yaml` установлено `sqlc.sqlc_example: enable`.

После этого выполните:

```bash
sqlc generate -f sqlc/sqlc.yaml
./bin/rest app generate
```

CLI поддерживает альтернативные каталоги:

```bash
rest config generate -out path/to/rest_config
rest config sqlc generate -out path/to/project
rest sqlc example generate -config path/to/rest_config -out path/to/project
rest app generate -config path/to/rest_config
```

## Алгоритм генерации

### 1. Чтение конфигурации

Из `sqlc/sqlc.yaml` извлекаются:

- каталог SQL-запросов;
- каталог SQL-схем;
- имя Go-пакета базы данных;
- каталог сгенерированного DB-кода.

Из `go.mod` читается имя модуля, которое используется для построения импортов.

### 2. Анализ SQL-схемы

Генератор находит выражения `CREATE TABLE` и для каждой таблицы определяет:

- имя таблицы;
- единственное и множественное Go-имя;
- базовый HTTP-путь;
- список колонок;
- типы полей;
- nullable и required поля;
- поля, доступные при создании записи.

Основные преобразования типов:

| PostgreSQL | Domain Go type | Nullable DB type |
| --- | --- | --- |
| `UUID` | `uuid.UUID` | `uuid.UUID` |
| `INTEGER` | `int32` | `sql.NullInt32` |
| `TIMESTAMP`, `DATE` | `time.Time` | `sql.NullTime` |
| `BOOLEAN` | `bool` | `bool` |
| `TEXT`, `VARCHAR` | `string` | `sql.NullString` |

Поля `id`, `created_at`, `updated_at`, `deleted` и поля с `DEFAULT` считаются управляемыми базой данных и не включаются в request создания.

### 3. Чтение результатов sqlc

Из файла `querier.go` читаются сигнатуры методов:

```go
GetItems(ctx context.Context, arg GetItemsParams) ([]Item, error)
```

Для каждого метода определяются:

- имя запроса;
- отсутствие аргумента, одиночный аргумент или структура параметров;
- тип структуры параметров;
- тип результата;
- возвращается одна запись, список, scalar value или только `error`.

Из файлов `*.sql.go` читаются структуры параметров, созданные `sqlc`.

### 4. Обязательные и необязательные параметры

Обычные параметры SQL считаются обязательными.

Параметры, объявленные через `sqlc.narg(...)`, считаются необязательными HTTP query-параметрами.

Пример универсальной фильтрации:

```sql
-- name: GetItems :many
SELECT * FROM items
WHERE deleted = false
  AND (sqlc.narg('date')::timestamp IS NULL
       OR created_at::date = sqlc.narg('date')::date)
  AND (sqlc.narg('type')::text IS NULL
       OR item_type = sqlc.narg('type'))
  AND (sqlc.narg('owner')::text IS NULL
       OR owner = sqlc.narg('owner'));
```

Такой запрос создает один endpoint коллекции:

```text
GET /items
GET /items?date=2026-06-14
GET /items?type=primary
GET /items?owner=user-1
GET /items?date=2026-06-14&type=primary&owner=user-1
```

Без query-параметров возвращается вся коллекция. Фильтры могут использоваться отдельно и в любых комбинациях.

## Правила HTTP endpoint-ов

HTTP-метод выводится из имени SQL-запроса:

| Префикс запроса | HTTP method |
| --- | --- |
| `Get`, `List`, `Find`, `Search` | `GET` |
| `Delete`, `Remove`, `SoftDelete` | `DELETE` |
| `Update`, `Patch` | `PATCH` |
| остальные | `POST` |

### Источники параметров

- необязательные параметры `GET` и `DELETE` передаются через query string;
- обязательные параметры `GET` и `DELETE` передаются через path;
- параметры `POST`, `PUT` и `PATCH` передаются через JSON body;
- UUID-параметр `id` для `PATCH` и `PUT` передается через path.

Примеры:

```sql
-- name: GetItemByID :one
SELECT * FROM items WHERE id = $1;

-- name: UpdateItemStatus :one
UPDATE items SET status = $2 WHERE id = $1 RETURNING *;

-- name: DeleteItemsByOwnerID :exec
DELETE FROM items WHERE owner_id = $1;
```

Ожидаемые маршруты:

```text
GET    /items/{id}
PATCH  /items/{id}/status
DELETE /items/owner/{owner_id}
```

## Стандартные операции

Генератор отдельно распознает следующие соглашения:

```text
Create<Item>
Get<Items>
Get<Item>ByID
SoftDelete<Item>
SoftDeleteAll<Items>
```

Обычный `Get<Items>` без аргументов становится стандартным чтением коллекции.

Если `Get<Items>` принимает структуру с `sqlc.narg(...)`, он становится параметризованным `GET /items` с фильтрами.

Остальные методы `sqlc` автоматически превращаются в дополнительные endpoint-ы.

## Генерируемые слои

`sqlc` создает DB-слой:

```text
internal/app/db
```

REST generator создает:

```text
internal/app/common
internal/app/config
internal/app/domain
internal/app/repository/pgrepo
internal/app/services
internal/app/transport/httpmodels
internal/app/transport/httpserver
cmd/main.go
```

В зависимости от переключателей также создаются:

```text
Makefile
init_db.sh
.gitignore
.env.example
.env                    только при generate_local_env: true, права 0600
Dockerfile
.dockerignore
internal/sql/migrations/00001_rest_generator_init.sql
internal/app/metrics
curl/*.md
```

`Makefile`, `.env.example`, `.env` и `init_db.sh` получают параметры БД из `sqlc_rest.yaml`. `init_db.sh` создаётся с правами `0755`; `.gitignore` сохраняет пользовательские строки вне блока `rest_generator`; цель `make rest-generate` использует путь к исходному каталогу конфигурации.

### Domain

Содержит:

- доменную модель;
- модель создания записи;
- структуры параметров endpoint-ов;
- required-валидацию;
- преобразование DB-модели в domain-модель.

### Repository

Содержит:

- вызовы методов `sqlc`;
- преобразование domain-параметров в `db.*Params`;
- преобразование DB-моделей в domain-модели;
- обработку `sql.ErrNoRows`;
- добавление контекста к ошибкам.

### Service

Определяет интерфейс repository и делегирует ему операции. Этот слой является точкой расширения для бизнес-логики.

### HTTP models

Содержит JSON request/response DTO и преобразование request в domain-модель.

### HTTP server

Содержит:

- интерфейсы сервисов;
- HTTP handlers;
- чтение path/query/body параметров;
- parsing UUID, integer и date;
- преобразование результата в JSON;
- обработку ошибок;
- сгенерированные handler-тесты.

### Application entrypoint

`cmd/main.go`:

- читает `HTTP_ADDR` и `DB_DSN`;
- подключается к PostgreSQL;
- создает repository, service и HTTP server;
- регистрирует маршруты в Gorilla Mux;
- запускает HTTP-сервер;
- выполняет graceful shutdown.

## Выполнение запроса

Типичный HTTP-запрос проходит цепочку:

```text
Gorilla Mux
    -> HTTP handler
    -> parsing и validation параметров
    -> domain params
    -> service
    -> repository
    -> sqlc method
    -> PostgreSQL
    -> DB model
    -> domain model
    -> HTTP response model
    -> JSON response
```

## Ошибки

Генерируемый HTTP-слой возвращает ошибки в формате:

```json
{
  "slug": "invalid-id",
  "error": "invalid UUID length"
}
```

Поле `error` добавляется при установленной переменной окружения `DEBUG_ERRORS`.

Поддерживаются основные категории:

- `400 Bad Request` для некорректных параметров и JSON;
- `404 Not Found` для отсутствующей записи;
- `500 Internal Server Error` для остальных ошибок.

## Важное ограничение

Перед каждой генерацией каталоги прикладных слоев удаляются и создаются заново:

```text
internal/app/common
internal/app/config
internal/app/domain
internal/app/repository
internal/app/services
internal/app/transport
```

Не следует вручную изменять файлы в этих каталогах: изменения будут потеряны при следующем запуске `rest app generate`.

DB-слой `internal/app/db` управляется командой `sqlc generate -f sqlc/sqlc.yaml`.

## Полный рабочий цикл

```bash
# 1. Создать конфиги генератора и SQLC-каркас
rest config sqlc generate

# 2. Создать готовый пример study
rest sqlc example generate

# 3. Сгенерировать DB-код
sqlc generate -f sqlc/sqlc.yaml

# 4. Сгенерировать REST-приложение
rest app generate

# 5. Проверить результат
go test ./...
go build ./...

# 6. Запустить приложение
DB_DSN="postgres://user:password@localhost:5432/app_db?sslmode=disable" \
HTTP_ADDR=:8080 \
DEBUG_ERRORS=1 \
make run
```

## Тестирование генератора

Тесты в `internal/generator` проверяют ключевые регрессии:

- стандартные CRUD-запросы не дублируются;
- запросы `:exec` не теряются;
- `sqlc.narg(...)` распознается как необязательный параметр;
- фильтры генерируются как query-параметры коллекции;
- обычные SQL-параметры остаются обязательными.

Запуск:

```bash
go test ./...
```
