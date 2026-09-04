// Package guard implements Spectrum's session.Processor interface. It filters
// the traffic of a session before it reaches the downstream server, which is
// where anti-cheat and abuse protection belong in a Spectrum based proxy.
package guard

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/ybriismc/vortex/event"
)

// Options holds the rules applied by a Guard.
type Options struct {
	// RateLimit enables the packets per second limit.
	RateLimit bool
	// PacketsPerSecond is the amount of packets a session may send per second.
	PacketsPerSecond int
	// Kick disconnects the player instead of dropping its packets when the limit is exceeded.
	Kick bool
	// KickMessage is shown to the player when it is disconnected by the limiter.
	KickMessage string
	// Blocked holds the client packet identifiers that are never forwarded.
	Blocked map[uint32]struct{}
	// MaxPacketSize is the maximum size in bytes of a client packet. 0 disables the check.
	MaxPacketSize int
	// TrackCamera keeps the camera animations aware of the player's position.
	TrackCamera bool
}

// Guard filters the packets of a single session.
type Guard struct {
	session.NopProcessor

	sess   *session.Session
	logger *slog.Logger
	bus    *event.Bus
	opts   Options

	limiter limiter
	dropped atomic.Int64
	once    sync.Once
}

// Ensure that Guard satisfies the session.Processor interface.
var _ session.Processor = (*Guard)(nil)

// New creates a Guard for the given session. Events are fired on the given
// bus, which may be nil when no plugin is loaded.
func New(sess *session.Session, logger *slog.Logger, bus *event.Bus, opts Options) *Guard {
	return &Guard{sess: sess, logger: logger, bus: bus, opts: opts}
}

// Dropped returns the amount of client packets dropped by the guard.
func (g *Guard) Dropped() int64 {
	return g.dropped.Load()
}

// ProcessClientEncoded filters the packets that are forwarded without being decoded.
func (g *Guard) ProcessClientEncoded(ctx *session.Context, payload *[]byte) {
	data := *payload
	if g.opts.MaxPacketSize > 0 && len(data) > g.opts.MaxPacketSize {
		g.drop(ctx, "oversized packet", slog.Int("size", len(data)))
		return
	}

	if len(g.opts.Blocked) > 0 {
		if id, ok := packetID(data); ok {
			if _, blocked := g.opts.Blocked[id]; blocked {
				g.drop(ctx, "blocked packet", slog.Uint64("id", uint64(id)))
				return
			}
		}
	}
	g.limit(ctx)
}

// ProcessClient filters the packets that are decoded by the proxy.
func (g *Guard) ProcessClient(ctx *session.Context, pk *packet.Packet) {
	if _, blocked := g.opts.Blocked[(*pk).ID()]; blocked {
		g.drop(ctx, "blocked packet", slog.Uint64("id", uint64((*pk).ID())))
		return
	}

	if g.limit(ctx); ctx.Cancelled() {
		return
	}

	switch decoded := (*pk).(type) {
	case *packet.PlayerAuthInput:
		if g.opts.TrackCamera {
			g.trackCamera(decoded)
		}
	case *packet.Text:
		g.chat(ctx, decoded)
	case *packet.CommandRequest:
		g.command(ctx, decoded)
	}
}

// chat fires the chat event, letting plugins rewrite or drop the message.
func (g *Guard) chat(ctx *session.Context, pk *packet.Text) {
	if pk.TextType != packet.TextTypeChat || !event.Subscribed[event.Chat](g.bus) {
		return
	}

	e := event.Call(g.bus, &event.Chat{Session: g.sess, Message: pk.Message})
	if e.Cancelled() {
		ctx.Cancel()
		return
	}
	pk.Message = e.Message
}

// command fires the command event, which is how a plugin implements a proxy
// side command: cancelling the event keeps the command from reaching the
// server.
func (g *Guard) command(ctx *session.Context, pk *packet.CommandRequest) {
	if !event.Subscribed[event.Command](g.bus) {
		return
	}

	e := event.Call(g.bus, &event.Command{Session: g.sess, Line: strings.TrimPrefix(pk.CommandLine, "/")})
	if e.Cancelled() {
		ctx.Cancel()
		return
	}
	pk.CommandLine = "/" + strings.TrimPrefix(e.Line, "/")
}

// ProcessStartGame ...
func (g *Guard) ProcessStartGame(_ *session.Context, data *minecraft.GameData) {
	g.logger.Debug("started game", "entity", data.EntityRuntimeID)
}

