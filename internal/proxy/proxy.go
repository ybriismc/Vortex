// Package proxy wires Spectrum together with the Vortex configuration,
// discovery, packet guard and API service.
package proxy

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/session"
	"github.com/cooldogedev/spectrum/util"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/ybriismc/vortex/internal/config"
	"github.com/ybriismc/vortex/internal/discovery"
	"github.com/ybriismc/vortex/internal/guard"
)

// Vortex is a Spectrum proxy driven by a configuration file.
type Vortex struct {
	conf   *config.Config
	logger *slog.Logger

	spectrum  *spectrum.Spectrum
	discovery *discovery.Balanced
	api       *api.API

	animation    animationFactory
	guardOpts    guard.Options
	loginTimeout time.Duration
	closed       atomic.Bool
}

// New creates a Vortex proxy from the given configuration. The listener is not
// started yet, Listen must be called for that.
func New(conf *config.Config, logger *slog.Logger) (*Vortex, error) {
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

	blocked := make(map[uint32]struct{}, len(conf.Security.BlockedPackets))
	for _, id := range conf.Security.BlockedPackets {
		blocked[id] = struct{}{}
	}

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

		spectrum:  spectrum.NewSpectrum(balancer, logger, opts, transport),
		discovery: balancer,

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
		"transport", v.conf.Proxy.Transport,
		"xbox_authentication", v.conf.Proxy.XboxAuthentication,
		"balancer", v.conf.Servers.Balancer,
		"servers", len(v.conf.Servers.Primary),
		"packs", len(packs),
	)
	if !v.conf.API.Enabled {
		return nil
	}
	return v.listenAPI()
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
	s.SetProcessor(guard.New(s, logger, v.guardOpts))
	s.SetAnimation(v.animation())
	go func() {
		if err := s.LoginTimeout(v.loginTimeout); err != nil {
			s.Disconnect(err.Error())
			if !errors.Is(err, context.Canceled) {
				logger.Error("failed to login session", "err", err)
			}
		}
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
