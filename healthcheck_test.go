package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("expected non-nil Health")
	}
}

func TestAddLivenessCheck(t *testing.T) {
	h := New()
	h.AddLivenessCheck("test", func(_ context.Context) error { return nil })
	if len(h.livenessChecks) != 1 {
		t.Fatalf("expected 1 liveness check, got %d", len(h.livenessChecks))
	}
	if h.livenessChecks[0].name != "test" {
		t.Fatalf("expected check name 'test', got %q", h.livenessChecks[0].name)
	}
}

func TestAddReadinessCheck(t *testing.T) {
	h := New()
	h.AddReadinessCheck("ready", func(_ context.Context) error { return nil })
	if len(h.readyChecks) != 1 {
		t.Fatalf("expected 1 readiness check, got %d", len(h.readyChecks))
	}
	if h.readyChecks[0].name != "ready" {
		t.Fatalf("expected check name 'ready', got %q", h.readyChecks[0].name)
	}
}

func TestLivenessHandler_AllHealthy(t *testing.T) {
	h := New()
	h.AddLivenessCheck("a", func(_ context.Context) error { return nil })
	h.AddLivenessCheck("b", func(_ context.Context) error { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Status != "up" {
		t.Fatalf("expected status 'up', got %q", resp.Status)
	}
	if len(resp.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(resp.Checks))
	}
	for name, cr := range resp.Checks {
		if cr.Status != "up" {
			t.Fatalf("check %q: expected 'up', got %q", name, cr.Status)
		}
	}
}

func TestLivenessHandler_OneFailing(t *testing.T) {
	h := New()
	h.AddLivenessCheck("ok", func(_ context.Context) error { return nil })
	h.AddLivenessCheck("fail", func(_ context.Context) error { return errors.New("broken") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Status != "down" {
		t.Fatalf("expected status 'down', got %q", resp.Status)
	}
	if resp.Checks["fail"].Status != "down" {
		t.Fatalf("expected 'fail' check to be 'down'")
	}
	if resp.Checks["ok"].Status != "up" {
		t.Fatalf("expected 'ok' check to be 'up'")
	}
}

func TestReadinessHandler_AllHealthy(t *testing.T) {
	h := New()
	h.AddReadinessCheck("db", func(_ context.Context) error { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.ReadinessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Status != "up" {
		t.Fatalf("expected status 'up', got %q", resp.Status)
	}
}

func TestCheckTimeout(t *testing.T) {
	h := New()
	h.AddLivenessCheck("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}, WithTimeout(50*time.Millisecond))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.LivenessHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var resp response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Checks["slow"].Status != "down" {
		t.Fatalf("expected 'slow' check to be 'down'")
	}
}

func TestCheckCaching(t *testing.T) {
	callCount := 0
	h := New()
	h.AddLivenessCheck("cached", func(_ context.Context) error {
		callCount++
		return nil
	}, WithCacheTTL(1*time.Second))

	// First call — executes the check.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.LivenessHandler().ServeHTTP(rec, req)

	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call — should use cached result.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.LivenessHandler().ServeHTTP(rec, req)

	if callCount != 1 {
		t.Fatalf("expected still 1 call (cached), got %d", callCount)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from cached result, got %d", rec.Code)
	}
}
