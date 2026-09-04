// Package discovery implements Spectrum's server.Discovery interface with a
// pool of addresses and a load balancing strategy.
package discovery

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/sandertv/gophertunnel/minecraft"
)

// ErrNoServers is returned when the requested pool holds no address.
var ErrNoServers = errors.New("no servers available")

// Strategy determines how an address is picked from a pool.
type Strategy string

const (
	// RoundRobin cycles through the pool, spreading the players evenly.
	RoundRobin Strategy = "round_robin"
	// Random picks a random address of the pool.
	Random Strategy = "random"
	// First always picks the first address of the pool.
	First Strategy = "first"
)

// Balanced is a server.Discovery that balances the players over a pool of
// primary servers and keeps a separate pool for fallbacks. Both pools may be
// updated while the proxy is running.
type Balanced struct {
	strategy Strategy
	primary  []string
	fallback []string
	counter  atomic.Uint64
	mu       sync.RWMutex
}

// NewBalanced creates a Balanced discovery using the given pools and strategy.
func NewBalanced(strategy string, primary []string, fallback []string) (*Balanced, error) {
	if len(primary) == 0 {
		return nil, ErrNoServers
	}

	s := Strategy(strategy)
	if !slices.Contains([]Strategy{RoundRobin, Random, First}, s) {
		return nil, fmt.Errorf("unknown balancer strategy %q", strategy)
	}
	return &Balanced{
		strategy: s,
		primary:  slices.Clone(primary),
		fallback: slices.Clone(fallback),
	}, nil
}

// Discover ...
func (b *Balanced) Discover(_ *minecraft.Conn) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.pick(b.primary)
}

// DiscoverFallback ...
func (b *Balanced) DiscoverFallback(_ *minecraft.Conn) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.fallback) == 0 {
		return "", ErrNoServers
	}
	return b.pick(b.fallback)
}

// Primary returns the current pool of primary servers.
func (b *Balanced) Primary() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Clone(b.primary)
}

// SetPrimary replaces the pool of primary servers.
func (b *Balanced) SetPrimary(servers []string) error {
	if len(servers) == 0 {
		return ErrNoServers
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.primary = slices.Clone(servers)
	return nil
}

// Fallback returns the current pool of fallback servers.
func (b *Balanced) Fallback() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Clone(b.fallback)
}

// SetFallback replaces the pool of fallback servers.
func (b *Balanced) SetFallback(servers []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fallback = slices.Clone(servers)
}

// pick returns an address of the pool following the configured strategy.
func (b *Balanced) pick(servers []string) (string, error) {
	switch {
	case len(servers) == 0:
		return "", ErrNoServers
	case len(servers) == 1:
		return servers[0], nil
	}

	switch b.strategy {
	case Random:
		return servers[rand.Intn(len(servers))], nil
	case First:
		return servers[0], nil
	default:
		return servers[int(b.counter.Add(1)-1)%len(servers)], nil
	}
}
