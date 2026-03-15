// Package healthcheck provides health and readiness endpoint builders for HTTP services.
//
// It supports registering liveness and readiness checks, running them in parallel
// with individual timeouts, caching results, and serving JSON responses compatible
// with Kubernetes health probes.
package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// CheckFunc is a health check function. It receives a context that may carry
// a timeout and should return nil if healthy or an error if unhealthy.
type CheckFunc = func(ctx context.Context) error

// CheckOption configures the behavior of an individual health check.
type CheckOption func(*checkConfig)

type checkConfig struct {
	timeout  time.Duration
	cacheTTL time.Duration
}

// WithTimeout sets the maximum duration a check is allowed to run.
// If the check does not complete within this duration, it is considered failed.
func WithTimeout(d time.Duration) CheckOption {
	return func(c *checkConfig) {
		c.timeout = d
	}
}

// WithCacheTTL sets how long a check result is cached before the check is
// re-executed. A zero value disables caching (the default).
func WithCacheTTL(d time.Duration) CheckOption {
	return func(c *checkConfig) {
		c.cacheTTL = d
	}
}

type check struct {
	name   string
	fn     CheckFunc
	config checkConfig

	mu         sync.RWMutex
	cachedErr  error
	cachedAt   time.Time
	cachedUsed bool
}

func (c *check) run(ctx context.Context) (error, time.Duration) {
	// Check cache first.
	if c.config.cacheTTL > 0 {
		c.mu.RLock()
		if c.cachedUsed && time.Since(c.cachedAt) < c.config.cacheTTL {
			err := c.cachedErr
			c.mu.RUnlock()
			return err, 0
		}
		c.mu.RUnlock()
	}

	// Apply per-check timeout.
	if c.config.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.timeout)
		defer cancel()
	}

	start := time.Now()
	err := c.fn(ctx)
	latency := time.Since(start)

	// Update cache.
	if c.config.cacheTTL > 0 {
		c.mu.Lock()
		c.cachedErr = err
		c.cachedAt = time.Now()
		c.cachedUsed = true
		c.mu.Unlock()
	}

	return err, latency
}

// Health holds registered liveness and readiness checks and provides HTTP
// handlers that execute them and return JSON responses.
type Health struct {
	mu             sync.RWMutex
	livenessChecks []*check
	readyChecks    []*check
}

// New creates a new Health instance with no registered checks.
func New() *Health {
	return &Health{}
}

// AddLivenessCheck registers a named check that will be executed by the
// liveness handler. Options can configure per-check timeout and caching.
func (h *Health) AddLivenessCheck(name string, fn CheckFunc, opts ...CheckOption) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.livenessChecks = append(h.livenessChecks, newCheck(name, fn, opts))
}

// AddReadinessCheck registers a named check that will be executed by the
// readiness handler. Options can configure per-check timeout and caching.
func (h *Health) AddReadinessCheck(name string, fn CheckFunc, opts ...CheckOption) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readyChecks = append(h.readyChecks, newCheck(name, fn, opts))
}

func newCheck(name string, fn CheckFunc, opts []CheckOption) *check {
	c := &check{name: name, fn: fn}
	for _, opt := range opts {
		opt(&c.config)
	}
	return c
}

// LivenessHandler returns an http.Handler that executes all registered
// liveness checks in parallel and responds with JSON. It returns HTTP 200
// when all checks pass and HTTP 503 when any check fails.
func (h *Health) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		checks := make([]*check, len(h.livenessChecks))
		copy(checks, h.livenessChecks)
		h.mu.RUnlock()
		h.serveChecks(w, r, checks)
	})
}

// ReadinessHandler returns an http.Handler that executes all registered
// readiness checks in parallel and responds with JSON. It returns HTTP 200
// when all checks pass and HTTP 503 when any check fails.
func (h *Health) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.RLock()
		checks := make([]*check, len(h.readyChecks))
		copy(checks, h.readyChecks)
		h.mu.RUnlock()
		h.serveChecks(w, r, checks)
	})
}

type checkResult struct {
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

type response struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks"`
}

func (h *Health) serveChecks(w http.ResponseWriter, r *http.Request, checks []*check) {
	ctx := r.Context()

	type result struct {
		name    string
		err     error
		latency time.Duration
	}

	results := make([]result, len(checks))
	var wg sync.WaitGroup
	wg.Add(len(checks))

	for i, c := range checks {
		go func(i int, c *check) {
			defer wg.Done()
			err, latency := c.run(ctx)
			results[i] = result{name: c.name, err: err, latency: latency}
		}(i, c)
	}
	wg.Wait()

	resp := response{
		Status: "up",
		Checks: make(map[string]checkResult, len(results)),
	}

	for _, res := range results {
		cr := checkResult{
			Status:  "up",
			Latency: res.latency.String(),
		}
		if res.err != nil {
			cr.Status = "down"
			cr.Error = res.err.Error()
			resp.Status = "down"
		}
		resp.Checks[res.name] = cr
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Status == "down" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(resp)
}
