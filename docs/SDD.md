# Software Design Document (SDD)
## 工业网关数据采集服务 — Industrial Gateway Data Collection Service

**Project:** Industrial Gateway &nbsp; **Student:** student-go &nbsp;
**Language:** Go 1.21 &nbsp; **Framework:** Gin v1.10 &nbsp;
**Architecture:** Hexagonal (Ports & Adapters)

> AI-assisted design: the port/adapter boundaries and Modbus register-mapping
> model were validated against the `qwen2.5-coder:7b` model via the platform
> LiteLLM gateway. Implementation and table-driven tests are hand-written.

---

## 1. Introduction

### 1.1 Purpose
A gateway that sits between field devices (MQTT sensors, Modbus PLCs) and
upstream analytics platforms, providing a single normalised ingestion point.
It decouples device protocols from consumers by exposing a clean REST/gRPC
surface and a Prometheus metrics endpoint.

### 1.2 Scope
Inbound: MQTT JSON messages and Modbus TCP holding registers. Processing:
validation, transformation pipeline (unit normalisation, scaling, F→C),
persistence, threshold classification, live-status caching. Outbound: REST API,
gRPC service, Prometheus metrics, scheduled collector polling. Out of scope:
OPC-UA, BACnet, and long-term historian storage (left to the downstream
platform).

