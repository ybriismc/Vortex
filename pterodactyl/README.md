# Pterodactyl egg

`egg-vortex.json` installs Vortex from the GitHub releases and starts it inside a
Pterodactyl server.

## Import

1. In the panel, go to **Nests → Import Egg**.
2. Upload `egg-vortex.json` and pick the nest it should live in.
3. Create a server with this egg.

## Variables

| Variable | Default | Description |
| --- | --- | --- |
| `VERSION` | `latest` | Release to install, for example `v0.1.0` |
| `CONFIG_FILE` | `config.yml` | Configuration file passed to the binary |

To update the proxy, change `VERSION` (or keep `latest`) and reinstall the
server — the install script downloads the build again.

## What the install script does

- Detects the node architecture (`amd64` or `arm64`).
- Downloads `vortex-linux-<arch>.tar.gz` from the release named by `VERSION`.
- Extracts the binary, `config.example.yml`, `README.md` and `LICENSE` into the
  server directory.
- Creates `config.yml` from the example when it does not exist yet.

## Allocations

- The **primary allocation** is the port players connect to. The panel rewrites
  `proxy.addr` with it on every boot, so leave that key alone in the file editor.
- Vortex speaks **UDP** on that port. Make sure the node's firewall forwards UDP,
  not only TCP.
- Enabling the API (`api.enabled: true`) needs a **second allocation**, and its
  address has to be set manually in `config.yml`. Keep it on a private interface.

## After installing

Edit `config.yml` and set `servers.primary` to your lobbies. Gameplay servers do
not belong there — the lobby transfers players to them. The downstream servers
must run [spectrum-df](https://github.com/cooldogedev/spectrum-df) or
[spectrum-pm](https://github.com/cooldogedev/spectrum-pm), listening on the same
transport as `proxy.transport`.
