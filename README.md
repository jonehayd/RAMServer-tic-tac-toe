# RAMServer Tic-Tac-Toe

This repository is where a tic-tac-toe game for RAMServer is built from the outside, the way any
third-party developer would build a game for it:

- a **ruleset** in Go, implementing the server's `game.Game` interface, and
- a **client** in C, which connects to a local or hosted server and plays a match against another client.

RAMServer is a game-agnostic multiplayer server. It handles connections, players, sessions, turn order,
and broadcasting state. Games plug in rulesets that decide everything about the rules. Clients talk to
the server through a documented protobuf protocol and never see each other directly.

## What's here

| Path | Contents |
| --- | --- |
| `AGENTS.md` | How to work in this repository. Read this first. |
| `docs/protocol.md` | The wire protocol: every message, field and error, and the session flow. |
| `docs/framing.md` | How messages are delimited on a connection. |
| `docs/websocket.md` | Connecting over WebSocket, as the hosted server requires. |
| `docs/writing-a-ruleset.md` | The `game.Game` contract: what the server calls, when, and what it expects. |
| `docs/writing-a-client.md` | How a client should connect, play, and handle errors. |
| `docs/servers.md` | The local and hosted servers, and how a new ruleset reaches them. |
| `proto/ramserver/v1/ramserver.proto` | The protocol schema. Generate client code from it. |
| `sdk/` | The server's public `game` package, which rulesets implement, and its compliance tests (`sdk/game/gametest`). See `sdk/README.md`. |
| `go.mod` | This repository's Go module. It points the server's import path at `sdk/`, so nothing is downloaded. |
| `server/` | A local server build, once the ruleset has been integrated (see `docs/servers.md`). |
| `Makefile` | `make test` runs every test in the repository. |
| `Dockerfile`, `docker-compose.yml` | The development environment, as a container. |
| `scripts/install-deps.sh` | Installs the same tools without a container (Debian/Ubuntu). |

## The protocol in one minute

1. Open a connection (TCP, or WebSocket for the hosted server).
2. Send `ConnectRequest`, then `AuthRequest` with a display name; you get a `player_id`.
3. Send `JoinSessionRequest` with `game_id` `"tictactoe"` to quick-match, create, or join a session.
4. Every member sends `SetReadyRequest`; when the game's start condition is met, the match starts.
5. Players send `ActionRequest`s with game-defined bytes; everyone receives `StateUpdate`s with
   game-defined views.
6. The match ends with a `MatchResult`; players can ready up again for a rematch, or leave.

The details are in `docs/protocol.md`.

## Development environment

Every developer, and every automated run, should build and test with the same tools. The easiest way is
the container defined by `Dockerfile` and `docker-compose.yml`. It has Go 1.24, a C toolchain (gcc,
CMake, gdb, valgrind), `protoc` with the C plugin `protoc-c`, protobuf-c, libwebsockets, OpenSSL, and
cmocka. All you need installed is Docker.

```bash
docker compose run --rm dev make test   # run every test
docker compose run --rm dev bash        # a shell inside the container, to build and run things
```

- The repository is mounted into the container, so edit files with any editor on your own machine and
  run commands in the container.
- The first run builds the image, which takes a few minutes. After changing `Dockerfile` or
  `scripts/install-deps.sh`, rebuild with `docker compose build`.
- A client running in the container reaches a server on your own machine at `host.docker.internal`
  instead of `127.0.0.1`. The hosted server works as usual.

**Without Docker (Debian/Ubuntu, including WSL):** install Go 1.24 or newer, run `make check-deps` to
install the same packages, then `make test`.

The C client's tests join `make test` once `client/` has a Makefile with a `test` target.

## Versions

- Protocol version **1.2**.
- Go version: see `go.mod`.
