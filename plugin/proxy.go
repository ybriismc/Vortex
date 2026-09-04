package plugin

import (
	"log/slog"

	"github.com/cooldogedev/spectrum/session"
)

// Proxy is the API a plugin acts on. It is handed to the plugin when it is
// enabled, and is implemented by the running proxy.
type Proxy interface {
	// Logger returns the logger of the proxy.
	Logger() *slog.Logger

	// Sessions returns every session currently connected to the proxy.
	Sessions() []*session.Session
	// Session returns the session of the player with the given username, or
	// nil when that player is not connected. The lookup is case insensitive.
	Session(username string) *session.Session
	// SessionByXUID returns the session of the player with the given XUID, or
	// nil when that player is not connected.
	SessionByXUID(xuid string) *session.Session
	// Count returns the amount of connected players.
	Count() int

	// Transfer moves a player to the given server address.
	Transfer(username string, addr string) error
	// Kick disconnects a player with the given message.
	Kick(username string, reason string) error
	// Message sends a chat message to a single player.
	Message(username string, message string) error
	// Broadcast sends a chat message to every connected player.
	Broadcast(message string)

	// Servers returns the pool of primary servers.
	Servers() []string
	// SetServers replaces the pool of primary servers. Players already
	// connected stay where they are.
	SetServers(servers []string) error
	// Fallbacks returns the pool of fallback servers.
	Fallbacks() []string
	// SetFallbacks replaces the pool of fallback servers.
	SetFallbacks(servers []string)

	// Plugins returns the manifests of the enabled plugins.
	Plugins() []Manifest
}
