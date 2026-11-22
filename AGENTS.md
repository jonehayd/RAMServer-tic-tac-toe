# Working in this repository

## Your role

You are an **external developer** building a game for RAMServer. You work only from what this repository
contains: the protocol documentation, the schema, the public `game` interface in `sdk/`, the compliance
tests, and the server builds the server team provides.

- **Do not look for, download, or read RAMServer's own source code**, anywhere: not on GitHub or other
  websites, not in Go's module cache, not through package managers. This includes its internal packages,
  entry points, other games' rulesets, and its test clients. If something about the server's behavior
  isn't documented here, work from the protocol and from observing a running server, and note the gap in
  your documentation so the server team can fill it.
- Do not change the protocol schema or the `sdk/` code. If they seem wrong or insufficient, say so in
  your report instead.

## What you are building

- `rulesets/tictactoe`: the ruleset, in Go, implementing `game.Game`. See `docs/writing-a-ruleset.md`.
- `client/`: a client, in C, that plays tic-tac-toe against another client through a local or hosted
  server. See `docs/writing-a-client.md`.

Each needs tests and a README. The ruleset's README is the only description of the game's encodings the
client has, so write it before the client depends on it.

## How to work

- Run every test before reporting work as done, and report the commands and their results honestly,
  including failures.
- Keep builds reproducible. `make test` at the repository root must run every test. Give `client/` a
  Makefile with a `test` target, and `make test` runs it too.
- Build with the tools in `scripts/install-deps.sh`, which the `Dockerfile` also uses. If you need
  another system package, add it there, so every machine and the container stay the same. Go modules
  go in `go.mod` as usual; never add the RAMServer module itself (it comes from `sdk/`).
- Make reasonable decisions on your own, and record the significant ones (and why) in the relevant
  README.
- When something is ambiguous in the documentation, choose the reading that follows the protocol most
  closely, and note the ambiguity.
