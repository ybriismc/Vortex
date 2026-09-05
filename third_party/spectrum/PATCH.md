# Patched Spectrum

This directory is [Spectrum](https://github.com/cooldogedev/spectrum) **v0.0.44**
with one patch, used through a `replace` directive in the root `go.mod`.

## Why

Vortex targets Minecraft: Bedrock Edition **1.26.45, protocol 2169**, which
needs gophertunnel v1.61.0. Spectrum v0.0.44 pins gophertunnel v1.57.1 and does
not compile against v1.61.0: gophertunnel moved `ActionType` from the
`PlayerList` packet into each `PlayerListEntry`, and the action constants moved
from the `packet` package to `protocol`.

Upstream has no release with that change yet.

## The patch

`session/tracker.go`, two places, both marked with a `Vortex patch` comment:

- Reading a player list now checks `entry.ActionType` against
  `protocol.PlayerListActionAdd` instead of the packet-wide field.
- Clearing the player list marks `protocol.PlayerListActionRemove` on every
  entry instead of on the packet.

`go.mod` requires gophertunnel v1.61.0. The `example` directory was removed; it
is not part of the library.

Nothing else was touched.

## Dropping it

When Spectrum releases a version built against gophertunnel v1.61.0 or newer:

```bash
# remove the replace directive from the root go.mod
go get github.com/cooldogedev/spectrum@latest
go mod tidy
rm -rf third_party/spectrum
go build ./... && go test ./...
```

Spectrum is MIT licensed; its `LICENSE` is kept in this directory.
