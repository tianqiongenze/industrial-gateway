# 工业网关数据采集服务 (Industrial Gateway Data Collection Service)

> Student: `student-go` — Language: Go 1.21 + Gin framework
> Architecture: Hexagonal Architecture (Ports & Adapters)

An industrial data-collection gateway that ingests device telemetry from MQTT
brokers and Modbus TCP PLCs, normalises it through a transformation pipeline,
persists it, classifies it against threshold policies, and exposes it via a
Gin REST API, a gRPC service, and Prometheus metrics. Built following
hexagonal architecture so every external system (broker, PLC, HTTP, gRPC,
database, cache) is an *adapter* behind a *port*, making the core domain logic
pure and trivially testable.

## Architecture (Hexagonal / Ports & Adapters)

```
       driving adapters                 |  domain  |           driven adapters
   ┌──────────────────┐                |          |        ┌──────────────────┐
   │  Gin REST API    │ ─► usecase ─► |  domain  | ─►     │ MQTT collector   │
   │  gRPC service    │   (IngestSvc) |  Reading |        │ Modbus collector  │
   │  scheduler (cron)│                |  Pipeline|        │ Repository (mem) │
   └──────────────────┘                |  Classify│        │ Cache (mem)      │
                                       └──────────┘        └──────────────────┘
   Ports: CollectorPort, RepositoryPort, CachePort  — interfaces in domain/.
```

## Modules
- `internal/domain/` — pure model (`Reading`, `DeviceStatus`), the
  `Transformer`/`Pipeline`, `Classify` policy, and the port interfaces.
- `internal/usecase/` — `IngestService` orchestrating collectors→pipeline→repo→cache.
- `internal/infra/memoryrepo`, `memorycache` — in-memory driven adapters.
- `internal/infra/mqtt`, `modbus` — field-device collectors (client abstracted).
- `internal/adapter/http` — Gin REST primary adapter.
- `internal/adapter/grpc` — gRPC service primary adapter.
- `internal/adapter/metrics` — Prometheus counters.
- `cmd/gateway/main.go` — wires the hexagon, starts REST + metrics + scheduler.

## Build & Test
```bash
go mod tidy
go build ./...
go test -cover ./...      # table-driven tests across every package
go run ./cmd/gateway      # REST :8080, metrics :9090
```

## API Example
```bash
curl -X POST localhost:8080/api/v1/ingest \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"pump-1","metric":"temperature","value":96,"unit":"C","source":"rest"}'
curl localhost:8080/api/v1/health
curl localhost:8080/api/v1/status/pump-1
```

See `docs/SDD.md` for the full Software Design Document.
