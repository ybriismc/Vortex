package plugin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goplugin "plugin"
	"runtime"
	"strings"
)

// Ext is the extension of a compiled plugin file.
const Ext = ".so"

// Loader opens compiled Go plugins from a directory, so that a plugin can be
// added to the proxy without rebuilding it.
//
// A plugin is a main package built with "go build -buildmode=plugin". It joins
// the proxy either by calling Register from an init function, or by exporting a
// "Plugin" variable or a "New" function:
//
//	package main
//
//	func init() { plugin.Register(&Bans{}) }
//
//	// Building it:
//	//   go build -buildmode=plugin -o plugins/bans.so ./bans
//
//	func main() {} // never called, keeps "go build ./..." happy
//
// Go loads compiled plugins under strict conditions: the proxy must be built
// with cgo enabled, the platform must be Linux or macOS, and the plugin must be
// built with the same Go toolchain and the same versions of every package it
// shares with the proxy. Mismatches are reported with an explanation instead of
// the raw runtime error.
type Loader struct {
	logger *slog.Logger
	dir    string
}

// NewLoader creates a Loader reading plugins from dir.
func NewLoader(logger *slog.Logger, dir string) *Loader {
	return &Loader{logger: logger, dir: dir}
}

// Open opens every plugin file in the directory and returns the plugins they
// contributed through an exported symbol. Plugins that register themselves from
// an init function end up in Registered instead. A file that fails to open is
// logged and skipped, so one broken plugin does not stop the proxy.
func (l *Loader) Open() []Plugin {
	if err := os.MkdirAll(l.dir, os.ModePerm); err != nil {
		l.logger.Error("failed to create the plugin directory", "dir", l.dir, "err", err)
		return nil
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		l.logger.Error("failed to read the plugin directory", "dir", l.dir, "err", err)
		return nil
	}

	var plugins []Plugin
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), Ext) {
			continue
		}

		path := filepath.Join(l.dir, entry.Name())
		p, err := l.open(path)
		if err != nil {
			l.logger.Error("failed to open plugin", "file", entry.Name(), "err", err)
			continue
		}

		if p != nil {
			plugins = append(plugins, p)
		}
		l.logger.Debug("opened plugin file", "file", entry.Name())
	}
	return plugins
}

// open loads a single plugin file. It returns the plugin exported by the file,
// or nil when the file registered itself from an init function.
func (l *Loader) open(path string) (Plugin, error) {
	before := len(Registered())
	lib, err := goplugin.Open(path)
	if err != nil {
		return nil, explain(err)
	}

	if len(Registered()) > before {
		// The file called Register from an init function, which is the way a
		// plugin is expected to announce itself.
		return nil, nil
	}
	return symbol(lib)
}

// symbol pulls a plugin out of a file that did not register itself, looking for
// a "New" function or a "Plugin" variable.
func symbol(lib *goplugin.Plugin) (Plugin, error) {
	if sym, err := lib.Lookup("New"); err == nil {
		factory, ok := sym.(func() Plugin)
		if !ok {
			return nil, fmt.Errorf(`the "New" symbol is %T, expected func() plugin.Plugin`, sym)
		}

		p := factory()
		if p == nil {
			return nil, errors.New(`the "New" function returned a nil plugin`)
		}
		return p, nil
	}

	sym, err := lib.Lookup("Plugin")
	if err != nil {
		return nil, errors.New(`the file registered no plugin: call plugin.Register from an init function, or export a "Plugin" variable or a "New" function`)
	}

	switch p := sym.(type) {
	case Plugin:
		return p, nil
	case *Plugin:
		if *p == nil {
			return nil, errors.New(`the "Plugin" variable is nil`)
		}
		return *p, nil
	default:
		return nil, fmt.Errorf(`the "Plugin" symbol is %T, which does not implement plugin.Plugin`, sym)
	}
}

// explain turns the runtime's plugin errors into something an operator can act
// on. The raw messages are accurate but say nothing about the fix.
func explain(err error) error {
	message := err.Error()
	switch {
	case strings.Contains(message, "not implemented"):
		if runtime.GOOS == "windows" {
			return fmt.Errorf("%w (Go cannot load plugins on Windows: run the proxy on Linux, or compile the plugin into the binary)", err)
		}
		return fmt.Errorf("%w (the proxy was built without cgo: rebuild it with CGO_ENABLED=1)", err)
	case strings.Contains(message, "different version of package"),
		strings.Contains(message, "plugin was built with a different version"):
		return fmt.Errorf("%w (rebuild the plugin against this version of Vortex, with the same Go toolchain)", err)
	case strings.Contains(message, "wrong ELF class"),
		strings.Contains(message, "invalid ELF header"),
		strings.Contains(message, "bad magic number"):
		return fmt.Errorf("%w (the plugin was built for another operating system or architecture, this proxy runs on %v/%v)", err, runtime.GOOS, runtime.GOARCH)
	default:
		return err
	}
}
