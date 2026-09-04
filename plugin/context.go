package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ybriismc/vortex/event"
	"gopkg.in/yaml.v3"
)

// Context is handed to a plugin when it is loaded. It gives access to the
// event bus, a logger tagged with the plugin's name and a directory the
// plugin owns.
type Context struct {
	manifest Manifest
	bus      *event.Bus
	logger   *slog.Logger
	dir      string
}

// Manifest returns the description of the plugin.
func (c *Context) Manifest() Manifest {
	return c.manifest
}

// Bus returns the event bus. Subscriptions made through it are attributed to
// the plugin, which is reported when one of its handlers panics.
func (c *Context) Bus() *event.Bus {
	return c.bus
}

// Logger returns a logger tagged with the name of the plugin.
func (c *Context) Logger() *slog.Logger {
	return c.logger
}

// Dir returns the directory of the plugin, creating it if needed. Use it for
// the files the plugin owns, such as its configuration or its data.
func (c *Context) Dir() (string, error) {
	if err := os.MkdirAll(c.dir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create the plugin directory: %w", err)
	}
	return c.dir, nil
}

// Config reads config.yml from the plugin's directory into v. When the file
// does not exist yet, v is written to it unchanged, so passing a struct
// holding the defaults both seeds the file and leaves v usable.
func (c *Context) Config(v any) error {
	dir, err := c.Dir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "config.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c.SaveConfig(v)
	} else if err != nil {
		return fmt.Errorf("failed to read %v: %w", path, err)
	}

	if err := yaml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to decode %v: %w", path, err)
	}
	return nil
}

// SaveConfig writes v to config.yml in the plugin's directory.
func (c *Context) SaveConfig(v any) error {
	dir, err := c.Dir()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yml"), data, 0o644)
}
