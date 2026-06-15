# StatusGuard

StatusGuard - это backend-сервис для мониторинга доступности HTTP-сервисов.

Проект позволяет добавлять целевые сервисы, переодически проверять их доступность, сохранять историю проверок и фиксировать инциденты, когда сервис перестает отвечать ожидаемым образом.

> Проект находится в стадии разработки. Текущая версия уже содержит минимальную рабочую основу: CRUD для мониторинга целей, ручные и фоновые проверки, историю проверок и базовую работу с инцидентами.

## Возможности

- Добавление HTTP-сервисов для мониторинга.
- Получение списка сервисов и информации по конкретному сервису.
- Обновление параметров мониторинга.
- Удаление сервиса из мониторинга.
- Ручная проверка сервиса по запросу.
- Автоматическая фоновая проверка активных сервисов через scheduler.
- Сохранение истории проверок в PostgreSQL.
- Создание инцидента при падении сервиса.
- Увеличение счётчика неуспешных проверок для уже открытого инцидента.
- Автоматическое закрытие инцидента при восстановлении сервиса.
- Health-check endpoint для проверки состояния приложения.
- Запуск через docker-compose.
- Структурированное логирование через zap.

## Стек

- Go
- PostgreSQL
- Docker / Docker compsoe
- gorrila/mux
- golang-migrate
- zap logger

## Архитектура проекта

Проект основан на слоистой архитектуре:

```text
cmd/app точка входа
internal/config загрузка конфигурации из .env
internal/logger настройка логгера
internal/transport HTTP-хендлеры и DTO
internal/monitor логика работы с целями мониторинга
internal/checker    выполнение HTTP-проверок и сохранение результатов
internal/scheduler  фоновый запуск проверок
internal/incident   обработка инцидентов
internal/notification   уведомления
migrations  SQL-миграции базы данных
```

Основной поток работы выглядит так:

```text
HTTP request
    -> transport handler
    -> service
    -> repository
    -> PostgreSQL
```

Для фонового мониторинга используется отдельный scheduler:

```text
scheduler
    -> получает активные targets
    -> запускает проверки через worker pool
    -> сохраняет check_result
    -> передаёт результат в incident service
    -> открывает или закрывает инцидент
```

## Быстрый запуск через Docker compose

Склонируйте репозиторий:

```bash
git clone https://github.com/Maestro1749/StatusGuard.git
cd StatusGuard
```

Создайте `.env` на основе примера:

```bash
cp .env.example .env
```

Пример `.env`:

```env
APP_PORT=8080

DATABASE_URL=postgres://postgres:postgres@db:5432/StatusGuard?sslmode=disable
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=StatusGuard

CHECKER_WORKERS=5
SCHEDULER_INTERVAL_SECONDS=20
```

Запустите проект:

```bash
docker compsoe up --build
```

После запуска API будет доступно по адресу:

```text
http://localhost:8080
```

Проверить, что приложение работает:

```bash
curl http://localhost:8080/health
```

Ожидаемый ответ:

```json
{
    "status": "ok"
}
```

## Переменные окружения

| Переменная | Назначение | Пример |
|---|---|---|
| `APP_PORT` | Порт HTTP-сервера | `8080` |
| `DATABASE_URL` | Строка подключения к PostgreSQL | `postgres://postgres:postgres@db:5432/StatusGuard?sslmode=disable` |
| `POSTGRES_PORT` | Порт PostgreSQL на хосте | `5432` |
| `POSTGRES_USER` | Пользователь PostgreSQL | `postgres` |
| `POSTGRES_PASSWORD` | Пароль PostgreSQL | `postgres` |
| `POSRTGRES_DB` | Название бызы данных | `StatusGuard` |
| `CHECKER_WORKERS` | Количество воркеров для фоновых проверок | `5` |
| `SCHEDULER_INTERVAL_SECONDS` | Интервал запуска scheduler | `20` |

## API

### Health check

```http
GET /health
```

Проверяет, что приложение запущено.

Пример:

```bash
curl http://localhost:8080/health
```

### Создать target

```http
POST /targets
```

Пример запроса:

```bash
curl -X POST http://localhost:8080/targets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Example",
    "url": "https://example.com",
    "method": "GET",
    "expected_status": 200,
    "interval_seconds": 60,
    "timeout_seconds": 5
  }'
```

