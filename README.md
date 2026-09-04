# Vortex

[![Build](https://github.com/ybriismc/Vortex/actions/workflows/build.yml/badge.svg)](https://github.com/ybriismc/Vortex/actions/workflows/build.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Minecraft: Bedrock Edition proxy written in Go, built on top of
[Spectrum](https://github.com/cooldogedev/spectrum).

Vortex packages Spectrum into a binary that is ready for production: YAML
configuration, server balancing, packet filtering (rate limit, block list and
size limit), transfer animations, resource packs and the API service for the
downstream servers.

📖 **Documentation:** https://ybriismc.github.io/Vortex/

---

## About Spectrum

Spectrum is the foundation of Vortex. The points that matter when running this proxy:

- **Its own protocol, no RakNet between proxy and server.** The link to the
  downstream servers uses [Spectral](https://github.com/cooldogedev/spectral),
  QUIC or TCP instead of RakNet and the standard Minecraft protocol, which is
  more reliable and faster. Each server keeps **a single connection**, and each
  player rides a *stream* inside it — cheaper dialing and less connection overhead.
- **Packet forwarding without decoding.** By default the proxy does not decode
  the player's packets, it simply forwards the bytes. Only the identifiers listed
  in `client_decode` (in Vortex: `security.decode_packets`) are decoded. That is
  what keeps the throughput high and the latency low.
- **Discovery.** Instead of a fixed list of servers, Spectrum asks a `Discovery`
  interface where to send the player on login, and where to send them when the
  server dies (fallback). The call is asynchronous, so it may perform blocking
  work — database queries, HTTP requests — and it doubles as a load balancer.
- **Processor.** The `Processor` interface intercepts every packet entering and
  leaving the session, plus events such as transfer, fallback, cache and
  disconnection. Any packet can be cancelled. It is the right place for
  anti-cheat, filters and telemetry.
- **Stateless.** The proxy keeps no registry of the existing servers.
  Transferring a player is just sending Spectrum's transfer packet from the
  downstream server, which makes horizontal scaling simple.
- **Deterministic.** Spectrum skips entity translation entirely: it relies on the
  deterministic entity identifiers provided by the downstream servers, dropping a
  whole translation layer (and its bugs).
- **API service.** A separate TCP service lets the servers transfer and kick
  players, with secret based authentication and support for custom packets and
  handlers.
- **Transfer animations.** The server switch can be masked by camera animations
  (`dimension`, `fade`, `smooth`, `ease`).
- **Compatible server implementations:**
  [spectrum-df](https://github.com/cooldogedev/spectrum-df) (Dragonfly) and
  [spectrum-pm](https://github.com/cooldogedev/spectrum-pm) (PocketMine-MP).

> The server behind the proxy **must** speak the Spectrum protocol. A plain
> Bedrock server (pure RakNet) cannot be used as a downstream server.

---

## What Vortex adds

| Feature | Description |
| --- | --- |
| YAML configuration | `config.yml` generated automatically on the first run |
| Balancing | Primary and fallback server pools with `round_robin`, `random` or `first` |
| Guard | A `Processor` with a per session rate limit, packet block list by ID and a size limit |
| Controlled login | The guard and the animation are attached **before** the session login starts |
| Animations | Selected through the configuration; the camera follows the player in `smooth` and `ease` |
| API | Spectrum's TCP service with secret authentication, toggled by configuration |
| Resource packs | Loaded from a directory, with content key support |
| Operations | Text or JSON logs and a clean shutdown on `SIGINT`/`SIGTERM` |

---

## Requirements

- Go 1.25 or newer
- A downstream server running Spectrum (spectrum-df or spectrum-pm)

## Installation

### From source

```bash
git clone https://github.com/ybriismc/Vortex.git
cd Vortex
make build
./vortex
```

### From a release

```bash
curl -L -o vortex.tar.gz https://github.com/ybriismc/Vortex/releases/latest/download/vortex-linux-amd64.tar.gz
tar -xzf vortex.tar.gz
./vortex
```

The first run creates `config.yml` with the default values. Edit it and start the
proxy again. To use another path:

```bash
./vortex -config /etc/vortex/config.yml
```

### Private hosts (VPS and dedicated servers)

[`start.sh`](start.sh) installs the toolchain when it is missing, builds the
binary and starts the proxy:

```bash
./start.sh              # installs what is needed, builds and starts
./start.sh --no-build   # starts the existing binary
./start.sh --update     # pulls the repository, rebuilds and starts
./start.sh --tune       # also applies the kernel network tuning (needs root)
```

### Pterodactyl

The [`pterodactyl/`](pterodactyl) directory holds an egg that downloads the
release build and starts it. Import `pterodactyl/egg-vortex.json` in the panel,
under **Nests → Import Egg**.

---

## Configuration

The commented file lives in [`config.example.yml`](config.example.yml). Summary of the sections:

### `proxy`

| Key | Default | Description |
| --- | --- | --- |
| `addr` | `:19132` | UDP address the players connect to |
| `name` / `sub_name` | `Vortex Proxy` / `Vortex` | Text shown in the server list |
| `transport` | `spectral` | Transport to the servers: `spectral` or `quic` |
| `xbox_authentication` | `true` | Requires an authenticated Xbox Live account |
| `max_players` | `0` | Player limit (`0` = unlimited) |
| `latency_interval` | `3000` | Latency report interval in milliseconds |
| `login_timeout` | `60` | Timeout of the login sequence, in seconds |
| `shutdown_message` | `Vortex closed.` | Message sent on shutdown |
| `sync_protocol` | `false` | Talks to the server using the client's protocol version |
| `transfer_animation` | `dimension` | `none`, `dimension`, `fade`, `smooth` or `ease` |

### `servers`

`primary` receives the players on login; `fallback` is used when the current
server dies mid-game. `balancer` picks the address: `round_robin`, `random` or
`first`. Gameplay servers do not belong here — the lobby transfers players to
them, and the proxy dials whatever address the transfer names.

### `security`

- `rate_limit`: packets per second per session; when exceeded, Vortex either
  drops the packets (`drop`) or disconnects the player (`kick`).
- `blocked_packets`: client packet identifiers that never reach the server. The
  identifier is read straight from the header, **without decoding the packet**.
- `decode_packets`: identifiers the proxy fully decodes (Spectrum's
  `client_decode`). The shorter the list, the faster the proxy.
- `max_packet_size`: maximum size, in bytes, of a client packet.

### `api`

TCP service for the downstream servers. With `secret` set, the server has to send
the same secret in its `ConnectionRequest`. Available packets:

| ID | Packet | Effect |
| --- | --- | --- |
| `0` | `ConnectionRequest` | Authenticates the server against the service |
| `1` | `ConnectionResponse` | Authentication result |
| `2` | `Kick` | Disconnects a player by username |
| `3` | `Transfer` | Transfers a player to another address |

On the game side, the Spectrum protocol packets (identifiers from `500` on) let
the server request `Transfer`, `Flush`, `Latency` and `UpdateCache` directly on
the session connection.

---

## Project layout

```
cmd/vortex          binary and flag parsing
internal/config     YAML configuration, defaults and validation
internal/discovery  server.Discovery with pools and balancing
internal/guard      session.Processor with rate limit and filters
internal/proxy      Spectrum wiring, animations, packs and API
docs                GitHub Pages documentation
changelogs          release notes
pterodactyl         egg for the Pterodactyl panel
```

---

## Kernel tuning (Linux)

Under heavy load the default Linux network buffers may be too small and cause
random errors or disconnections. Spectrum's recommendation:

```bash
sysctl -w net.core.rmem_max=7500000
sysctl -w net.core.wmem_max=7500000
sysctl -w net.ipv4.tcp_rmem="4096 87380 7500000"
```

`./start.sh --tune` applies these values for you.

---

## Development

```bash
make fmt    # gofmt
make vet    # go vet
go test ./...
make build
```

---

## Credits

- [Spectrum](https://github.com/cooldogedev/spectrum) and
  [Spectral](https://github.com/cooldogedev/spectral), by
  [cooldogedev](https://github.com/cooldogedev)
- [gophertunnel](https://github.com/sandertv/gophertunnel), by
  [Sandertv](https://github.com/Sandertv)

## License

[MIT](LICENSE)
