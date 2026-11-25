#!/usr/bin/env bash
# Integration test: real client processes play complete matches over TCP against tests/fakeserver, a
# stand-in server that runs the real tic-tac-toe ruleset (see its header comment for what it is and
# isn't).
#
#   1. Alice creates a private session; Bob joins it by id; Cy joins as a spectator. Alice and Bob
#      play three matches (each a rematch of the last) with --bot --casual, then leave and quit.
#   2. Dee and Eve find each other with quick match and play one match.
#   3. Uma and Vic play Ultimate Tic-Tac-Toe using the selected game id and two-digit moves.
#   4. A client with --no-guard sends a move out of turn, and shows the server's OUT_OF_TURN error.
#
# Usage: tests/integration.sh BUILD_DIR   (BUILD_DIR holds ttt and fakeserver)
set -euo pipefail

BUILD=${1:-build}
TTT="$BUILD/ttt"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ttt-it.XXXXXX")
SERVER_PID=""

fail() {
    echo "FAIL: $*" >&2
    for f in "$WORK"/*.log; do
        echo "---- $f" >&2
        tail -n 40 "$f" >&2
    done
    exit 1
}

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then kill -TERM "$SERVER_PID" 2>/dev/null || true; fi
    jobs -p | xargs -r kill 2>/dev/null || true
}
trap cleanup EXIT

"$BUILD/fakeserver" -heartbeat 500ms >"$WORK/server.log" 2>&1 &
SERVER_PID=$!
PORT=""
for _ in $(seq 50); do
    PORT=$(sed -n 's/^listening 127\.0\.0\.1://p' "$WORK/server.log")
    [ -n "$PORT" ] && break
    sleep 0.1
done
[ -n "$PORT" ] || fail "the fake server didn't start"
SERVER="127.0.0.1:$PORT"
echo "fake server on $SERVER (logs in $WORK)"

wait_for() { # wait_for FILE PATTERN: prints the first capture group of a sed -n pattern
    local out=""
    for _ in $(seq 100); do
        out=$(sed -n "$2" "$1" | head -n 1)
        [ -n "$out" ] && { echo "$out"; return 0; }
        sleep 0.05
    done
    return 1
}

# ---- 1. private session, join by id, spectator, three matches ----
"$TTT" --plain -s "$SERVER" -n Alice --create --bot --casual --games 3 --delay 120 </dev/null >"$WORK/alice.log" 2>&1 &
ALICE=$!
SID=$(wait_for "$WORK/alice.log" 's/^Joined private session //p') || fail "Alice didn't create a session"
echo "session $SID"
"$TTT" --plain -s "$SERVER" -n Bob --join "$SID" --bot --casual --games 3 --delay 120 </dev/null >"$WORK/bob.log" 2>&1 &
BOB=$!
wait_for "$WORK/bob.log" 's/^\(Joined session\).*/\1/p' >/dev/null || fail "Bob didn't join"
"$TTT" --plain -s "$SERVER" -n Cy --join "$SID" --bot --games 3 </dev/null >"$WORK/cy.log" 2>&1 &
CY=$!

wait "$ALICE" || fail "Alice exited with $?"
wait "$BOB" || fail "Bob exited with $?"
wait "$CY" || fail "Cy exited with $?"

results() { grep -cE '^(You win by|Draw(, by|:)|.* wins by)' "$1" || true; }
[ "$(results "$WORK/alice.log")" = 3 ] || fail "Alice saw $(results "$WORK/alice.log") results, want 3"
[ "$(results "$WORK/bob.log")" = 3 ] || fail "Bob saw $(results "$WORK/bob.log") results, want 3"
[ "$(results "$WORK/cy.log")" = 3 ] || fail "Cy saw $(results "$WORK/cy.log") results, want 3"
grep -q "Match started: you are X" "$WORK/alice.log" || fail "Alice wasn't X"
grep -q "Match started: you are O" "$WORK/bob.log" || fail "Bob wasn't O"
grep -q "Match started: you're watching" "$WORK/cy.log" || fail "Cy wasn't spectating"
grep -q "Cy joined as spectator" "$WORK/alice.log" || fail "Alice didn't see Cy join"
grep -q "You left the session" "$WORK/alice.log" || fail "Alice didn't leave cleanly"
echo "ok: private session, join by id, spectator, 3 matches with rematches, clean leave"

# ---- 2. quick match ----
"$TTT" --plain -s "$SERVER" -n Dee --quick --bot --games 1 --delay 30 </dev/null >"$WORK/dee.log" 2>&1 &
DEE=$!
wait_for "$WORK/dee.log" 's/^\(Joined session\).*/\1/p' >/dev/null || fail "Dee didn't join"
"$TTT" --plain -s "$SERVER" -n Eve --quick --bot --games 1 --delay 30 </dev/null >"$WORK/eve.log" 2>&1 &
EVE=$!
wait "$DEE" || fail "Dee exited with $?"
wait "$EVE" || fail "Eve exited with $?"
[ "$(results "$WORK/dee.log")" = 1 ] || fail "Dee didn't finish a match"
grep -q "Dee joined as player" "$WORK/eve.log" || grep -q "Eve joined as player" "$WORK/dee.log" || fail "Dee and Eve weren't matched"
echo "ok: quick match"

# ---- 3. Ultimate Tic-Tac-Toe ----
"$TTT" --plain -s "$SERVER" -n Uma --game ultimate --create-public --bot --games 1 --delay 5 </dev/null >"$WORK/uma.log" 2>&1 &
UMA=$!
wait_for "$WORK/uma.log" 's/^Joined session //p' >/dev/null || fail "Uma didn't create an Ultimate session"
"$TTT" --plain -s "$SERVER" -n Vic --game ultimate --quick --bot --games 1 --delay 5 </dev/null >"$WORK/vic.log" 2>&1 &
VIC=$!
wait "$UMA" || fail "Uma exited with $?"
wait "$VIC" || fail "Vic exited with $?"
[ "$(results "$WORK/uma.log")" = 1 ] || fail "Uma didn't finish an Ultimate match"
[ "$(results "$WORK/vic.log")" = 1 ] || fail "Vic didn't finish an Ultimate match"
grep -q "Bot plays in board" "$WORK/uma.log" || fail "Uma didn't send an Ultimate board/cell move"
echo "ok: Ultimate game selection and two-digit moves"

# ---- 4. a server error reaches the player ----
# Gil is X and moves first; Fay (O) types a move immediately, out of turn, with the local guard off.
FIFO="$WORK/fay.in"
mkfifo "$FIFO"
"$TTT" --plain -s "$SERVER" -n Gil --create-public --bot --games 1 --delay 1500 </dev/null >"$WORK/gil.log" 2>&1 &
GIL=$!
SID2=$(wait_for "$WORK/gil.log" 's/^Joined session //p') || fail "Gil didn't create a session"
"$TTT" --plain --no-guard -s "$SERVER" -n Fay --join "$SID2" <"$FIFO" >"$WORK/fay.log" 2>&1 &
FAY=$!
exec 3>"$FIFO"
wait_for "$WORK/fay.log" 's/^\(Joined session\).*/\1/p' >/dev/null || fail "Fay didn't join"
echo ready >&3
wait_for "$WORK/fay.log" 's/^\(Match started\).*/\1/p' >/dev/null || fail "Fay's match didn't start"
echo 5 >&3
wait_for "$WORK/fay.log" 's/^\(error: Server: OUT_OF_TURN\).*/\1/p' >/dev/null || fail "Fay didn't see OUT_OF_TURN"
echo resign >&3
wait "$GIL" || fail "Gil exited with $?"
echo quit >&3
exec 3>&-
wait "$FAY" || fail "Fay exited with $?"
grep -q "Gil wins by resignation" "$WORK/fay.log" || fail "Fay's resignation didn't end the match"
echo "ok: server errors are shown, resignation ends the match"

# ---- heartbeats ----
kill -TERM "$SERVER_PID"
wait "$SERVER_PID" 2>/dev/null || true
SERVER_PID=""
STATS=$(grep '^stats' "$WORK/server.log") || fail "no server stats"
echo "server $STATS"
RECEIVED=$(echo "$STATS" | sed -n 's/.*heartbeats_received=\([0-9]*\).*/\1/p')
SENT=$(echo "$STATS" | sed -n 's/.*heartbeats_sent=\([0-9]*\).*/\1/p')
[ "${RECEIVED:-0}" -gt 0 ] || fail "the clients never answered a heartbeat"
# The run is shorter than the 15 s keepalive, so every client heartbeat must be a reply
[ "$RECEIVED" -le "$SENT" ] || fail "clients sent $RECEIVED heartbeats for $SENT received: some weren't replies"
echo "ok: clients answered heartbeats, one reply each"
echo "Integration test passed."
