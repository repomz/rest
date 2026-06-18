# Deployment

Этот репозиторий деплоится как CLI-бинарник `rest`. Описанные ниже workflows относятся именно к генератору `rest`.

Опции `features.ci` и `features.cd` в `rest_config/rest.yaml` имеют другое назначение: они генерируют GitHub Actions workflows внутри целевого REST-приложения.

## Установка пользователем

После публикации хотя бы одного semver-тега пользователь может установить CLI через Go:

```bash
go install github.com/repomz/rest/cmd/rest@latest
```

Или установить конкретную версию:

```bash
go install github.com/repomz/rest/cmd/rest@v0.1.0
```

Go устанавливает executable в `GOBIN`, а если `GOBIN` не задан, в `$(go env GOPATH)/bin`. Каталог установки должен находиться в `PATH`.

`go get` для установки CLI использовать не нужно: в современных версиях Go он предназначен для изменения зависимостей текущего модуля.

## Release flow

1. Подготовить изменения и проверить локально:

```bash
go test ./...
go build -trimpath -ldflags="-s -w -X main.version=v0.1.0" -o bin/rest ./cmd/rest
```

2. Создать semver-тег:

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. Workflow `Release` соберет бинарники для Linux, macOS и Windows, проставит версию в бинарник, посчитает checksums и опубликует GitHub Release.

## Self-update

После публикации нового GitHub Release установленный генератор можно обновить командой:

```bash
rest update
```

Команда проверяет последний release `github.com/repomz/rest`, скачивает подходящий архив для текущей ОС/архитектуры и заменяет текущий исполняемый файл `rest`.

Для установки конкретного релиза:

```bash
rest update --version v0.1.0
```

Если нужно переустановить ту же версию:

```bash
rest update --force
```

Проверить установленную версию:

```bash
rest version
```

## CI/CD

Workflow `CI` запускается на pull request и push в `main`/`master`:

- проверяет `gofmt`;
- запускает `go test ./...`;
- собирает CLI-бинарник.

Workflow `Release` запускается по тегам `v*` и публикует release artifacts, которые использует `rest update`.
