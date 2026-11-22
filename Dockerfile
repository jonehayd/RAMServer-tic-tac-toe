# Development environment for this repository: Go for the ruleset, and a C toolchain with protobuf,
# WebSocket, and TLS libraries for the client. The fixed base image keeps versions the same on every
# machine. scripts/install-deps.sh lists the packages, so machines without Docker can match it.
#
# Use it through docker-compose.yml (see the README), not on its own.
FROM golang:1.24.0-bookworm

COPY scripts/install-deps.sh /tmp/install-deps.sh
RUN bash /tmp/install-deps.sh && rm -rf /var/lib/apt/lists/* /tmp/install-deps.sh

# The repository is mounted here by docker-compose.yml, so edits on your machine are seen immediately.
# The RAMServer module is never downloaded: go.mod's replace directive points it at ./sdk.
WORKDIR /workspace
