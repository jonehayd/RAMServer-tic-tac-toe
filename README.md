# RAMServer Tic-Tac-Toe

This repository is where a tic-tac-toe game for RAMServer is built from the outside, the way any
third-party developer would build a game for it:

- a **ruleset** in Go, implementing the server's `game.Game` interface, and
- a **client** in C, which connects to a local or hosted server and plays a match against another client.

RAMServer is a game-agnostic multiplayer server. It handles connections, players, sessions, turn order,
and broadcasting state. Games plug in rulesets that decide everything about the rules. Clients talk to
the server through a documented protobuf protocol and never see each other directly.

## What's here

| Path                                 | Contents                                                                                                                           |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| `AGENTS.md`                          | How to work in this repository. Read this first.                                                                                   |
| `docs/protocol.md`                   | The wire protocol: every message, field and error, and the session flow.                                                           |
| `docs/framing.md`                    | How messages are delimited on a connection.                                                                                        |
| `docs/websocket.md`                  | Connecting over WebSocket, as the hosted server requires.                                                                          |
| `docs/writing-a-ruleset.md`          | The `game.Game` contract: what the server calls, when, and what it expects.                                                        |
| `docs/writing-a-client.md`           | How a client should connect, play, and handle errors.                                                                              |
| `docs/servers.md`                    | The local and hosted servers, and how a new ruleset reaches them.                                                                  |
| `proto/ramserver/v1/ramserver.proto` | The protocol schema. Generate client code from it.                                                                                 |
| `rulesets/tictactoe/`                | The standard tic-tac-toe ruleset, tests, and game-encoding README.                                                                  |
| `rulesets/ultimatetictactoe/`         | The Ultimate Tic-Tac-Toe ruleset, tests, and game-encoding README.                                                                  |
| `client/`                            | The C client (`ttt`): terminal UI, protocol and networking, unit and integration tests. See `client/README.md`.                     |
| `sdk/`                               | The server's public `game` package, which rulesets implement, and its compliance tests (`sdk/game/gametest`). See `sdk/README.md`. |
| `go.mod`                             | This repository's Go module. It points the server's import path at `sdk/`, so nothing is downloaded.                               |
| `server/`                            | A local server build, once the ruleset has been integrated (see `docs/servers.md`).                                                |
| `Makefile`                           | `make test` runs every test in the repository.                                                                                     |
| `Dockerfile`, `docker-compose.yml`   | The development environment, as a container.                                                                                       |
| `scripts/install-deps.sh`            | Installs the same tools without a container (Debian/Ubuntu).                                                                       |

## The protocol in one minute

1. Open a connection (TCP, or WebSocket for the hosted server).
2. Send `ConnectRequest`, then `AuthRequest` with a display name; you get a `player_id`.
3. Send `JoinSessionRequest` with `game_id` `"tictactoe"` or `"ultimate_tictactoe"` to quick-match,
   create, or join a session. The client selects with `--game tictactoe` (default) or `--game ultimate`.
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

`make test` also runs the C client's tests (`make -C client test`). To play either game, build the
client with `make -C client` and see `client/README.md`.

## Versions

- Protocol version **1.2**.
- Go version: see `go.mod`.

## Agent choice

Claude Opus 5.5

- Good model that works in the cloud and can access repositories.
- Good configuration for limiting actions in CLAUDE.md and can prevent agent from looking up server code in the public repository

## Ruleset Prompt

You're an external developer building a game for RAMServer, a game-agnostic multiplayer server. Your first
job is the tic-tac-toe ruleset.

Start by reading AGENTS.md and README.md, then docs/writing-a-ruleset.md and docs/protocol.md. Everything you
need is in this repository. Do not look for, download, or read RAMServer's own source code anywhere - not on
GitHub, not in Go's module cache, not through any package manager. If the documentation doesn't answer a
question, make the choice that follows the protocol most closely and write the question down in your README.

Build a ruleset in Go, in rulesets/tictactoe, that implements game.Game from sdk/game:

- Standard tic-tac-toe rules on a 3x3 board: players alternate placing their mark, three in a row
  (horizontal, vertical, or diagonal) wins, and a full board with no line is a draw.
- It must support at least two players. Decide whether others can join to watch, and whether a match is a
  single game or a series (best of three, and so on). Explain your choices in the README.
- Design the encodings clients will use: the action a player sends (encoded_action) and the view each
  member receives (encoded_view). Keep them simple to implement in C, and specify them precisely, with
  examples - a C client will be written from your README alone.
- Use the optional interfaces (annotations, out-of-turn actions) if they make the game better for clients,
  and document anything you add.

Testing and documentation:

- Unit tests for the rules: every way a game can end, illegal and malformed moves, turn order, and that
  Apply never modifies the state it receives.
- A compliance test that runs sdk/game/gametest against your game, with a roster that includes every kind
  of member your game allows.
- `make test` at the repository root must pass.
- rulesets/tictactoe/README.md must cover: the game_id, roles and player counts, the start condition, the
  exact action and view formats with examples, annotation keys, every termination_reason you produce, and
  the error messages players can see.

When you're done, report: what you built, the main design decisions and why, the exact test commands you ran
with their results, and anything in the documentation that was unclear or missing.

## Client Prompt

Next, build a client in C that plays tic-tac-toe through RAMServer, in client/.

Read docs/writing-a-client.md, docs/framing.md, docs/websocket.md, and docs/servers.md, and use your
ruleset's README as the description of the game's encodings. The same rule applies as before: don't look for
or read the server's source code.

Requirements:

- Connect to a local server over TCP, and to the hosted server over WebSocket with TLS
  (wss://ramserver-core-80752473398.us-east5.run.app/connect). Let the user choose the server.
- Handle the whole session properly: the handshake, finding a game (quick match, creating a private
  session, and joining one by its id), readying up, taking turns, showing the result, rematches, leaving a
  session, and quitting or losing the connection cleanly. Reply to heartbeats and keep reading the
  connection at all times, as the timing rules require.
- Two instances of the client must be able to play a full match against each other.
- Treat the server as the source of truth: draw the board from the StateUpdates you receive, and show the
  player any error the server sends.

The development team runs it in WSL on Windows 11, where GUI windows appear on the Windows desktop. A
terminal UI or a GUI library are both fine, and you're encouraged to make creative choices about the
interface and experience, as long as everything follows the protocol. Whatever you choose:

- It must build and pass `make test` in the Docker environment (docker compose run --rm dev make test)
  without a display. Keep the protocol, networking, and game state separate from the rendering, so they
  can be tested on their own.
- If you use a GUI library, prefer a cross-platform one, and add any new system packages to
  scripts/install-deps.sh.
- client/ needs a Makefile with a `test` target, so the root `make test` runs your tests too.

Testing and documentation:

- Unit tests for framing (including partial and combined reads), building every request you send,
  decoding views, and updating your session state from sequences of server messages, including errors
  and match ends.
- client/README.md must cover: how to build it, how to run it against a local and the hosted server, how
  to play two instances against each other, the controls, and the design and creative choices you made.

When you're done, report: what you built, the creative choices and why, the exact test commands you ran
with their results, whether you could connect to a server (and which), and anything in the documentation
that was unclear or missing.
