# Changelog

All notable changes to Vortex are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Every release also has its own file in this directory, which is what the release
workflow publishes as the release notes.

## [Unreleased]

Nothing yet.

## [1.0.0] - 2026-09-05

First stable release of the proxy. See [v1.0.0.md](v1.0.0.md).

### Added

- Release builds of the proxy for Linux, Windows and macOS.

### Changed

- The release workflow pins the tag to the commit it built.

## [0.4.0] - 2026-09-05

Minecraft 1.26.45, protocol 2169. See [v0.4.0.md](v0.4.0.md).

### Changed

- gophertunnel updated to v1.61.0, moving the proxy from protocol 1001 to
  **2169** (Minecraft 1.26.45).
- The protocol is printed by `./vortex -version` and logged on startup.
- `third_party/spectrum` carries Spectrum v0.0.44 with a two line patch for the
  player list change, used through a `replace` directive, because upstream has
  no release against gophertunnel v1.61.0 yet.

## [0.3.0] - 2026-09-04

Plugins loaded from the plugin directory. See [v0.3.0.md](v0.3.0.md).

### Added

- `plugin.Loader`, which opens the `.so` files in the plugin directory at
  startup, so a plugin can be added without rebuilding the proxy.
- Actionable errors for the conditions Go imposes on plugins: a proxy built
  without cgo, a platform that cannot load plugins, a version mismatch or a
  wrong architecture.
- `make plugin` and `make plugins`, and automatic rebuilds of `plugins/src/*`
  in `start.sh`.
- A loader test that builds the example plugin and loads it.

### Changed

- Linux release builds are dynamically linked (cgo) so they can load plugins;
  Windows and macOS builds stay static and cannot.
- The example plugin is a `main` package, built as a plugin file.

## [0.2.0] - 2026-09-04

Plugin API. See [v0.2.0.md](v0.2.0.md).

### Added

- `plugin` package: plugin lifecycle (`Manifest`, `Load`, `Enable`, `Disable`),
  registration from `init`, dependency ordering, per plugin directory, logger
  and `config.yml`.
- `event` package: generic event bus with priorities, cancellable events and
  panic recovery around handlers.
- Events for the proxy lifecycle, logins, joins, quits, server selection,
  transfers, fallbacks, chat and commands.
- `plugin.Proxy`: sessions, transfer, kick, message, broadcast and the server
  pools.
- `plugins` section in the configuration and an example plugin in
  `examples/plugins/greeter`.
- Plugin API documentation page.

## [0.1.0] - 2026-09-04

First release. See [v0.1.0.md](v0.1.0.md).

### Added

- Spectrum based proxy driven by a YAML configuration file.
- Server discovery with primary and fallback pools and `round_robin`, `random`
  and `first` balancing.
- Packet guard: per session rate limit, block list by packet identifier and a
  maximum packet size.
- Transfer animations (`none`, `dimension`, `fade`, `smooth`, `ease`), with the
  camera following the player in `smooth` and `ease`.
- Spectral and QUIC transports to the downstream servers.
- Spectrum API service with secret based authentication.
- Resource pack loading with content key support.
- Text and JSON logging, and a clean shutdown on `SIGINT`/`SIGTERM`.
- GitHub Pages documentation, build and release workflows, a Pterodactyl egg and
  a start script for private hosts.

[Unreleased]: https://github.com/ybriismc/Vortex/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/ybriismc/Vortex/releases/tag/v1.0.0
[0.4.0]: https://github.com/ybriismc/Vortex/releases/tag/v0.4.0
[0.3.0]: https://github.com/ybriismc/Vortex/releases/tag/v0.3.0
[0.2.0]: https://github.com/ybriismc/Vortex/releases/tag/v0.2.0
[0.1.0]: https://github.com/ybriismc/Vortex/releases/tag/v0.1.0
