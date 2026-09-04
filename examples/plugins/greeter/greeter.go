// Package greeter is an example Vortex plugin. It welcomes players, announces
// who leaves and adds a proxy side /players command.
//
// To load it, import the package from cmd/vortex and rebuild the proxy:
//
//	_ "github.com/ybriismc/vortex/examples/plugins/greeter"
package greeter

import (
	"fmt"
	"strings"

	"github.com/ybriismc/vortex/event"
	"github.com/ybriismc/vortex/plugin"
)

func init() {
	plugin.Register(&Greeter{})
}

// Config holds the messages of the plugin, read from plugins/greeter/config.yml.
type Config struct {
	Welcome string `yaml:"welcome"`
	Goodbye string `yaml:"goodbye"`
}

// Greeter greets players as they join.
type Greeter struct {
	plugin.Base

	conf  Config
	proxy plugin.Proxy
}

// Manifest ...
func (*Greeter) Manifest() plugin.Manifest {
	return plugin.Manifest{
		Name:        "greeter",
		Version:     "1.0.0",
		Author:      "yBriisMC",
		Description: "Welcomes players and adds a /players command.",
	}
}

// Load subscribes to the events the plugin needs. It runs before the proxy
// starts listening.
func (g *Greeter) Load(ctx *plugin.Context) error {
	g.conf = Config{
		Welcome: "§eWelcome, %v!",
		Goodbye: "§7%v left the network.",
	}
	if err := ctx.Config(&g.conf); err != nil {
		return err
	}

	event.Subscribe(ctx.Bus(), func(e *event.PlayerJoin) {
		username := e.Session.Client().IdentityData().DisplayName
		ctx.Logger().Info("player joined", "username", username, "server", e.Addr)
		g.proxy.Broadcast(fmt.Sprintf(g.conf.Welcome, username))
	}, event.Normal)

	event.Subscribe(ctx.Bus(), func(e *event.PlayerQuit) {
		g.proxy.Broadcast(fmt.Sprintf(g.conf.Goodbye, e.Session.Client().IdentityData().DisplayName))
	}, event.Normal)

	// Cancelling the command event keeps it from reaching the server, which is
	// how a plugin implements a command of its own.
	event.Subscribe(ctx.Bus(), func(e *event.Command) {
		if !strings.EqualFold(e.Line, "players") {
			return
		}

		e.Cancel()
		username := e.Session.Client().IdentityData().DisplayName
		if err := g.proxy.Message(username, fmt.Sprintf("§a%v players online.", g.proxy.Count())); err != nil {
			ctx.Logger().Error("failed to answer the command", "username", username, "err", err)
		}
	}, event.Normal)
	return nil
}

// Enable stores the proxy API the plugin acts on. It runs once the proxy is
// listening.
func (g *Greeter) Enable(proxy plugin.Proxy) error {
	g.proxy = proxy
	return nil
}