### 1.3 Definitions
- **Port** — an interface defined by the application that an adapter must
  satisfy (Alistair Cockburn's hexagonal terminology).
- **Adapter** — a concrete implementation of a port (driving = REST/gRPC/scheduler;
  driven = MQTT/Modbus/repo/cache).
- **Reading** — the canonical `{device_id, metric, value, unit, timestamp,
  source, quality}` measurement.
- **Modbus holding register** — a 16-bit value addressed by (address, quantity).

---

## 2. Architecture (Hexagonal)

### 2.1 Dependency Rule
All dependencies point **inward** toward the `domain` package:
`adapters → usecase → domain`. The domain imports only the standard library.
This means the gateway can swap MQTT for NATS, Gin for Echo, or in-memory
storage for PostgreSQL without touching a line of business logic.

### 2.2 Ports

| Port | Package | Adapters |
|------|---------|----------|
| `CollectorPort` | domain | `mqtt.Collector`, `modbus.Collector` |
| `RepositoryPort` | domain | `memoryrepo.Repository` (PostgreSQL in prod) |
| `CachePort` | domain | `memorycache.Cache` (Redis in prod) |

### 2.3 Adapters
- **Driving (primary):** `http.Router` (Gin), `grpc.Service`,
  `runScheduler` (cron driver).
- **Driven (secondary):** `mqtt.Collector` (abstracted `Client`), `modbus.Collector`
  (abstracted `Client`), `memoryrepo`, `memorycache`.

### 2.4 Data Flow
```
[scheduler | REST | gRPC] ─► IngestService.RunOnce / .Ingest
   ─► for each collector: collector.Collect() ─► []Reading
   ─► pipeline.Apply(reading)  ─► normalised Reading
   ─► repo.Save(reading)
   ─► cache.Set("status:<device>", Classify(reading))
```

---

## 3. Domain Model

```
Reading { DeviceID, Metric, Value, Unit, Timestamp, Source, Quality }
  Validate() -> error            // invariants
Transformer { normaliseUnits, scaling }
  Transform(Reading) -> Reading
Pipeline { Steps []*Transformer }
  Apply(Reading) -> Reading
Classify(Reading) -> NORMAL|WARNING|CRITICAL   // threshold table
```

---

## 4. Component Design

### 4.1 Transformation Pipeline
A `Pipeline` is an ordered list of `Transformer`s. Each `Transformer`
normalises units (e.g. "C"→"Celsius"), converts Fahrenheit→Celsius as a
special case, and applies a per-metric scaling multiplier (raw counts→SI).
Pure functions; fully table-driven tested.

### 4.2 MQTT Collector
Abstracts the broker behind a `Client` interface (`Subscribe`/`Disconnect`).
Incoming JSON is decoded into `Reading`s and buffered in a queue; `Collect()`
drains the queue. Tests inject a `fakeClient` that captures the handler and
replays canned payloads — no broker required.

### 4.3 Modbus Collector
A `RegisterMap` maps `{address, quantity, deviceID, metric, unit, scale,
offset}` to a reading. The adapter reads holding registers via an abstracted
`Client` and scales the raw 16-bit value. Tests inject a `fakeClient` with a
register map.

### 4.4 REST API (Gin)
`/api/v1/health`, `/api/v1/ingest`, `/api/v1/readings/:device`,
`/api/v1/status/:device`. Validation via Gin binding; domain errors map to
422; collector/repository errors map to 400/500.

### 4.5 gRPC Service
A typed `Service.Ingest(ctx, req)` mirroring the REST flow. Kept as a
hand-written skeleton (no protoc dependency) so the package builds in the
sandbox; the request/response structs are gRPC-message-shaped.

### 4.6 Metrics & Health
Prometheus counters (`gateway_ingest_total`, `gateway_collector_errors_total`)
on `:9090/metrics`. Health endpoint reports reading count + cache type.

---

## 5. Sequence Diagram

```
Scheduler ─► IngestService.RunOnce
             │ for each collector:
             │   collector.Collect() ─► []Reading
             │   for each reading:
             │     pipeline.Apply(reading)
             │     repo.Save(normalised)
             │     cache.Set("status:<dev>", Classify)
             └─► count
REST ─► http.ingest ─► repo.Save ─► Classify ─► 201 + status
gRPC  ─► grpc.Service.Ingest ─► validate ─► Classify ─► response
```

---

## 6. Test Strategy (table-driven, TDD)

Every package has a `_test.go` with table-driven cases:

| Package | Tests | Covers |
|---------|-------|--------|
| domain | Reading.Validate, Transformer.Transform, Pipeline.Apply, Classify | invariants, unit conversion, scaling, thresholds |
| infra/memoryrepo | Save, Count, Latest filter/order, invalid rejection | RepositoryPort |
| infra/memorycache | Set/Get, TTL, overwrite | CachePort |
| infra/mqtt | decode+queue, drain, nil-client | MQTT adapter |
| infra/modbus | poll maps, scaling, nil-client | Modbus adapter |
| adapter/http | health, ingest, validation, latest, status | REST (httptest) |
| adapter/grpc | ingest, missing device, invalid value | gRPC logic |
| usecase | RunOnce, StatusFor, invalid skipped | orchestration |

Run: `go test -cover ./...`.

---

## 7. Non-Functional Requirements
- **Concurrency:** `RepositoryPort`/`CachePort` use `sync.Mutex`/`RWMutex`;
  the scheduler runs in a goroutine; context propagation enables cancellation.
- **Performance:** table-driven unit tests run in ms; no I/O in the domain.
- **Portability:** adapters swap via constructor injection; no global state.
- **Observability:** structured logging + Prometheus metrics.

## 8. Future Work
- Real gRPC codegen + protobuf service registration.
- NATS/Kafka collector; PostgreSQL repository; Redis cache adapters.
- Circuit breakers around collector clients.

## 9. Revision History
| Version | Date | Author | Notes |
|---------|------|--------|-------|
| 1.0 | 2026-08-31 | student-go | Initial SDD, hexagonal architecture, table-driven tests |
---

## 10. Design Rationale & Trade-offs

### 10.1 Why hexagonal over a conventional layered design
Field device protocols (MQTT, Modbus) and consumer protocols (REST, gRPC)
change at different rates and for different reasons. By expressing every
external system as an *adapter* behind a *port* interface defined in
`domain`, the core ingestion logic (`IngestService`) becomes immune to
protocol churn: swapping MQTT for NATS, Gin for Echo, or the in-memory
repository for PostgreSQL touches only one adapter and zero lines of
business logic. The domain package imports nothing but the standard
library, which is the operational test that the dependency rule holds.

### 10.2 Abstracted transport clients
Both the MQTT and Modbus adapters depend on a minimal `Client` interface
(`Subscribe`/`Disconnect` and `ReadHoldingRegisters`/`Close`) rather than a
concrete library. This has two pay-offs: (1) unit tests inject a `fakeClient`
that captures the message handler or returns canned register values, so the
adapters are tested with no broker and no PLC; (2) a real deployment can
substitute `paho.mqtt` or a vendor Modbus library without changing the
adapter's `Collect()` logic, only the client construction at the composition
root.

### 10.3 Table-driven tests and the pure domain
Every domain package ships `_test.go` files using the canonical Go
`[]struct{...}` + `t.Run` table-driven pattern. Because the domain is pure
(has no I/O), each sub-test is deterministic and runs in microseconds. The
adapters are tested with table-driven cases too, but against fakes — so the
entire suite completes in well under a second. The only package with
network-ish behaviour is the HTTP adapter, and it is tested with
`httptest.NewRecorder`, again with no real port binding.

### 10.4 Register-mapping model for Modbus
A `RegisterMap` struct declares `(address, quantity, deviceID, metric, unit,
scale, offset)`. This declarative approach means adding a new sensor channel
is a configuration change, not a code change — the adapter reads the map list,
polls each address, scales the raw 16-bit value, and emits a canonical
`Reading`. The model deliberately supports one reading per map (the first
register of the read) to keep the example tractable; a production version
would interpret multi-register values (e.g. 32-bit floats across two
registers) — an extension point that does not alter the port or pipeline.

### 10.5 Scheduler as a driving adapter
The cron-style `runScheduler` goroutine is itself a *driving* adapter: it
calls `IngestService.RunOnce` on an interval, exactly as the REST and gRPC
adapters do on demand. Treating time as just another input boundary keeps the
use case synchronous and free of timers; the goroutine owns the ticker and
honours context cancellation for graceful shutdown. This separation is why
`RunOnce` is trivially unit-testable — the test constructs the service with
fakes and calls `RunOnce` directly, no waiting.

---

## 11. Quality Attributes (NFRs) in detail

| Attribute | Strategy |
|-----------|----------|
| Concurrency | `sync.Mutex`/`RWMutex` in adapters; goroutine scheduler; context cancellation |
| Performance | Pure domain logic; no I/O on the hot path; NumPy-equivalent vectorisation not needed for counts |
| Portability | Adapter swap via constructor injection; no global state; standard library domain |
| Observability | Structured `log` + Prometheus counters on `:9090/metrics` |
| Safety | Strong typing; explicit error returns; no exceptions panicking across adapters |

---

## 12. Glossary
- **Hexagonal architecture** — Alistair Cockburn's pattern of ports (interfaces)
  and adapters (implementations) isolating application logic from technology.
- **Port** — an interface in the application that an adapter must satisfy.
- **Holding register** — a 16-bit read/write Modbus value addressed by (address, quantity).
- **Pipeline** — an ordered list of transformers applied to normalise raw readings.
- **OEE** — Overall Equipment Effectiveness = Availability × Performance × Quality.
---

## 13. Operational Runbook

### 13.1 Startup sequence
1. `go run ./cmd/gateway` starts three goroutines: the REST API on `:8080`,
   the Prometheus metrics endpoint on `:9090`, and the collector scheduler
   polling every 5 seconds. 2. Inject real `mqtt.Collector` and
   `modbus.Collector` instances at the composition root in `main.go` (the
   shipped version ships with an empty collector list for a clean local run).
3. Configure collectors via environment flags or a config struct.

### 13.2 Failure modes
- **Collector error**: `RunOnce` logs the per-collector error, increments the
  `gateway_collector_errors_total` counter, and continues to the next
  collector — a single misbehaving device never blocks the whole batch.
- **Transform error**: the offending reading is skipped and logged; the rest
  of the batch proceeds.
- **Repository save error**: the reading is not counted as ingested and is
  logged; downstream cache writes for that reading are skipped.

### 13.3 Graceful shutdown
On `SIGINT`/`SIGTERM` the scheduler's context is cancelled, `RunOnce`
returns early on the next tick, and the process exits after a 500ms drain.
The in-memory adapters are lossy by design (a production deployment uses the
PostgreSQL repository adapter).

---

## 14. Alternatives Considered
- **Echo/Fiber over Gin**: Gin was chosen for ecosystem maturity and the
  binding/validation integration; the HTTP adapter is isolated so a swap is
  a single-file change.
- **Real gRPC codegen**: shipped as a hand-written skeleton to avoid the
  `protoc` toolchain in the sandbox; the service shape is gRPC-ready.
- **Global registry pattern**: rejected in favour of constructor injection;
  every adapter is passed its dependencies explicitly, which is what makes
  the table-driven tests able to substitute fakes without globals.

---

## 15. Concurrency Model
The repository and cache adapters guard their internal slices/maps with
`sync.Mutex`/`RWMutex` because the scheduler goroutine, the HTTP handlers,
and (in production) multiple gRPC handlers all call into the same ports
concurrently. The domain is deliberately free of locks: it operates on owned
values passed by the caller, so the locking policy lives entirely in the
adapters where the shared mutable state actually resides. Context
propagation (`context.Context`) threads cancellation from the scheduler down
into `RunOnce`, enabling a clean shutdown mid-batch.

---

## 16. Metrics & Alerting
Prometheus counters `gateway_ingest_total{source,status}` and
`gateway_collector_errors_total{collector}` are registered at package init
via `promauto`. A typical alert rule: rate of `collector_errors_total` above
zero for 5 minutes indicates a device or network fault; a sudden drop in
`ingest_total` indicates a collector outage. The `/metrics` endpoint on
`:9090` is scraped by the platform's Prometheus.
---

## 17. Lifecycle & Deployment
The gateway is a single statically-linked binary (`go build ./cmd/gateway`)
with no runtime dependencies, making it trivial to containerise in a
`FROM scratch`-style minimal image. In the cluster it runs as a Deployment
behind an Ingress exposing `:8080`; the `:9090` metrics port is scraped by
Prometheus via a `ServiceMonitor`. Horizontal scaling is safe because the
shipped adapters are either stateless (HTTP, gRPC) or self-contained
(in-memory repo/cache); a production deployment swaps in shared PostgreSQL
and Redis adapters at the composition root and scales horizontally. Rolling
updates are zero-downtime because each instance polls independently and the
scheduler honours context cancellation within one tick interval.


This lifecycle model keeps operational complexity proportional to the number of services, not to the size of the codebase, and ensures that a single misconfigured collector never compromises the availability of the REST or metrics endpoints.