Поля:

| Поле | Описание |
|---|---|
| `name` | Название сервиса |
| `url` | URL сервиса |
| `method` | HTTP-метод. Сейчас поддерживается `GET` |
| `expected_status` | Ожидаемый HTTP-статус |
| `interval_seconds` | Интервал проверки конкретного target. Минимум `10` |
| `timeout_seconds` | Таймаут HTTP-запроса. От `1` до `30` секунд |

### Получить все targets

```http
GET /targets
```

Пример:

```bash
curl http://localhost:8080/targets
```

### Получить target по id

```http
GET /targets/{id}
```

Пример:

```bash
curl http://localhost:8080/targets/1
```

### Обновить target

```http
PATCH /targets/{id}
```

Пример:

```bash
curl -X PATCH http://localhost:8080/targets/1 \
  -H "Content-Type: application/json" \
  -d '{
    "timeout_seconds": 10,
    "enabled": true
  }'
```

Поддерживается частичное обновление. Можно передавать только те поля, которые нужно изменить.

### Удалить target

```http
DELETE /targets/{id}
```

Пример:

```bash
curl -X DELETE http://localhost:8080/targets/1
```

### Выполнить ручную проверку target

```http
POST /targets/{id}/check
```

Пример:

```bash
curl -X POST http://localhost:8080/targets/1/check
```

Проверка выполнит HTTP-запрос к target, сохранит результат в базе и вернёт результат проверки.

### Получить историю проверок target

```http
GET /targets/{id}/checks?limit=20
```

Пример:

```bash
curl "http://localhost:8080/targets/1/checks?limit=10"
```

### Получить открытые инциденты

```http
GET /incidents/open
```

Пример:

```bash
curl http://localhost:8080/incidents/open
```

### Получить открытые инциденты по target

```http
GET /targets/{id}/incidents
```

Пример:

```bash
curl http://localhost:8080/targets/1/incidents
```

## Как проверить работу инцидентов

Самый простой способ — добавить target с несуществующим адресом или с ожидаемым статусом, который не совпадает с реальным ответом.

Пример с неправильным expected status:

```bash
curl -X POST http://localhost:8080/targets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Broken expected status",
    "url": "https://example.com",
    "method": "GET",
    "expected_status": 500,
    "interval_seconds": 60,
    "timeout_seconds": 5
  }'
```

`https://example.com` обычно возвращает `200`, поэтому при `expected_status: 500` проверка должна получить статус `DOWN`. После фоновой проверки scheduler должен создать открытый инцидент.

Проверить открытые инциденты:

```bash
curl http://localhost:8080/incidents/open
```

Чтобы проверить закрытие инцидента, обновите target и верните корректный ожидаемый статус:

```bash
curl -X PATCH http://localhost:8080/targets/1 \
  -H "Content-Type: application/json" \
  -d '{
    "expected_status": 200
  }'
```

После следующей успешной проверки инцидент должен перейти в статус `resolved`.

## Миграции

При запуске через Docker Compose миграции применяются автоматически отдельным контейнером `migrate`.

В проекте создаются таблицы:

- `targets` — сервисы для мониторинга;
- `check_result` — история проверок;
- `incidents` — инциденты по недоступным сервисам;
- `settings` — таблица под настройки приложения.

## Логи

Логи пишутся в stdout и в файл:

```text
logs/app.log
```

При запуске через Docker Compose директория `logs` пробрасывается в контейнер как volume.

## Текущие ограничения

- Сейчас поддерживается только HTTP-метод `GET`.
- Уведомления пока представлены noop-реализацией без реальной отправки сообщений.
- Нет авторизации и пользовательских ролей.
- Нет frontend-интерфейса.
- Нет unit/integration-тестов.
- Поле `interval_seconds` у target пока не используется как индивидуальное расписание проверки; scheduler запускается по общему `SCHEDULER_INTERVAL_SECONDS`.
- JSON-ответы некоторых сущностей требуют унификации через `json` tags.

## Планы по доработке

- Добавить реальные уведомления, например Telegram или почту.
- Добавить тесты для service и repository слоёв.
- Улучшить формат JSON-ответов.
- Добавить поддержку разных HTTP-методов.
- Реализовать индивидуальный интервал проверки для каждого target.
- Добавить фильтрацию и пагинацию истории проверок.
