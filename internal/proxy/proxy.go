// Package proxy wires Spectrum together with the Vortex configuration,
// discovery, packet guard and API service.
package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"slices"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/ybriismc/vortex/event"
	"github.com/ybriismc/vortex/internal/config"
	"github.com/ybriismc/vortex/internal/discovery"
	"github.com/ybriismc/vortex/internal/guard"
	"github.com/ybriismc/vortex/plugin"
)

// Vortex is a Spectrum proxy driven by a configuration file.
type Vortex struct {
	conf   *config.Config
	logger *slog.Logger

	spectrum  *spectrum.Spectrum
	discovery *discovery.Balanced
	api       *api.API

	selector *eventDiscovery
	bus      *event.Bus
	plugins  *plugin.Manager

	animation    animationFactory
	guardOpts    guard.Options
	loginTimeout time.Duration
	closed       atomic.Bool
}

// New creates a Vortex proxy from the given configuration. Plugins must have
// been loaded on the given bus before this call, so that the proxy knows which
// packets their events need. The listener is not started yet, Listen must be
// called for that.
func New(conf *config.Config, logger *slog.Logger, bus *event.Bus, plugins *plugin.Manager) (*Vortex, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}

	balancer, err := discovery.NewBalanced(conf.Servers.Balancer, conf.Servers.Primary, conf.Servers.Fallback)
	if err != nil {
		return nil, err
	}

	transport, err := newTransport(conf.Proxy.Transport, logger)
	if err != nil {
		return nil, err
	}

	factory, camera, err := newAnimation(conf.Proxy.TransferAnimation)
	if err != nil {
		return nil, err
	}

	// The camera animations follow the player, which requires the proxy to decode
	// the input packet. Every other packet stays untouched.
	decode := slices.Clone(conf.Security.DecodePackets)
	if camera && !slices.Contains(decode, packet.IDPlayerAuthInput) {
		decode = append(decode, packet.IDPlayerAuthInput)
	}

	// Plugins only pay for the packets their events actually need.
	if event.Subscribed[event.Chat](bus) && !slices.Contains(decode, packet.IDText) {
		decode = append(decode, packet.IDText)
	}

	if event.Subscribed[event.Command](bus) && !slices.Contains(decode, packet.IDCommandRequest) {
		decode = append(decode, packet.IDCommandRequest)
	}

	blocked := make(map[uint32]struct{}, len(conf.Security.BlockedPackets))
	for _, id := range conf.Security.BlockedPackets {
		blocked[id] = struct{}{}
	}

	selector := &eventDiscovery{inner: balancer, bus: bus}
	opts := &util.Opts{
		Addr: conf.Proxy.Addr,
		// Sessions are logged in by Vortex itself, once the guard and the animation
		// are attached to them.
		AutoLogin:       false,
		ClientDecode:    decode,
		LatencyInterval: conf.Proxy.LatencyInterval,
		ShutdownMessage: conf.Proxy.ShutdownMessage,
		SyncProtocol:    conf.Proxy.SyncProtocol,
	}
	return &Vortex{
		conf:   conf,
		logger: logger,

		spectrum:  spectrum.NewSpectrum(selector, logger, opts, transport),
		discovery: balancer,
		selector:  selector,

		bus:     bus,
		plugins: plugins,

		animation: factory,
		guardOpts: guard.Options{
			RateLimit:        conf.Security.RateLimit.Enabled,
			PacketsPerSecond: conf.Security.RateLimit.PacketsPerSecond,
			Kick:             conf.Security.RateLimit.Action == "kick",
			KickMessage:      conf.Security.RateLimit.KickMessage,
			Blocked:          blocked,
			MaxPacketSize:    conf.Security.MaxPacketSize,
			TrackCamera:      camera,
		},
		loginTimeout: time.Duration(conf.Proxy.LoginTimeout) * time.Second,
	}, nil
}

