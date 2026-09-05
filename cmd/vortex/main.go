// Command vortex runs the Vortex proxy.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/ybriismc/vortex/event"
	"github.com/ybriismc/vortex/internal/config"
	"github.com/ybriismc/vortex/internal/proxy"
	"github.com/ybriismc/vortex/plugin"
	// Plugins are compiled into the binary. Add the import of a plugin
	// package here to load it, for example:
	//
	//	_ "github.com/ybriismc/vortex/examples/plugins/greeter"
)

// version is set at build time with -ldflags "-X main.version=v0.0.0".
var version = "dev"

func main() {
	path := flag.String("config", "config.yml", "path of the configuration file")
	print := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *print {
		fmt.Printf("vortex %v (Minecraft %v, protocol %v)\n", version, protocol.CurrentVersion, protocol.CurrentProtocol)
		return
	}

	conf, err := config.Load(*path)
	if err != nil {
		slog.Error("failed to load the configuration", "path", *path, "err", err)
		os.Exit(1)
	}

	logger := newLogger(conf.Logging)
	bus := event.NewBus(logger)

	// Plugins are loaded before the proxy is built: their subscriptions decide
	// which packets the proxy has to decode.
	var plugins *plugin.Manager
	if conf.Plugins.Enabled {
		plugins = plugin.NewManager(bus, logger, conf.Plugins.Directory, conf.Plugins.Disabled)
		if err := plugins.Load(); err != nil {
			logger.Error("failed to load the plugins", "err", err)
			os.Exit(1)
		}
	}

	vortex, err := proxy.New(conf, logger, bus, plugins)
	if err != nil {
		logger.Error("failed to create the proxy", "err", err)
		os.Exit(1)
	}

	logger.Info("starting vortex",
		"version", version,
		"minecraft", protocol.CurrentVersion,
		"protocol", protocol.CurrentProtocol,
	)
	if err := vortex.Listen(); err != nil {
		logger.Error("failed to start the proxy", "err", err)
		os.Exit(1)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		logger.Info("shutting down")
		if err := vortex.Close(); err != nil {
			logger.Error("failed to shut down cleanly", "err", err)
		}
	}()

	if err := vortex.Accept(); err != nil {
		logger.Error("stopped accepting sessions", "err", err)
		_ = vortex.Close()
		os.Exit(1)
	}
}

// newLogger creates the logger used by the proxy and by Spectrum.
func newLogger(conf config.Logging) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(conf.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	if conf.JSON {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
