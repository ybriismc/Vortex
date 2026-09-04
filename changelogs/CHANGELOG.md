# Changelog

All notable changes to Vortex are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Every release also has its own file in this directory, which is what the release
workflow publishes as the release notes.

## [Unreleased]

Nothing yet.

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

[Unreleased]: https://github.com/ybriismc/Vortex/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ybriismc/Vortex/releases/tag/v0.1.0
