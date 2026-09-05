// Package loadtest checks that a plugin built with -buildmode=plugin is really
// loaded by the loader. It lives in its own package because a plugin cannot be
// loaded into a test binary of the package it links against: the test build of
// that package has a different hash than the one the plugin was built with.
package loadtest

import (
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ybriismc/vortex/plugin"
)

// TestOpenLoadsABuiltPlugin builds the example plugin and loads it, which is
// the whole point of the loader. It is skipped where Go cannot load plugins.
func TestOpenLoadsABuiltPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("building a plugin takes too long for -short")
	}

	if raceEnabled {
		// A plugin built without the race detector cannot be loaded into a
		// binary built with it, and vice versa.
		t.Skip("plugins and the race detector cannot be mixed")
	}

	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("Go cannot load plugins on %v", runtime.GOOS)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "greeter.so")
	build := exec.Command("go", "build", "-buildmode=plugin", "-o", path, "../../examples/plugins/greeter")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("could not build the plugin, skipping: %v: %s", err, out)
	}

	before := len(plugin.Registered())
	loader := plugin.NewLoader(slog.New(slog.NewTextHandler(io.Discard, nil)), dir)
	plugins := loader.Open()

	added := append(plugins, plugin.Registered()[before:]...)
	if len(added) != 1 {
		t.Fatalf("loading the plugin added %v plugins, expected 1", len(added))
	}

	if name := added[0].Manifest().Name; name != "greeter" {
		t.Fatalf("loaded plugin %q, expected \"greeter\"", name)
	}

}
