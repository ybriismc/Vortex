package proxy

import (
	"fmt"
	"log/slog"

	"github.com/cooldogedev/spectrum/session"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/ybriismc/vortex/plugin"
)

// Ensure that Vortex implements the API handed to the plugins.
var _ plugin.Proxy = (*Vortex)(nil)

// Logger ...
func (v *Vortex) Logger() *slog.Logger {
	return v.logger
}

// Sessions ...
func (v *Vortex) Sessions() []*session.Session {
	return v.spectrum.Registry().GetSessions()
}

// Session ...
func (v *Vortex) Session(username string) *session.Session {
	return v.spectrum.Registry().GetSessionByUsername(username)
}

// SessionByXUID ...
func (v *Vortex) SessionByXUID(xuid string) *session.Session {
	return v.spectrum.Registry().GetSession(xuid)
}

// Count ...
func (v *Vortex) Count() int {
	return len(v.spectrum.Registry().GetSessions())
}

// Transfer ...
func (v *Vortex) Transfer(username string, addr string) error {
	s := v.Session(username)
	if s == nil {
		return fmt.Errorf("player %v is not connected", username)
	}
	return s.Transfer(addr)
}

// Kick ...
func (v *Vortex) Kick(username string, reason string) error {
	s := v.Session(username)
	if s == nil {
		return fmt.Errorf("player %v is not connected", username)
	}
	s.Disconnect(reason)
	return nil
}

// Message ...
func (v *Vortex) Message(username string, message string) error {
	s := v.Session(username)
	if s == nil {
		return fmt.Errorf("player %v is not connected", username)
	}
	return sendMessage(s, message)
}

// Broadcast ...
func (v *Vortex) Broadcast(message string) {
	for _, s := range v.Sessions() {
		if err := sendMessage(s, message); err != nil {
			v.logger.Debug("failed to broadcast to session", "err", err)
		}
	}
}

// Servers ...
func (v *Vortex) Servers() []string {
	return v.discovery.Primary()
}

// SetServers ...
func (v *Vortex) SetServers(servers []string) error {
	return v.discovery.SetPrimary(servers)
}

// Fallbacks ...
func (v *Vortex) Fallbacks() []string {
	return v.discovery.Fallback()
}

// SetFallbacks ...
func (v *Vortex) SetFallbacks(servers []string) {
	v.discovery.SetFallback(servers)
}

// Plugins ...
func (v *Vortex) Plugins() []plugin.Manifest {
	if v.plugins == nil {
		return nil
	}
	return v.plugins.Plugins()
}

// sendMessage writes a chat message to the client of a session.
func sendMessage(s *session.Session, message string) error {
	if err := s.Client().WritePacket(&packet.Text{TextType: packet.TextTypeRaw, Message: message}); err != nil {
		return err
	}
	return s.Client().Flush()
}
