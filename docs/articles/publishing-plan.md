# План публикации Query-First

## Рекомендуемая последовательность

1. Довести публичный README и quick start до состояния, в котором команды из статьи выполняются без дополнительных догадок.
2. Создать стабильный tagged release и приложить бинарники/checksums.
3. Опубликовать русскую статью на Habr.
4. Собрать вопросы и возражения читателей в течение нескольких дней.
5. Обновить английскую версию с учётом этой обратной связи.
6. Опубликовать английскую статью на DEV Community.
7. При желании кросспостить её в Hashnode или Medium, указав canonical URL основной английской публикации.

## Почему основной английской площадкой выбран DEV Community

- аудитория сфокусирована на разработчиках;
- формат хорошо подходит для open-source launch и технических walkthrough;
- удобно использовать Markdown, code blocks и теги;
- статья может вести обсуждение вокруг архитектурной идеи, а не только вокруг продукта;
- готовую версию легко адаптировать для других площадок.

## Заголовки для A/B выбора

### Habr

- Query-First: как получить REST-приложение из схемы PostgreSQL и SQL-запросов
- Написал SQL-запросы — получил REST API: знакомство с Query-First
- Между SQLC и REST: генератор backend-приложения из реальных запросов

Рекомендуемый: первый. Он ясный, хорошо ищется и сразу объясняет концепцию.

### English

- Query-First: Generate a REST Application from PostgreSQL Schema and SQL Queries
- Write Queries, Get an Application: A Query-First Approach to REST
- What If SQL Queries Were the Starting Point of Your REST API?

Рекомендуемый для DEV: второй, если нужен более эмоциональный launch; первый — если приоритет у поиска и технической ясности.

## Визуальные материалы

- `assets/query-first-cover-v2.png` — основная нейтральная обложка без текста и брендов;
- `assets/query-first-cover.png` — первый вариант обложки с узнаваемыми технологическими символами; можно не использовать;
- `assets/query-first-flow.svg` — схема концепции;
- после публичного release стоит добавить GIF или короткое видео терминала:
  - `rest init --example`;
  - `rest generate`;
  - список routes;
  - Swagger UI.

Не стоит перегружать статью скриншотами generated-кода: один хороший SQL→HTTP пример объясняет идею лучше десяти изображений редактора.

## Что проверить перед публикацией

- команды quick start выполняются на чистом окружении;
- URL репозитория публично доступен;
- README содержит installation section;
- есть понятный license;
- release не называется `dev`;
- статья не заявляет auth, MongoDB, rate limiting или transactions как реализованные функции;
- issue tracker готов принять обратную связь;
- закреплён issue/discussion с вопросами по Query-First;
- generated example компилируется и проходит тесты.

## Рекомендуемые теги

### Habr

Go, PostgreSQL, RESTful API, SQL, Open source.

### DEV Community

`go`, `postgres`, `backend`, `opensource`.

DEV разрешает не более четырёх тегов, поэтому `sqlc` лучше оставить в тексте и заголовках разделов.

## Первый комментарий автора

Полезно сразу оставить комментарий:

> Особенно интересна критика самой границы Query-First: что можно надёжно вывести из схемы и именованных запросов, а что обязательно должно оставаться явной конфигурацией или custom-кодом. Если у вас есть неудобный SQLC-запрос из реального проекта — приносите его в issues.

Так обсуждение с большей вероятностью уйдёт в инженерную сторону, а не в вечный спор «генерация кода — добро или зло».

## Материалы для следующих статей

После реализации каждая тема заслуживает отдельной публикации:

1. Auth-first extension: authentication, authorization, endpoint visibility and OpenAPI security.
2. Query-First for MongoDB: named operations and aggregation pipelines.
3. Transactional workflows: composing several queries into one generated business operation.
4. Explicit REST annotations: overriding inferred method, path, status and access policy.
5. Safe regeneration: how to evolve generated applications without destroying custom code.
