# REST Generator

`rest_generator` создает готовое REST-приложение на Go поверх кода, предварительно сгенерированного `sqlc`.

Генератор использует только:

- `sqlc.yaml`;
- SQL-схемы и SQL-запросы, указанные в `sqlc.yaml`;
- Go-файлы, созданные `sqlc`;
- имя Go-модуля из `go.mod`.

Дополнительные YAML-файлы для описания HTTP endpoint-ов не требуются.

## Общая схема

```text
SQL schema + SQL queries
          |
          v
     sqlc generate
          |
          v
DB-модели, параметры и методы sqlc
          |
          v
     rest generate
          |
          v
domain -> repository -> service -> HTTP -> main -> tests
```

## Требования

- Go 1.24 или новее;
- PostgreSQL;
- `sqlc`;
- корректный `go.mod`;
- `sqlc.yaml` с настроенной Go-генерацией;
- SQL-файлы схемы и запросов.

Пример минимальной конфигурации `sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/sql/queries"
    schema: "internal/sql/schema"
    gen:
      go:
        package: "db"
        out: "internal/app/db"
        emit_json_tags: true
        emit_prepared_queries: true
        emit_interface: true
```

## Быстрый старт

Соберите бинарник генератора:

```bash
make build-rest
```

Сначала запустите `sqlc`, затем REST generator:

```bash
sqlc generate
./bin/rest generate
```

Проверьте результат:

```bash
go test ./...
go build ./...
```

CLI поддерживает альтернативные пути:

```bash
rest generate -sqlc path/to/sqlc.yaml -out path/to/project
```

## Алгоритм генерации

### 1. Чтение конфигурации

Из `sqlc.yaml` извлекаются:

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

Также создаются или обновляются:

```text
Makefile
init_db.sh
```

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

Не следует вручную изменять файлы в этих каталогах: изменения будут потеряны при следующем запуске `rest generate`.

DB-слой `internal/app/db` управляется командой `sqlc generate`.

## Полный рабочий цикл

```bash
# 1. Изменить SQL-схему или запросы

# 2. Обновить DB-код
sqlc generate

# 3. Обновить REST-приложение
make rest-generate

# 4. Проверить результат
go test ./...
go build ./...

# 5. Запустить приложение
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
