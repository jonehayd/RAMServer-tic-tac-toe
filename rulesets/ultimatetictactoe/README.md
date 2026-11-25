# Ultimate Tic-Tac-Toe ruleset

Package `ultimatetictactoe`, type `Game`, implements the public `game.Game` interface and the optional
`game.OutOfTurnActions` and `game.Annotator` interfaces. It is a separate game from standard
tic-tac-toe; clients select it with the `game_id` `ultimate_tictactoe`.

This file is the complete description of the game's wire encodings. A client needs nothing else from
this package.

| | |
| --- | --- |
| `game_id` | `ultimate_tictactoe` (the constant `ultimatetictactoe.Name`) |
| Roles | `player`: exactly 2. `spectator`: any number, including none. |
| Start condition | Exactly two players are in the session and both have sent `SetReadyRequest{ready: true}`. |
| `encoded_action` | Two ASCII digits `00`–`88` (board then cell), or the 6 ASCII bytes `resign`. |
| `encoded_view` | Exactly 93 ASCII bytes: 81 local cells, 9 local-board statuses, next-board constraint, match status, recipient's mark. |
| `termination_reason` | `THREE_IN_A_ROW`, `DRAW`, `RESIGNATION` (and `NOT_ENOUGH_PLAYERS`, which shouldn't occur). |

Integration, for the server team:

```go
games.Register(ultimatetictactoe.Name, func() game.Game { return &ultimatetictactoe.Game{} })
```

## Rules

- The game is a 3×3 global board made of nine 3×3 local boards. Both boards and cells are numbered
  row-major from the top-left, `0` through `8`.
- X moves first. The first player to join is X; the second is O. Any additional members spectate.
- A move places a mark in a cell of a local board. The cell number selects which local board the
  opponent must play in next.
- The first player to make three of their marks in a row, column, or diagonal in a local board wins
  that board. A won board cannot be played in again.
- If a local board fills without a line, that board is drawn and cannot be played in again. A draw does
  not count for either player on the global board.
- If the selected next board is already won or drawn, the next player may choose any unfinished local
  board. The same applies to the first move.
- A player who wins three local boards in a row, column, or diagonal wins the match
  (`THREE_IN_A_ROW`). If all nine local boards are won or drawn before either player wins globally,
  the match is a draw (`DRAW`).
- Either player may resign at any time while the match is running, including out of turn. Spectators
  cannot play or resign.

The source instruction sheet does not specify how a full local board without a line is handled. This
ruleset treats it as a drawn, unavailable local board; this is the conventional interpretation needed
for all legal positions to have a defined next move.

## `encoded_action`

An action is exactly two ASCII digits: the first is the local-board number and the second is the cell
number within that board. For example, `48` places a mark in cell 8 of local board 4. Both numbers use
the numbering shown below. `resign` is the exact six-byte resignation payload. No terminator,
whitespace, or length prefix is included. Other payloads are malformed.

After a move to cell `n`, the opponent is sent to local board `n` if that board is unfinished. If it is
already won or drawn, the opponent may select any unfinished local board.

For `LegalActionsResponse`, the player to move receives every legal two-digit board/cell action in
board-major, then cell-major order, followed by `resign`. The other player receives only `resign`; a
spectator or finished match receives no actions.

## `encoded_view`

Every in-match view is exactly 93 ASCII bytes, with no separators or terminator:

| Byte offsets | Length | Contents |
| --- | ---: | --- |
| 0–80 | 81 | Local cells. Nine consecutive groups of 9, one per local board in global row-major order; each group is row-major. Each cell is `X`, `O`, or `.`. |
| 81–89 | 9 | Local-board outcomes in global row-major order: `.` unfinished, `X` won by X, `O` won by O, `D` drawn. |
| 90 | 1 | Next-board constraint: `0`–`8` means that local board is required; `.` means there is no forced board (including after the match ends). |
| 91 | 1 | Match status: `X` or `O` is the mark to move; `x` or `o` is the winner; `D` is a match draw; `-` is an ended match without a winner or draw. |
| 92 | 1 | Recipient: `X`, `O`, or `S` for spectator. |

Local-board and cell numbering are both:

```text
0 | 1 | 2
---+---+---
3 | 4 | 5
---+---+---
6 | 7 | 8
```

For instance, an initial view starts with 81 dots, then nine dots, a dot for unrestricted play, `X`
for X to move, and the recipient mark: 93 bytes total.

## Annotations

`StateUpdate.annotations` includes `x_player`, `o_player`, and `moves`. `last_move` is
`<board>,<cell>` for the latest move, `winning_line` is the three global board indices for a match win,
and `resigned` is `X` or `O` when applicable. `local_status` contains the nine local-board status bytes
in global row-major order and `next_board` is `0`–`8` or `.`; these duplicate the encoded view for
clients that prefer named metadata.

## Validation errors

Malformed payloads return `game.ErrMalformedAction` with a message describing the required two digits or
`resign` payload. A well-formed action that is not legal in the current state returns
`game.ErrIllegalAction`; its message identifies the relevant condition, such as the wrong turn, a
required board, a finished board, an occupied cell, a spectator attempting to play, or a finished match.
