#!/usr/bin/env bash
# Installs every system package this repository builds and tests with, on Debian or Ubuntu.
# The Dockerfile runs this, so a machine that runs it gets the same tools as the container. If you
# add a dependency, add it here, so every developer and the container stay in sync.
set -euo pipefail

if ! command -v apt-get >/dev/null; then
    echo "install-deps.sh supports Debian and Ubuntu (apt-get). Use the Docker container on other systems (see the README)." >&2
    exit 1
fi

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi

$SUDO apt-get update
$SUDO apt-get install -y --no-install-recommends \
    build-essential cmake pkg-config git ca-certificates curl \
    gdb valgrind \
    protobuf-compiler protobuf-c-compiler libprotobuf-c-dev \
    libwebsockets-dev libssl-dev \
    libcmocka-dev

# Go is not installed from apt, whose version is too old. Check it's present and recent enough.
required_go="1.24"
if ! command -v go >/dev/null; then
    echo "Go is not installed. Install Go ${required_go} or newer from https://go.dev/dl/ (the Docker container includes it)." >&2
    exit 1
fi
installed_go="$(go env GOVERSION | sed 's/^go//')"
if [ "$(printf '%s\n%s\n' "$required_go" "$installed_go" | sort -V | head -n1)" != "$required_go" ]; then
    echo "Go ${installed_go} is too old; this repository needs Go ${required_go} or newer." >&2
    exit 1
fi

echo "All dependencies are installed."
