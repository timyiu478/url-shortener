# A URL Shortener Service

A URL shortener service built in Go. Designed for high availability, global scalability, and end-to-end observability, featuring distributed tracing, RED metrics, and structured logging.

---

## Demo

Watch the walkthrough video demonstrating local stack deployment, endpoint execution, trace propagation in Jaeger, and metric scraping in Prometheus:

[Watch the Live Demo Video](https://docs.google.com/videos/d/1QrrYpG016cxRjL34Qv4f2tmTglyUnCh6aavISQdJRfY/play)

---

## Benchmark

### Environment

* **OS / Kernel:** Linux 6.8.0-136-generic x86_64 (glibc 2.35)
* **CPU:** 13th Gen Intel(R) Core(TM) i5-13600 (4 logical cores)
* **Memory:** 15.4 GB

### Configuration
* **Target:** `http://localhost:8080`
* **Concurrency:** 100 connections
* **Workload Duration:** 60 seconds per run
* **Repeats:** 3 runs per workload (Warmup: 5s)
* **Total Wall Time:** 16m 51s
* **Timestamp:** 2026-09-07 13:15:58

### Performance Metrics

| Workload | Description | Success Rate | Throughput | p50 | p90 | p99 | p99.9 | Max |
| :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: |
| **Workload A** | 100% Write (`POST /newurl`) | 3/3 ok | **1,146.77 req/s** | 96.99 ms | 122.39 ms | 164.53 ms | 318.40 ms | 882.30 ms |
| **Workload B** | 90% Read / 10% Write | 3/3 ok | **1,231.92 req/s** | 51.67 ms | 131.83 ms | 597.57 ms | 2,149.17 ms | 6,199.07 ms |
| **Workload C** | 100% Read (Hot Keys) | 3/3 ok | **1,205.79 req/s** | 60.20 ms | 144.57 ms | 463.28 ms | 796.49 ms | 3,562.78 ms |
| **Workload D** | 100% Read (Cold/Miss) | 3/3 ok | **786.88 req/s** | 58.00 ms | 134.91 ms | 575.10 ms | 2,471.85 ms | 4,784.06 ms |

---

## Core Features

- **High Throughput & Low Latency:** Engineered to handle 1000+ req/sec with horizontal scaling capabilities.
- **Full-Stack Observability:**
  - **Distributed Tracing:** OpenTelemetry (OTLP gRPC) integration exporting spans to Jaeger.
  - **RED Metrics:** Custom Prometheus metrics (Rate, Errors, Duration) and DB latency tracking served via `/metrics`.
  - **Context-Aware Logging:** Structured JSON logging using Go `log/slog` that dynamically extracts and injects `trace_id` from request contexts.
- **Persistence:** Scalable NoSQL persistence powered by Amazon DynamoDB (supports DynamoDB Local / LocalStack for offline development).
- **Production Quality Gates:** Strict GitHub Actions CI pipeline running formatting checks (`gofmt`), static analysis (`go vet`), and race condition testing (`go test -race`).
- **Containerized Development Stack:** Single-command local environment managed via Docker Compose V2.

---

## API Specification

### 1. Submit New URL

Generates a unique short URL for a given target address.

- **Endpoint:** `POST /newurl`
- **Content-Type:** `application/json`

**Request Body:**
```json
{
  "domain": "shortenurl.org",
  "url": "[https://www.google.com](https://www.google.com)"
}
```

**Response Payload (`200 OK`):**
```json
{
  "shortenUrl": "[https://shortenurl.org/g20hi3k9Z](https://shortenurl.org/g20hi3k9Z)",
  "url": "[https://www.google.com](https://www.google.com)"
}
```

---

### 2. Redirect Short URL

Resolves a short key and directs the client to the original URL.

- **Endpoint:** `GET /{shortKey}`
- **Pattern:** `/[a-zA-Z0-9]{9}/`

**Request Example:**
```bash
# Inspect headers and location target
curl -i http://localhost:8080/g20hi3k9Z

# Follow redirects automatically
curl -L http://localhost:8080/g20hi3k9Z
```

> **Note on Redirect Semantics:**
> By design for this implementation, the `GET /{shortKey}` endpoint returns `HTTP 304 (Not Modified)` to signal that target resource handling is managed externally. To use standard HTTP redirection (`302 Found` or `307 Temporary Redirect`), update the handler in `internal/handlers/handlers.go`.

---

## System Architecture

Designed for multi-region deployment on AWS to guarantee fault tolerance and eliminate single points of failure.

```text
                  ┌────────────────────────┐
                  │    Route 53 / DNS      │
                  └───────────┬────────────┘
                              │
                  ┌───────────▼────────────┐
                  │ CloudFront / Global ALB│
                  └───────────┬────────────┘
                              │
          ┌───────────────────┴───────────────────┐
          │                                       │
┌─────────▼──────────┐                 ┌──────────▼─────────┐
│ Region A (Primary) │                 │ Region B (Failover)│
│ ┌────────────────┐ │                 │ ┌────────────────┐ │
│ │  Go API Nodes  │ │                 │ │  Go API Nodes  │ │
│ └───────┬────────┘ │                 │ └───────┬────────┘ │
└─────────┼──────────┘                 └─────────┼──────────┘
          │                                      │
          └───────────────────┬──────────────────┘
                              │
               ┌──────────────▼──────────────┐
               │ Amazon DynamoDB Global Table│
               └─────────────────────────────┘
```

---

## Quick Start (Local Environment)

### Prerequisites

- **Docker & Docker Compose V2** (`docker compose`)
- **Go 1.21+** (for host testing and compilation)

### 1. Launch the Observability & Infrastructure Stack

Start the Go API, DynamoDB Local, OpenTelemetry Collector, Jaeger, and Prometheus:

```bash
docker compose up -d
```

Verify that all services pass health checks:

```bash
docker compose ps
```

### 2. Initialize the Database

Create the required DynamoDB table by running the setup script:

```bash
make create-table
```

### 3. Test Endpoints

Create a short URL:

```bash
curl -i -X POST http://localhost:8080/newurl \
  -H "Content-Type: application/json" \
  -d '{"domain":"shortenurl.org","url":"[https://example.com](https://example.com)"}'
```

Resolve a short URL:

```bash
curl -i http://localhost:8080/<shortKey>
```

---

## Telemetry Dashboards

Once the local stack is running, access the telemetry interfaces directly from your browser:

| Interface | URL | Purpose |
| :--- | :--- | :--- |
| **Jaeger UI** | `http://localhost:16686` | Distributed end-to-end request tracing |
| **Prometheus** | `http://localhost:9090` | Scrapes API metrics (`/metrics`) |
| **API Health** | `http://localhost:8080/readyz` | Liveness and readiness health checks |

---

## Testing & CI Checks

Run local static analysis and test suites prior to pushing code:

```bash
# 1. Format validation
gofmt -l .

# 2. Static analysis
go vet ./...

# 3. Unit test execution with race detection
go test -v -race -cover ./...

# 4. Compilation verification
go build ./...
```