// ProcessPreTransfer ...
func (g *Guard) ProcessPreTransfer(ctx *session.Context, origin *string, target *string) {
	g.logger.Info("transferring session", "origin", *origin, "target", *target)
	e := event.Call(g.bus, &event.Transfer{Session: g.sess, Origin: *origin, Target: *target})
	if e.Cancelled() {
		ctx.Cancel()
		return
	}
	*target = e.Target
}

// ProcessPostTransfer ...
func (g *Guard) ProcessPostTransfer(_ *session.Context, origin *string, target *string) {
	event.Call(g.bus, &event.TransferComplete{Session: g.sess, Origin: *origin, Target: *target})
}

// ProcessTransferFailure ...
func (g *Guard) ProcessTransferFailure(_ *session.Context, origin *string, target *string, err error) {
	g.logger.Warn("failed to transfer session", "origin", *origin, "target", *target, "err", err)
	event.Call(g.bus, &event.TransferFailed{Session: g.sess, Origin: *origin, Target: *target, Err: err})
}

// ProcessPreFallback ...
func (g *Guard) ProcessPreFallback(ctx *session.Context, origin *string, target *string) {
	g.logger.Warn("falling back session", "origin", *origin, "target", *target)
	e := event.Call(g.bus, &event.Transfer{Session: g.sess, Origin: *origin, Target: *target, Fallback: true})
	if e.Cancelled() {
		ctx.Cancel()
		return
	}
	*target = e.Target
}

// ProcessPostFallback ...
func (g *Guard) ProcessPostFallback(_ *session.Context, origin *string, target *string) {
	event.Call(g.bus, &event.TransferComplete{Session: g.sess, Origin: *origin, Target: *target, Fallback: true})
}

// ProcessFallbackFailure ...
func (g *Guard) ProcessFallbackFailure(_ *session.Context, origin *string, target *string, err error) {
	g.logger.Error("failed to fall back session", "origin", *origin, "target", *target, "err", err)
	event.Call(g.bus, &event.TransferFailed{Session: g.sess, Origin: *origin, Target: *target, Fallback: true, Err: err})
}

// ProcessDisconnection ...
func (g *Guard) ProcessDisconnection(_ *session.Context, message *string) {
	g.logger.Info("disconnected session", "reason", *message, "dropped", g.dropped.Load())
	event.Call(g.bus, &event.PlayerQuit{Session: g.sess, Message: *message})
}

// limit cancels the context when the session exceeds its packets per second budget.
func (g *Guard) limit(ctx *session.Context) {
	if !g.opts.RateLimit || g.limiter.allow(g.opts.PacketsPerSecond) {
		return
	}

	if g.opts.Kick {
		g.once.Do(func() {
			g.logger.Warn("kicked session for exceeding the packet rate limit", "limit", g.opts.PacketsPerSecond)
			go g.sess.Disconnect(g.opts.KickMessage)
		})
	}
	g.drop(ctx, "rate limit exceeded", slog.Int("limit", g.opts.PacketsPerSecond))
}

// drop cancels the context, keeping the packet from reaching the server.
func (g *Guard) drop(ctx *session.Context, reason string, attrs ...any) {
	ctx.Cancel()
	if g.dropped.Add(1) <= 5 {
		g.logger.Debug("dropped client packet", append([]any{"reason", reason}, attrs...)...)
	}
}

// trackCamera feeds the player's position to the camera based transfer animations,
// which otherwise place the camera at the origin of the world.
func (g *Guard) trackCamera(pk *packet.PlayerAuthInput) {
	switch current := g.sess.Animation().(type) {
	case *animation.Smooth:
		current.Position = pk.Position.Add(mgl32.Vec3{0, 1, 0})
		current.Yaw = pk.HeadYaw
	case *animation.Ease:
		current.Position = pk.Position.Add(mgl32.Vec3{0, 1, 0})
		current.Yaw = pk.HeadYaw
	}
}

// packetID reads the identifier of an encoded client packet without decoding its payload.
func packetID(payload []byte) (uint32, bool) {
	header := &packet.Header{}
	if err := header.Read(bytes.NewReader(payload)); err != nil {
		return 0, false
	}
	return header.PacketID, true
}

// limiter counts the packets sent within the current second.
type limiter struct {
	mu     sync.Mutex
	window int64
	count  int
}

// allow reports whether another packet fits within the given budget.
func (l *limiter) allow(max int) bool {
	now := time.Now().Unix()
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.window != now {
		l.window = now
		l.count = 0
	}
	l.count++
	return l.count <= max
}
