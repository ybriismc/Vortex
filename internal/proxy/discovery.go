package proxy

import (
	"errors"
	"sync"

	"github.com/cooldogedev/spectrum/server"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/ybriismc/vortex/event"
)

// eventDiscovery wraps a server.Discovery and gives plugins a say in which
// server a player is sent to.
type eventDiscovery struct {
	inner server.Discovery
	bus   *event.Bus

	// picked remembers the address chosen for a connection, so that the join
	// event can report the server the player landed on.
	picked sync.Map
}

// Discover ...
func (d *eventDiscovery) Discover(conn *minecraft.Conn) (string, error) {
	addr, err := d.inner.Discover(conn)
	if err != nil {
		return "", err
	}

	addr, err = d.fire(conn, addr, false)
	if err == nil {
		d.picked.Store(conn, addr)
	}
	return addr, err
}

// DiscoverFallback ...
func (d *eventDiscovery) DiscoverFallback(conn *minecraft.Conn) (string, error) {
	addr, err := d.inner.DiscoverFallback(conn)
	if err != nil {
		return "", err
	}
	return d.fire(conn, addr, true)
}

// fire lets the plugins rewrite or refuse the picked address.
func (d *eventDiscovery) fire(conn *minecraft.Conn, addr string, fallback bool) (string, error) {
	if !event.Subscribed[event.ServerSelect](d.bus) {
		return addr, nil
	}

	e := event.Call(d.bus, &event.ServerSelect{Conn: conn, Addr: addr, Fallback: fallback})
	if e.Cancelled() {
		return "", errors.New("server selection cancelled by a plugin")
	}
	return e.Addr, nil
}

// take returns the address picked for the connection and forgets it.
func (d *eventDiscovery) take(conn *minecraft.Conn) string {
	if addr, ok := d.picked.LoadAndDelete(conn); ok {
		return addr.(string)
	}
	return ""
}
