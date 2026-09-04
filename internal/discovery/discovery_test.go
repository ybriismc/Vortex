package discovery

import (
	"errors"
	"testing"
)

func TestRoundRobinCyclesThroughThePool(t *testing.T) {
	d, err := NewBalanced("round_robin", []string{"a", "b", "c"}, nil)
	if err != nil {
		t.Fatalf("failed to create discovery: %v", err)
	}

	for i, expected := range []string{"a", "b", "c", "a"} {
		addr, err := d.Discover(nil)
		if err != nil {
			t.Fatalf("discover %d failed: %v", i, err)
		}

		if addr != expected {
			t.Fatalf("discover %d returned %q, expected %q", i, addr, expected)
		}
	}
}

func TestFallbackRequiresAPool(t *testing.T) {
	d, err := NewBalanced("first", []string{"a"}, nil)
	if err != nil {
		t.Fatalf("failed to create discovery: %v", err)
	}

	if _, err := d.DiscoverFallback(nil); !errors.Is(err, ErrNoServers) {
		t.Fatalf("expected ErrNoServers, got %v", err)
	}

	d.SetFallback([]string{"b"})
	addr, err := d.DiscoverFallback(nil)
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}

	if addr != "b" {
		t.Fatalf("fallback returned %q, expected \"b\"", addr)
	}
}

func TestUnknownStrategyIsRejected(t *testing.T) {
	if _, err := NewBalanced("nearest", []string{"a"}, nil); err == nil {
		t.Fatal("expected an error for an unknown strategy")
	}
}
