package event

import (
	"net"

	"github.com/cooldogedev/spectrum/session"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
)

// ProxyStart is fired once, after the listener is up and every plugin is enabled.
type ProxyStart struct {
	// Addr is the address the proxy listens on.
	Addr string
}

// ProxyStop is fired once, when the proxy begins shutting down.
type ProxyStop struct{}

// PlayerLogin is fired before the login sequence starts, as soon as the client
// identifies itself. Cancelling it rejects the connection with Message, which
// makes it the right place for bans and whitelists: no session is created and
// no server is contacted.
type PlayerLogin struct {
	Cancelling

	// Addr is the address the player connects from.
	Addr net.Addr
	// Identity holds the XUID, UUID and display name of the player.
	Identity login.IdentityData
	// Client holds the data the client sent about itself. It is not trusted:
	// the player is free to change it.
	Client login.ClientData
	// Message is shown to the player when the event is cancelled.
	Message string
}

// PlayerJoin is fired after the player is connected to its first server and
// spawned in the world.
type PlayerJoin struct {
	// Session is the session of the player.
	Session *session.Session
	// Addr is the address of the server the player joined.
	Addr string
}

// PlayerQuit is fired when the player leaves the proxy.
type PlayerQuit struct {
	// Session is the session of the player.
	Session *session.Session
	// Message is the disconnection message.
	Message string
}

// ServerSelect is fired when the proxy has picked the server for a player,
// before dialing it. Handlers may rewrite Addr to send the player elsewhere.
// Cancelling it refuses the connection.
type ServerSelect struct {
	Cancelling

	// Conn is the connection of the player.
	Conn *minecraft.Conn
	// Addr is the address the player is about to be sent to.
	Addr string
	// Fallback reports whether this is a fallback selection, which happens
	// when the server the player was on died.
	Fallback bool
}

// Transfer is fired before the player is moved to another server. Cancelling
// it keeps the player where it is.
type Transfer struct {
	Cancelling

	// Session is the session of the player.
	Session *session.Session
	// Origin is the address of the server the player is leaving.
	Origin string
	// Target is the address of the server the player is going to.
	Target string
	// Fallback reports whether the move is a fallback rather than a transfer.
	Fallback bool
}

// TransferComplete is fired once the player is connected to the new server.
type TransferComplete struct {
	Session  *session.Session
	Origin   string
	Target   string
	Fallback bool
}

// TransferFailed is fired when the move to another server fails. The player
// stays on its current server.
type TransferFailed struct {
	Session  *session.Session
	Origin   string
	Target   string
	Fallback bool
	Err      error
}

// Chat is fired when the player sends a chat message. Handlers may rewrite
// Message; cancelling the event drops it before it reaches the server.
//
// Subscribing to this event makes the proxy decode the client's text packets.
type Chat struct {
	Cancelling

	// Session is the session of the player.
	Session *session.Session
	// Message is the text the player sent.
	Message string
}

// Command is fired when the player runs a command. Handlers may rewrite Line;
// cancelling the event keeps the command from reaching the server, which is
// how a plugin implements a proxy side command.
//
// Subscribing to this event makes the proxy decode the client's command
// packets. Note that the client only sends a command it knows about, so a
// command that no server registered never reaches the proxy.
type Command struct {
	Cancelling

	// Session is the session of the player.
	Session *session.Session
	// Line is the command line, without the leading slash.
	Line string
}
