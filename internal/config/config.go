// Package config holds the on-disk configuration of the Vortex proxy.
package config

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration of the proxy.
type Config struct {
	Proxy         Proxy         `yaml:"proxy"`
	Servers       Servers       `yaml:"servers"`
	API           API           `yaml:"api"`
	Security      Security      `yaml:"security"`
	Plugins       Plugins       `yaml:"plugins"`
	ResourcePacks ResourcePacks `yaml:"resource_packs"`
	Logging       Logging       `yaml:"logging"`
}

// Proxy holds the options of the listener exposed to the players.
type Proxy struct {
	// Addr is the UDP address the RakNet listener binds to.
	Addr string `yaml:"addr"`
	// Name and SubName are shown in the server list of the client.
	Name    string `yaml:"name"`
	SubName string `yaml:"sub_name"`
	// Transport is the protocol used to talk to the downstream servers: "spectral" or "quic".
	Transport string `yaml:"transport"`
	// XboxAuthentication requires the players to be authenticated with their Xbox Live account.
	XboxAuthentication bool `yaml:"xbox_authentication"`
	// MaxPlayers is the amount of players accepted by the proxy. 0 means unlimited.
	MaxPlayers int `yaml:"max_players"`
	// LatencyInterval is the interval in milliseconds at which the latency is reported to the server.
	LatencyInterval int64 `yaml:"latency_interval"`
	// LoginTimeout is the timeout in seconds of the whole login sequence.
	LoginTimeout int `yaml:"login_timeout"`
	// ShutdownMessage is shown to the players when the proxy shuts down.
	ShutdownMessage string `yaml:"shutdown_message"`
	// SyncProtocol makes the proxy speak the client's protocol version to the servers instead
	// of the latest one supported by gophertunnel.
	SyncProtocol bool `yaml:"sync_protocol"`
	// TransferAnimation is played while the player is moved between servers:
	// "none", "dimension", "fade", "smooth" or "ease".
	TransferAnimation string `yaml:"transfer_animation"`
}

// Servers holds the addresses Vortex sends the players to.
type Servers struct {
	// Balancer is the strategy used to pick an address: "round_robin", "random" or "first".
	Balancer string `yaml:"balancer"`
	// Primary is the pool of servers players are sent to on login.
	Primary []string `yaml:"primary"`
	// Fallback is the pool used when the current server dies mid-game. May be empty.
	Fallback []string `yaml:"fallback"`
}

// API holds the options of the TCP service used by the downstream servers.
type API struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	// Secret authenticates the servers. An empty secret disables authentication.
	Secret string `yaml:"secret"`
}

// Security holds the packet filtering rules applied to every session.
type Security struct {
	RateLimit RateLimit `yaml:"rate_limit"`
	// BlockedPackets is a list of client packet identifiers dropped by the proxy.
	BlockedPackets []uint32 `yaml:"blocked_packets"`
	// DecodePackets is a list of client packet identifiers the proxy fully decodes.
	// Everything else is forwarded untouched, which is what keeps the proxy fast.
	DecodePackets []uint32 `yaml:"decode_packets"`
	// MaxPacketSize is the maximum size in bytes of a client packet. 0 disables the check.
	MaxPacketSize int `yaml:"max_packet_size"`
}

// RateLimit limits how many packets a session may send per second.
type RateLimit struct {
	Enabled          bool `yaml:"enabled"`
	PacketsPerSecond int  `yaml:"packets_per_second"`
	// Action is applied when the limit is exceeded: "drop" or "kick".
	Action      string `yaml:"action"`
	KickMessage string `yaml:"kick_message"`
}

// Plugins holds the options of the plugins compiled into the proxy.
type Plugins struct {
	// Enabled determines whether the registered plugins are loaded at all.
	Enabled bool `yaml:"enabled"`
	// Directory is where each plugin gets a directory for its own files.
	Directory string `yaml:"directory"`
	// Disabled holds the names of the plugins that must not be loaded.
	Disabled []string `yaml:"disabled"`
}

// ResourcePacks holds the resource packs served by the proxy itself.
type ResourcePacks struct {
	Enabled   bool   `yaml:"enabled"`
	Directory string `yaml:"directory"`
	Required  bool   `yaml:"required"`
	// ContentKeys maps a pack UUID to its encryption key.
	ContentKeys map[string]string `yaml:"content_keys"`
}

