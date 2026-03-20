package healthcheck

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"
)

// DatabasePing returns a CheckFunc that pings the given database connection.
// It calls db.PingContext with the provided context, respecting any deadline
// or cancellation set on the context.
func DatabasePing(db *sql.DB) CheckFunc {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}

// TCPDial returns a CheckFunc that attempts a TCP connection to the given
// address (host:port). The connection is closed immediately after a
// successful dial. The dial respects any deadline set on the context.
func TCPDial(addr string) CheckFunc {
	return func(ctx context.Context) error {
		timeout := 5 * time.Second
		if dl, ok := ctx.Deadline(); ok {
			timeout = time.Until(dl)
		}
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return fmt.Errorf("tcp dial %s: %w", addr, err)
		}
		return conn.Close()
	}
}

// GoroutineCount returns a CheckFunc that fails if the current number of
// goroutines exceeds the specified maximum. This is useful for detecting
// goroutine leaks.
func GoroutineCount(max int) CheckFunc {
	return func(_ context.Context) error {
		count := runtime.NumGoroutine()
		if count > max {
			return fmt.Errorf("goroutine count %d exceeds max %d", count, max)
		}
		return nil
	}
}

// DNSResolve returns a CheckFunc that resolves the given hostname using
// net.LookupHost. It fails if the hostname cannot be resolved.
func DNSResolve(host string) CheckFunc {
	return func(ctx context.Context) error {
		resolver := net.Resolver{}
		_, err := resolver.LookupHost(ctx, host)
		if err != nil {
			return fmt.Errorf("dns resolve %s: %w", host, err)
		}
		return nil
	}
}

// HTTPEndpoint returns a check that verifies an HTTP endpoint responds with a 2xx status.
func HTTPEndpoint(url string) CheckFunc {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("http request: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return nil
	}
}

// MemoryUsage returns a check that verifies memory usage is below the given threshold in bytes.
func MemoryUsage(maxBytes uint64) CheckFunc {
	return func(_ context.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.Alloc > maxBytes {
			return fmt.Errorf("memory usage %d bytes exceeds threshold %d bytes", m.Alloc, maxBytes)
		}
		return nil
	}
}
