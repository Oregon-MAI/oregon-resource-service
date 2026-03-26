# Oregon Resource Service

Микросервис для управления ресурсами (переговорки, рабочие места, устройства).

Сейчас поднимаются 2 gRPC API:
- `ResourcePublicService` — CRUD + списки/доступность.
- `ResourceBookingService` — методы для бронирования.

## Быстрый старт (Docker Compose)

Запуск Postgres + сервиса:

```bash
docker compose up -d
```

Остановка:

```bash
docker compose down
```

По умолчанию открыты порты:
- `60009` — public gRPC
- `60008` — booking gRPC
- `5432` — Postgres

## Локальный запуск

```bash
go run ./cmd/resource -config ./config/local.yml
```

## Тесты

Прогон всех тестов:

```bash
go test ./...
```

Покрытие по ключевым пакетам:

```bash
go test ./internal/service/resource ./internal/grpc/resource/public ./internal/grpc/resource/booking ./internal/grpc/resource/utils -cover
```

## Список ручек

### `ResourcePublicService`

- `CreateResource`
- `GetResource`
- `GetResourcesList`
- `UpdateResource`
- `DeleteResource`
- `ChangeResourceStatus`
- `GetAvailableResources`

### `ResourceBookingService`

- `GetResource`
- `CheckResourceStatus`
- `GetAvailableResources`
- `UpdateResourceOccupancy`
