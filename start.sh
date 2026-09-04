#!/usr/bin/env bash
#
# Vortex launcher for private hosts (VPS and dedicated servers).
#
# Installs what is missing, builds the binary and starts the proxy.
# For the Pterodactyl panel use the egg in pterodactyl/ instead.

set -euo pipefail

readonly ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly BINARY="${ROOT}/vortex"

# Go 1.21 and newer download the toolchain a module asks for, so anything from
# that version on can build Vortex.
readonly MIN_GO_MAJOR=1
readonly MIN_GO_MINOR=21

CONFIG="config.yml"
BUILD=1
UPDATE=0
TUNE=0
START=1

log()  { printf '\033[0;36m[vortex]\033[0m %s\n' "$*"; }
warn() { printf '\033[0;33m[vortex]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[0;31m[vortex]\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
    cat <<'USAGE'
Usage: ./start.sh [options]

Options:
  --config <path>   Configuration file to use (default: config.yml)
  --no-build        Skip the build and start the existing binary
  --build-only      Build the binary and exit
  --update          Pull the repository before building
  --tune            Apply the kernel network tuning (needs root)
  -h, --help        Show this help

Examples:
  ./start.sh                       install, build and start
  ./start.sh --update --tune       update, tune the kernel and start
  ./start.sh --no-build            start the binary that is already built
USAGE
}

while [ $# -gt 0 ]; do
    case "$1" in
        --config) CONFIG="${2:-}"; [ -n "$CONFIG" ] || die "--config needs a path"; shift 2 ;;
        --no-build) BUILD=0; shift ;;
        --build-only) START=0; shift ;;
        --update) UPDATE=1; shift ;;
        --tune) TUNE=1; shift ;;
        -h|--help) usage; exit 0 ;;
        *) die "unknown option: $1 (try --help)" ;;
    esac
done

# sudo_run runs a command as root when possible.
sudo_run() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo "$@"
    else
        return 1
    fi
}

# install_packages installs the given packages with the distribution's package manager.
install_packages() {
    [ $# -gt 0 ] || return 0
    log "installing: $*"

    if command -v apt-get >/dev/null 2>&1; then
        sudo_run env DEBIAN_FRONTEND=noninteractive apt-get update -qq &&
            sudo_run env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$@"
    elif command -v dnf >/dev/null 2>&1; then
        sudo_run dnf install -y -q "$@"
    elif command -v yum >/dev/null 2>&1; then
        sudo_run yum install -y -q "$@"
    elif command -v pacman >/dev/null 2>&1; then
        sudo_run pacman -Sy --noconfirm --needed "$@"
    elif command -v apk >/dev/null 2>&1; then
        sudo_run apk add --no-cache "$@"
    else
        return 1
    fi
}

# ensure_tools makes sure the basic tools used by this script are present.
ensure_tools() {
    local missing=()
    command -v curl >/dev/null 2>&1 || missing+=("curl")
    command -v tar  >/dev/null 2>&1 || missing+=("tar")
    command -v git  >/dev/null 2>&1 || missing+=("git")

    if [ ${#missing[@]} -eq 0 ]; then
        return 0
    fi

    if ! install_packages "${missing[@]}"; then
        die "missing tools (${missing[*]}) and they could not be installed automatically"
    fi
}

# go_version_ok reports whether the Go in PATH is new enough to build Vortex.
go_version_ok() {
    command -v go >/dev/null 2>&1 || return 1

    local raw major minor
    raw="$(go env GOVERSION 2>/dev/null || true)"
    raw="${raw#go}"
    major="${raw%%.*}"
    minor="${raw#*.}"
    minor="${minor%%.*}"

    case "$major$minor" in
        ''|*[!0-9]*) return 1 ;;
    esac

    [ "$major" -gt "$MIN_GO_MAJOR" ] ||
        { [ "$major" -eq "$MIN_GO_MAJOR" ] && [ "$minor" -ge "$MIN_GO_MINOR" ]; }
}

# required_go returns the Go version declared in go.mod.
required_go() {
    awk '/^go [0-9]/ { print $2; exit }' "${ROOT}/go.mod"
}

# install_go downloads the Go toolchain into ~/.local/go.
install_go() {
    local version os arch url prefix
    version="$(required_go)"
    version="${version:-1.25.0}"

    case "$(uname -s)" in
        Linux) os="linux" ;;
        Darwin) os="darwin" ;;
        *) die "unsupported operating system: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l|armv6l) arch="armv6l" ;;
        *) die "unsupported architecture: $(uname -m)" ;;
    esac

    prefix="${HOME}/.local"
    url="https://go.dev/dl/go${version}.${os}-${arch}.tar.gz"

    log "installing Go ${version} into ${prefix}/go"
    mkdir -p "$prefix"
    rm -rf "${prefix}/go"
    curl -fsSL "$url" | tar -xz -C "$prefix"

    export PATH="${prefix}/go/bin:${PATH}"
    go_version_ok || die "the Go installation failed, install Go ${version} manually"
}

