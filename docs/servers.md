# Servers

You develop against two servers: a local one you run yourself, and the hosted one. Both run the same
server code, with the same protocol.

## Getting tic-tac-toe onto a server

You write the ruleset in this repository, but you cannot run it inside a server yourself: the server's
code is not part of this repository. Integration is done by the server team:

1. You finish the ruleset with passing tests and a README describing its encodings.
2. The server team adds it to the server and registers it as `tictactoe`.
3. They provide a local server build in `server/` in this repository, and deploy the hosted server.

Until then, a server answers `JoinSessionRequest` for `tictactoe` with `ERROR_CODE_UNKNOWN_GAME`. Test the
ruleset with its own unit tests and the compliance tests in the meantime.

## Local server

The server team provides a build of the server, with tic-tac-toe registered, in `server/`; see
`server/README.md` for how to start it on your platform. It has two modes:

| Mode | Started with | Connect to |
| --- | --- | --- |
| TCP | `RAMSERVER_ADDR=127.0.0.1:8080` (the default) | `127.0.0.1:8080` |
| WebSocket | `PORT=8081` (any free port) | `ws://127.0.0.1:8081/connect` |

Use TCP mode for quick testing, and WebSocket mode to test the transport your client uses for the hosted
server without depending on it.

A local server keeps everything in memory: restarting it loses all sessions and players.

## Hosted server

`wss://ramserver-core-80752473398.us-east5.run.app/connect`

- Check it is up with `GET https://ramserver-core-80752473398.us-east5.run.app/`.
- Other people may be using it at the same time. Use private sessions (`create_new` with
  `private_session`) for tests, so quick match doesn't pair your test clients with strangers.
- The first connection after a quiet period can take several seconds.
- It is redeployed from time to time, which ends every connection and session. A client should handle
  this like any dropped connection.

## Playing two clients against each other

Start two client instances against the same server. Have one create a private session and the other
join it by `session_id`, or have both quick-match into `tictactoe` on a local server with no one else on
it. Both ready up, and the match starts.
