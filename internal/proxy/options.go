package proxy

import (
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/cooldogedev/spectrum/transport"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/ybriismc/vortex/internal/config"
)

// animationFactory creates a fresh animation for a session. Camera animations
// keep per connection state, so they must never be shared between sessions.
type animationFactory func() animation.Animation

// newTransport creates the transport used to reach the downstream servers.
func newTransport(name string, logger *slog.Logger) (transport.Transport, error) {
	switch name {
	case "spectral":
		return transport.NewSpectral(logger), nil
	case "quic":
		return transport.NewQUIC(logger), nil
	default:
		return nil, fmt.Errorf("unknown transport %q", name)
	}
}

// newAnimation returns a factory for the configured transfer animation and
// whether that animation has to follow the player's camera.
func newAnimation(name string) (animationFactory, bool, error) {
	timing := protocol.CameraFadeTimeData{
		FadeInDuration:  0.75,
		WaitDuration:    3.25,
		FadeOutDuration: 0.75,
	}
	switch name {
	case "none":
		return func() animation.Animation { return animation.NopAnimation{} }, false, nil
	case "dimension":
		return func() animation.Animation { return &animation.Dimension{} }, false, nil
	case "fade":
		return func() animation.Animation {
			return &animation.Fade{
				Colour: color.RGBA{},
				Timing: protocol.CameraFadeTimeData{
					FadeInDuration:  0.25,
					WaitDuration:    4.50,
					FadeOutDuration: 0.25,
				},
			}
		}, false, nil
	case "smooth":
		return func() animation.Animation {
			return &animation.Smooth{Colour: color.RGBA{}, Timing: timing}
		}, true, nil
	case "ease":
		return func() animation.Animation {
			return &animation.Ease{Flicker: true, Colour: color.RGBA{}, Timing: timing}
		}, true, nil
	default:
		return nil, false, fmt.Errorf("unknown transfer animation %q", name)
	}
}

// resourcePacks reads the packs served by the proxy from the configured directory.
func resourcePacks(conf config.ResourcePacks) ([]*resource.Pack, error) {
	if !conf.Enabled {
		return nil, nil
	}

	if _, err := os.Stat(conf.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(conf.Directory, os.ModePerm); err != nil {
			return nil, err
		}
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(conf.Directory)
	if err != nil {
		return nil, err
	}

	packs := make([]*resource.Pack, 0, len(entries))
	for _, entry := range entries {
		pack, err := resource.ReadPath(filepath.Join(conf.Directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read resource pack %v: %w", entry.Name(), err)
		}

		if key, ok := conf.ContentKeys[pack.UUID().String()]; ok {
			pack = pack.WithContentKey(key)
		}
		packs = append(packs, pack)
	}
	return packs, nil
}
