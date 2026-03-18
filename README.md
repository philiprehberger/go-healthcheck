# go-healthcheck

[![CI](https://github.com/philiprehberger/go-healthcheck/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-healthcheck/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-healthcheck.svg)](https://pkg.go.dev/github.com/philiprehberger/go-healthcheck)
[![License](https://img.shields.io/github/license/philiprehberger/go-healthcheck)](LICENSE)

Health and readiness endpoint builder for Go HTTP services. Built for Kubernetes

## Installation

```bash
go get github.com/philiprehberger/go-healthcheck
```

## Usage

```go
package main

import (
	"net/http"

	hc "github.com/philiprehberger/go-healthcheck"
)

func main() {
	h := hc.New()

	h.AddLivenessCheck("goroutines", hc.GoroutineCount(10000))
	h.AddReadinessCheck("dns", hc.DNSResolve("example.com"))

	http.Handle("/healthz", h.LivenessHandler())
	http.Handle("/readyz", h.ReadinessHandler())
	http.ListenAndServe(":8080", nil)
}
```

## Built-in Checks

| Check | Description |
|-------|-------------|
| `DatabasePing(db)` | Pings a `*sql.DB` connection |
| `TCPDial(addr)` | Dials a TCP address (host:port) |
| `GoroutineCount(max)` | Fails if goroutine count exceeds max |
| `DNSResolve(host)` | Resolves a hostname via DNS |

## Check Options

```go
h.AddLivenessCheck("db", hc.DatabasePing(db),
	hc.WithTimeout(2*time.Second),
	hc.WithCacheTTL(5*time.Second),
)
```

| Option | Description |
|--------|-------------|
| `WithTimeout(d)` | Maximum duration for the check |
| `WithCacheTTL(d)` | Cache the result for the given duration |

## Response Format

```json
{
  "status": "up",
  "checks": {
    "goroutines": {
      "status": "up",
      "latency": "52.1µs"
    },
    "dns": {
      "status": "up",
      "latency": "1.23ms"
    }
  }
}
```

Returns HTTP 200 when all checks pass, HTTP 503 when any check fails.

## API

| Function / Type | Description |
|-----------------|-------------|
| `New()` | Creates a new Health instance |
| `AddLivenessCheck(name, fn, ...opts)` | Registers a liveness check |
| `AddReadinessCheck(name, fn, ...opts)` | Registers a readiness check |
| `LivenessHandler()` | Returns an `http.Handler` for liveness |
| `ReadinessHandler()` | Returns an `http.Handler` for readiness |
| `DatabasePing(db)` | Check that pings a database |
| `TCPDial(addr)` | Check that dials a TCP address |
| `GoroutineCount(max)` | Check that limits goroutine count |
| `DNSResolve(host)` | Check that resolves a hostname |
| `WithTimeout(d)` | Option: per-check timeout |
| `WithCacheTTL(d)` | Option: result caching duration |

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
