package plugin

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ybriismc/vortex/event"
)

// Manager loads, enables and disables the registered plugins.
type Manager struct {
	bus      *event.Bus
	logger   *slog.Logger
	dir      string
	disabled []string

	loaded  []Plugin
	enabled []Plugin
}

// NewManager creates a Manager storing plugin directories under dir. Plugins
// whose name appears in disabled are skipped entirely.
func NewManager(bus *event.Bus, logger *slog.Logger, dir string, disabled []string) *Manager {
	return &Manager{bus: bus, logger: logger, dir: dir, disabled: disabled}
}

// Load loads every registered plugin, in dependency order. A plugin that
// fails to load, or whose dependency is missing, is skipped rather than
// stopping the proxy.
func (m *Manager) Load() error {
	sorted, skipped, err := order(Registered())
	if err != nil {
		return err
	}

	for _, manifest := range skipped {
		m.logger.Warn("skipped plugin with a missing dependency", "plugin", manifest.Name, "depends", manifest.Depends)
	}

	for _, p := range sorted {
		manifest := p.Manifest()
		if m.isDisabled(manifest.Name) {
			m.logger.Info("skipped disabled plugin", "plugin", manifest.Name)
			continue
		}

		ctx := &Context{
			manifest: manifest,
			bus:      m.bus.For(manifest.Name),
			logger:   m.logger.With("plugin", manifest.Name),
			dir:      filepath.Join(m.dir, manifest.Name),
		}
		if err := p.Load(ctx); err != nil {
			m.logger.Error("failed to load plugin", "plugin", manifest.Name, "err", err)
			continue
		}

		m.loaded = append(m.loaded, p)
		m.logger.Info("loaded plugin", "plugin", manifest.Name, "version", manifest.Version)
	}
	return nil
}

// Enable enables every loaded plugin, handing it the proxy API.
func (m *Manager) Enable(proxy Proxy) {
	for _, p := range m.loaded {
		manifest := p.Manifest()
		if err := p.Enable(proxy); err != nil {
			m.logger.Error("failed to enable plugin", "plugin", manifest.Name, "err", err)
			continue
		}
		m.enabled = append(m.enabled, p)
	}

	if len(m.enabled) > 0 {
		m.logger.Info("enabled plugins", "count", len(m.enabled), "plugins", m.names())
	}
}

// Disable disables the enabled plugins in the reverse order of enabling, so
// that a plugin is never disabled before the plugins depending on it.
func (m *Manager) Disable() {
	for i := len(m.enabled) - 1; i >= 0; i-- {
		p := m.enabled[i]
		if err := p.Disable(); err != nil {
			m.logger.Error("failed to disable plugin", "plugin", p.Manifest().Name, "err", err)
		}
	}
	m.enabled = nil
}

// Plugins returns the manifests of the enabled plugins.
func (m *Manager) Plugins() []Manifest {
	manifests := make([]Manifest, 0, len(m.enabled))
	for _, p := range m.enabled {
		manifests = append(manifests, p.Manifest())
	}
	return manifests
}

// isDisabled reports whether the plugin was disabled in the configuration.
func (m *Manager) isDisabled(name string) bool {
	return slices.ContainsFunc(m.disabled, func(disabled string) bool {
		return strings.EqualFold(disabled, name)
	})
}

// names returns the names of the enabled plugins.
func (m *Manager) names() string {
	names := make([]string, 0, len(m.enabled))
	for _, p := range m.enabled {
		names = append(names, fmt.Sprintf("%v %v", p.Manifest().Name, p.Manifest().Version))
	}
	return strings.Join(names, ", ")
}