// Listen starts the RakNet listener and, when enabled, the API service.
func (v *Vortex) Listen() error {
	packs, err := resourcePacks(v.conf.ResourcePacks)
	if err != nil {
		return err
	}

	listenConfig := minecraft.ListenConfig{
		Allow:                  v.allow,
		ErrorLog:               v.logger,
		AuthenticationDisabled: !v.conf.Proxy.XboxAuthentication,
		MaximumPlayers:         v.conf.Proxy.MaxPlayers,
		StatusProvider:         util.NewStatusProvider(v.conf.Proxy.Name, v.conf.Proxy.SubName),
		ResourcePacks:          packs,
		TexturePacksRequired:   v.conf.ResourcePacks.Enabled && v.conf.ResourcePacks.Required,
	}
	if err := v.spectrum.Listen(listenConfig); err != nil {
		return err
	}

	v.logger.Info("vortex is ready",
		"protocol", protocol.CurrentProtocol,
		"transport", v.conf.Proxy.Transport,
		"xbox_authentication", v.conf.Proxy.XboxAuthentication,
		"balancer", v.conf.Servers.Balancer,
		"servers", len(v.conf.Servers.Primary),
		"packs", len(packs),
	)
	if v.conf.API.Enabled {
		if err := v.listenAPI(); err != nil {
			return err
		}
	}

	if v.plugins != nil {
		v.plugins.Enable(v)
	}
	event.Call(v.bus, &event.ProxyStart{Addr: v.conf.Proxy.Addr})
	return nil
}

// allow answers the listener whether a connecting player may log in. It fires
// the login event, which is where bans and whitelists belong: no session is
// created and no server is contacted for a player rejected here.
func (v *Vortex) allow(addr net.Addr, identity login.IdentityData, client login.ClientData) (string, bool) {
	if !event.Subscribed[event.PlayerLogin](v.bus) {
		return "", true
	}

	e := event.Call(v.bus, &event.PlayerLogin{Addr: addr, Identity: identity, Client: client})
	if e.Cancelled() {
		v.logger.Info("refused login", "username", identity.DisplayName, "addr", addr, "reason", e.Message)
		return e.Message, false
	}
	return "", true
}

// Accept accepts sessions until the proxy is closed.
func (v *Vortex) Accept() error {
	for {
		s, err := v.spectrum.Accept()
		if err != nil {
			if v.closed.Load() {
				return nil
			}
			return err
		}
		v.handle(s)
	}
}

// Discovery returns the discovery holding the pools of servers.
func (v *Vortex) Discovery() *discovery.Balanced {
	return v.discovery
}

// Registry returns the registry holding the active sessions.
func (v *Vortex) Registry() *session.Registry {
	return v.spectrum.Registry()
}

// Close disconnects every session and stops the listeners.
func (v *Vortex) Close() error {
	if !v.closed.CompareAndSwap(false, true) {
		return nil
	}

	event.Call(v.bus, &event.ProxyStop{})
	if v.plugins != nil {
		v.plugins.Disable()
	}

	if v.api != nil {
		_ = v.api.Close()
	}

	if v.spectrum.Listener() == nil {
		return nil
	}
	return v.spectrum.Close()
}

// handle prepares an accepted session and starts its login sequence.
func (v *Vortex) handle(s *session.Session) {
	logger := v.logger.With("username", s.Client().IdentityData().DisplayName)
	s.SetProcessor(guard.New(s, logger, v.bus, v.guardOpts))
	s.SetAnimation(v.animation())
	go func() {
		defer v.selector.take(s.Client())
		if err := s.LoginTimeout(v.loginTimeout); err != nil {
			s.Disconnect(err.Error())
			if !errors.Is(err, context.Canceled) {
				logger.Error("failed to login session", "err", err)
			}
			return
		}
		event.Call(v.bus, &event.PlayerJoin{Session: s, Addr: v.selector.take(s.Client())})
	}()
}

// listenAPI starts the TCP service downstream servers use to transfer and kick players.
func (v *Vortex) listenAPI() error {
	var authentication api.Authentication
	if v.conf.API.Secret != "" {
		authentication = api.NewSecretBasedAuthentication(v.conf.API.Secret)
	} else {
		v.logger.Warn("api secret is empty, the api accepts unauthenticated servers")
	}

	service := api.NewAPI(v.spectrum.Registry(), v.logger, authentication)
	if err := service.Listen(v.conf.API.Addr); err != nil {
		return err
	}

	v.api = service
	go func() {
		for {
			if err := service.Accept(); err != nil {
				if !v.closed.Load() {
					v.logger.Error("api stopped accepting connections", "err", err)
				}
				return
			}
		}
	}()
	return nil
}
