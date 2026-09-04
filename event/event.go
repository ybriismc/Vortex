// Package event holds the event bus used by Vortex plugins.
//
// Handlers subscribe to a concrete event type and are called in priority
// order whenever the proxy fires that event. Events that implement
// Cancellable may be cancelled by a handler, which tells the proxy to drop
// the action the event describes.
package event

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
)

// Priority determines the order in which handlers of the same event run.
// Handlers run from Lowest to Monitor, so a Monitor handler observes the
// final state of the event after every other handler has seen it.
type Priority int

const (
	Lowest  Priority = -2
	Low     Priority = -1
	Normal  Priority = 0
	High    Priority = 1
	Highest Priority = 2
	Monitor Priority = 3
)

// Cancellable is implemented by events whose action can be stopped.
type Cancellable interface {
	// Cancel stops the action the event describes.
	Cancel()
	// Cancelled reports whether the event has been cancelled.
	Cancelled() bool
}

// Cancelling implements Cancellable and is embedded by cancellable events.
type Cancelling struct {
	cancelled atomic.Bool
}

// Cancel ...
func (c *Cancelling) Cancel() {
	c.cancelled.Store(true)
}

// Cancelled ...
func (c *Cancelling) Cancelled() bool {
	return c.cancelled.Load()
}

// handler is a single subscription to an event type.
type handler struct {
	priority Priority
	order    int
	owner    string
	call     func(any)
}

// registry holds the handlers shared by every view of a bus.
type registry struct {
	logger   *slog.Logger
	mu       sync.RWMutex
	handlers map[reflect.Type][]handler
	next     int
}

// Bus dispatches events to the handlers subscribed to them. A Bus is safe for
// concurrent use.
type Bus struct {
	registry *registry
	owner    string
}

// NewBus creates an empty Bus.
func NewBus(logger *slog.Logger) *Bus {
	return &Bus{registry: &registry{logger: logger, handlers: make(map[reflect.Type][]handler)}}
}

// For returns a view of the bus that attributes every subscription made
// through it to the given owner, which is reported when a handler panics.
// The handlers themselves are shared with the original bus.
func (b *Bus) For(owner string) *Bus {
	return &Bus{registry: b.registry, owner: owner}
}

// Subscribe registers a handler for the event type T. Handlers of the same
// priority run in the order they were subscribed.
func Subscribe[T any](b *Bus, h func(*T), priority Priority) {
	key := reflect.TypeFor[T]()
	r := b.registry

	r.mu.Lock()
	defer r.mu.Unlock()

	r.next++
	r.handlers[key] = append(r.handlers[key], handler{
		priority: priority,
		order:    r.next,
		owner:    b.owner,
		call:     func(e any) { h(e.(*T)) },
	})
	sort.SliceStable(r.handlers[key], func(i, j int) bool {
		return r.handlers[key][i].priority < r.handlers[key][j].priority
	})
}

// Call fires the event, running every handler subscribed to its type, and
// returns the event so that its fields may be read afterwards. A handler that
// panics is logged and skipped, keeping a faulty plugin from taking the proxy
// down with it.
func Call[T any](b *Bus, e *T) *T {
	if b == nil {
		return e
	}

	r := b.registry
	r.mu.RLock()
	handlers := r.handlers[reflect.TypeFor[T]()]
	r.mu.RUnlock()

	for _, h := range handlers {
		r.dispatch(h, e)
	}
	return e
}

// Subscribed reports whether the event type T has at least one handler. The
// proxy uses it to skip work that nothing is listening for.
func Subscribed[T any](b *Bus) bool {
	if b == nil {
		return false
	}

	r := b.registry
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers[reflect.TypeFor[T]()]) > 0
}

// dispatch runs a single handler, recovering from panics raised by it.
func (r *registry) dispatch(h handler, e any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logger.Error("event handler panicked",
				"plugin", h.owner,
				"event", fmt.Sprintf("%T", e),
				"err", recovered,
			)
		}
	}()
	h.call(e)
}
