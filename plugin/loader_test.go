package plugin

import (
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainPointsAtTheFix(t *testing.T) {
	cases := []struct {
		err      string
		contains string
	}{
		{"plugin: not implemented", "cgo"},
		{"plugin.Open: plugin was built with a different version of package internal/abi", "rebuild the plugin"},
		{"plugin.Open: bad magic number", "architecture"},
		{"plugin.Open: something else entirely", "something else entirely"},
	}

	for _, c := range cases {
		explained := explain(errors.New(c.err)).Error()
		if !strings.Contains(explained, c.contains) {
			t.Errorf("explaining %q gave %q, expected it to mention %q", c.err, explained, c.contains)
		}

		if !strings.Contains(explained, c.err) {
			t.Errorf("explaining %q dropped the original error: %q", c.err, explained)
		}
	}
}

func TestOpenSkipsFilesThatAreNotPlugins(t *testing.T) {
	dir := t.TempDir()
	loader := NewLoader(slog.New(slog.NewTextHandler(io.Discard, nil)), dir)

	// A directory, a file with another extension and a file that is not a
	// plugin at all: none of them may stop the loader.
	if err := exec.Command("mkdir", filepath.Join(dir, "greeter")).Run(); err != nil {
		t.Fatalf("failed to create the directory: %v", err)
	}

	writeFile(t, filepath.Join(dir, "config.yml"), "welcome: hello")
	writeFile(t, filepath.Join(dir, "broken.so"), "this is not a plugin")

	if plugins := loader.Open(); len(plugins) != 0 {
		t.Fatalf("opened %v plugins, expected none", len(plugins))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := exec.Command("sh", "-c", "printf %s "+shellQuote(content)+" > "+shellQuote(path)).Run(); err != nil {
		t.Fatalf("failed to write %v: %v", path, err)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
