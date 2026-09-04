// Package plugin holds the plugin API of Vortex.
//
// A plugin is a Go type that implements Plugin and registers itself from an
// init function. Plugins are compiled into the proxy binary: add the plugin's
// package to the imports of cmd/vortex and rebuild. Go has no reliable way of
// loading code at runtime, so this is the model Vortex uses.
//
//	package greeter
//
//	func init() { plugin.Register(&Greeter{}) }
//
//	type Greeter struct{ plugin.Base }
//
//	func (*Greeter) Manifest() plugin.Manifest {
//		return plugin.Manifest{Name: "greeter", Version: "1.0.0"}
//	}
//
//	func (g *Greeter) Load(ctx *plugin.Context) error {
//		event.Subscribe(ctx.Bus(), func(e *event.PlayerJoin) {
//			ctx.Logger().Info("welcome", "player", e.Session.Client().IdentityData().DisplayName)
//		}, event.Normal)
//		return nil
//	}
package plugin

import (
	"fmt"
	"sync"
)

// Manifest describes a plugin.
type Manifest struct {
	// Name identifies the plugin. It is used for the data directory, the
	// logger and the dependencies of other plugins, and must be unique.
	Name string
	// Version is the version of the plugin, for display purposes.
	Version string
	// Author is the author of the plugin, for display purposes.
	Author string
	// Description is a one line summary of what the plugin does.
	Description string
	// Depends holds the names of the plugins that must be loaded and enabled
	// before this one. A plugin whose dependency is missing is skipped.
	Depends []string
}

// Plugin is implemented by every Vortex plugin.
type Plugin interface {
	// Manifest returns the description of the plugin.
	Manifest() Manifest
	// Load is called before the proxy starts listening. Subscribe to events
	// and read the configuration here. Returning an error skips the plugin.
	Load(ctx *Context) error
	// Enable is called once the proxy is listening, with the API the plugin
	// may act on. Returning an error disables the plugin.
	Enable(proxy Proxy) error
	// Disable is called on shutdown, in the reverse order of enabling.
	Disable() error
}

// Base provides no-op Enable and Disable implementations, so that a plugin
// only has to implement Manifest and Load. Embed it in the plugin type.
type Base struct{}

// Enable ...
func (Base) Enable(Proxy) error { return nil }

// Disable ...
func (Base) Disable() error { return nil }

var (
	registryMu sync.Mutex
	registered []Plugin
)

// Register adds a plugin to the list loaded when the proxy starts. Call it
// from an init function of the plugin's package.
func Register(p Plugin) {
	if p == nil {
		panic("plugin: Register called with a nil plugin")
	}

	if name := p.Manifest().Name; name == "" {
		panic("plugin: Register called with a plugin without a name")
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	registered = append(registered, p)
}

// Registered returns the plugins registered so far, in registration order.
func Registered() []Plugin {
	registryMu.Lock()
	defer registryMu.Unlock()
	return append([]Plugin(nil), registered...)
}

// order sorts the plugins so that every plugin comes after the plugins it
// depends on. Plugins with a missing dependency are returned separately, and
// a dependency cycle is an error.
func order(plugins []Plugin) (sorted []Plugin, skipped []Manifest, err error) {
	byName := make(map[string]Plugin, len(plugins))
	for _, p := range plugins {
		name := p.Manifest().Name
		if _, ok := byName[name]; ok {
			return nil, nil, fmt.Errorf("duplicate plugin name %q", name)
		}
		byName[name] = p
	}

	const (
		visiting = 1
		visited  = 2
	)

	state := make(map[string]int, len(plugins))
	missing := make(map[string]struct{})

	var visit func(p Plugin, chain []string) error
	visit = func(p Plugin, chain []string) error {
		manifest := p.Manifest()
		switch state[manifest.Name] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("circular dependency: %v -> %v", chain, manifest.Name)
		}

		state[manifest.Name] = visiting
		for _, depend := range manifest.Depends {
			dependency, ok := byName[depend]
			if !ok {
				state[manifest.Name] = visited
				missing[manifest.Name] = struct{}{}
				skipped = append(skipped, manifest)
				return nil
			}

			if err := visit(dependency, append(chain, manifest.Name)); err != nil {
				return err
			}

			if _, ok := missing[depend]; ok {
				state[manifest.Name] = visited
				missing[manifest.Name] = struct{}{}
				skipped = append(skipped, manifest)
				return nil
			}
		}
		state[manifest.Name] = visited
		sorted = append(sorted, p)
		return nil
	}

	for _, p := range plugins {
		if err := visit(p, nil); err != nil {
			return nil, nil, err
		}
	}
	return sorted, skipped, nil
}
