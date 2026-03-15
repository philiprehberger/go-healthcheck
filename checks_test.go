package healthcheck

import (
	"context"
	"runtime"
	"testing"
)

func TestGoroutineCount_Under(t *testing.T) {
	// Set max well above the current count.
	max := runtime.NumGoroutine() + 1000
	fn := GoroutineCount(max)
	if err := fn(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGoroutineCount_Over(t *testing.T) {
	// Set max to zero so the check always fails.
	fn := GoroutineCount(0)
	if err := fn(context.Background()); err == nil {
		t.Fatal("expected error for goroutine count over max")
	}
}

func TestDNSResolve_Valid(t *testing.T) {
	fn := DNSResolve("localhost")
	if err := fn(context.Background()); err != nil {
		t.Fatalf("expected no error resolving localhost, got: %v", err)
	}
}

func TestTCPDial(t *testing.T) {
	// Attempt to dial a port that is almost certainly not listening.
	// This verifies the check returns an error for unreachable addresses.
	fn := TCPDial("127.0.0.1:1")
	err := fn(context.Background())
	if err == nil {
		t.Fatal("expected error dialing unreachable port")
	}
}