# ensure_go makes sure a usable Go toolchain is in PATH.
ensure_go() {
    if [ -x "${HOME}/.local/go/bin/go" ]; then
        export PATH="${HOME}/.local/go/bin:${PATH}"
    fi

    if go_version_ok; then
        log "using $(go env GOVERSION)"
        return 0
    fi

    if command -v go >/dev/null 2>&1; then
        warn "$(go env GOVERSION) is too old, installing a newer toolchain"
    fi
    install_go
}

# update_sources pulls the repository, keeping local commits intact.
update_sources() {
    [ -d "${ROOT}/.git" ] || { warn "not a git checkout, skipping the update"; return 0; }

    log "updating the sources"
    git -C "$ROOT" pull --ff-only || warn "the update failed, building the current sources"
}

# build_binary compiles the proxy.
build_binary() {
    local version
    version="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"

    log "building vortex ${version}"
    # Lets Go fetch the toolchain the module asks for when the local one is older.
    GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" CGO_ENABLED=0 go build \
        -C "$ROOT" -trimpath -ldflags "-s -w -X main.version=${version}" \
        -o "$BINARY" ./cmd/vortex
    log "built ${BINARY}"
}

# ensure_config creates the configuration file on the first run.
ensure_config() {
    local path="$CONFIG"
    case "$path" in
        /*) ;;
        *) path="${ROOT}/${path}" ;;
    esac

    if [ -f "$path" ]; then
        return 0
    fi

    if [ -f "${ROOT}/config.example.yml" ]; then
        log "creating ${path} from config.example.yml"
        cp "${ROOT}/config.example.yml" "$path"
    else
        log "${path} will be created with the default values on the first run"
    fi
}

# tune_kernel raises the UDP buffers, as recommended by Spectrum.
tune_kernel() {
    if [ "$(uname -s)" != "Linux" ]; then
        warn "kernel tuning is only available on Linux, skipping"
        return 0
    fi

    log "applying the kernel network tuning"
    if ! sudo_run sysctl -qw net.core.rmem_max=7500000 \
        net.core.wmem_max=7500000 \
        net.ipv4.tcp_rmem="4096 87380 7500000"; then
        warn "could not apply the tuning, run this script as root to enable it"
        return 0
    fi
    log "make it permanent with a file in /etc/sysctl.d/"
}

main() {
    log "Vortex — Minecraft: Bedrock proxy powered by Spectrum"

    if [ "$TUNE" -eq 1 ]; then
        tune_kernel
    fi

    if [ "$BUILD" -eq 1 ]; then
        ensure_tools
        if [ "$UPDATE" -eq 1 ]; then
            update_sources
        fi
        ensure_go
        build_binary
    fi

    [ -x "$BINARY" ] || die "vortex is not built, run this script without --no-build"
    ensure_config

    if [ "$START" -eq 0 ]; then
        log "build finished"
        return 0
    fi

    log "starting the proxy with ${CONFIG}"
    cd "$ROOT"
    exec "$BINARY" -config "$CONFIG"
}

main "$@"