// Logging holds the options of the logger.
type Logging struct {
	// Level is one of "debug", "info", "warn" or "error".
	Level string `yaml:"level"`
	JSON  bool   `yaml:"json"`
}

// Default returns the configuration used when no file exists yet.
func Default() *Config {
	return &Config{
		Proxy: Proxy{
			Addr:               ":19132",
			Name:               "Vortex Proxy",
			SubName:            "Vortex",
			Transport:          "spectral",
			XboxAuthentication: true,
			MaxPlayers:         0,
			LatencyInterval:    3000,
			LoginTimeout:       60,
			ShutdownMessage:    "Vortex closed.",
			SyncProtocol:       false,
			TransferAnimation:  "dimension",
		},
		Servers: Servers{
			Balancer: "round_robin",
			Primary:  []string{"127.0.0.1:19133"},
			Fallback: []string{},
		},
		API: API{
			Enabled: false,
			Addr:    "127.0.0.1:19131",
			Secret:  "",
		},
		Security: Security{
			RateLimit: RateLimit{
				Enabled:          true,
				PacketsPerSecond: 500,
				Action:           "drop",
				KickMessage:      "You are sending packets too fast.",
			},
			BlockedPackets: []uint32{},
			DecodePackets:  []uint32{},
			MaxPacketSize:  2 * 1024 * 1024,
		},
		Plugins: Plugins{
			Enabled:   true,
			Directory: "plugins",
			Disabled:  []string{},
		},
		ResourcePacks: ResourcePacks{
			Enabled:     false,
			Directory:   "resource_packs",
			Required:    false,
			ContentKeys: map[string]string{},
		},
		Logging: Logging{Level: "info", JSON: false},
	}
}

// Load reads the configuration at the given path. The default configuration is
// written to disk and returned when the file does not exist yet.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		conf := Default()
		if err := conf.Save(path); err != nil {
			return nil, err
		}
		return conf, nil
	} else if err != nil {
		return nil, err
	}

	conf := Default()
	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, fmt.Errorf("failed to decode %v: %w", path, err)
	}

	if err := conf.Validate(); err != nil {
		return nil, err
	}
	return conf, nil
}

// Save writes the configuration to the given path.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Validate reports whether the configuration may be used to start the proxy.
func (c *Config) Validate() error {
	if c.Proxy.Addr == "" {
		return fmt.Errorf("proxy.addr must not be empty")
	}

	if !slices.Contains([]string{"spectral", "quic"}, c.Proxy.Transport) {
		return fmt.Errorf("proxy.transport %q must be either spectral or quic", c.Proxy.Transport)
	}

	if !slices.Contains([]string{"none", "dimension", "fade", "smooth", "ease"}, c.Proxy.TransferAnimation) {
		return fmt.Errorf("proxy.transfer_animation %q is unknown", c.Proxy.TransferAnimation)
	}

	if c.Proxy.LatencyInterval <= 0 {
		return fmt.Errorf("proxy.latency_interval must be greater than zero")
	}

	if c.Proxy.LoginTimeout <= 0 {
		return fmt.Errorf("proxy.login_timeout must be greater than zero")
	}

	if c.Proxy.MaxPlayers < 0 {
		return fmt.Errorf("proxy.max_players must not be negative")
	}

	if len(c.Servers.Primary) == 0 {
		return fmt.Errorf("servers.primary must hold at least one address")
	}

	if !slices.Contains([]string{"round_robin", "random", "first"}, c.Servers.Balancer) {
		return fmt.Errorf("servers.balancer %q is unknown", c.Servers.Balancer)
	}

	if c.API.Enabled && c.API.Addr == "" {
		return fmt.Errorf("api.addr must not be empty when the api is enabled")
	}

	if c.Security.RateLimit.Enabled {
		if c.Security.RateLimit.PacketsPerSecond <= 0 {
			return fmt.Errorf("security.rate_limit.packets_per_second must be greater than zero")
		}

		if !slices.Contains([]string{"drop", "kick"}, c.Security.RateLimit.Action) {
			return fmt.Errorf("security.rate_limit.action %q must be either drop or kick", c.Security.RateLimit.Action)
		}
	}

	if c.Security.MaxPacketSize < 0 {
		return fmt.Errorf("security.max_packet_size must not be negative")
	}

	if c.Plugins.Enabled && c.Plugins.Directory == "" {
		return fmt.Errorf("plugins.directory must not be empty when plugins are enabled")
	}
	return nil
}
