package ultimatetictactoe

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/stevenlagoy/ramserver-core/server/game"
)

const (
	alice   = "alice"
	bob     = "bob"
	watcher = "watcher"
)

var g = &Game{}

func newState() State {
	return g.NewState([]string{alice, bob, watcher}, map[string]game.Role{
		alice: game.RolePlayer, bob: game.RolePlayer, watcher: game.RoleSpectator,
	}).(State)
}

func action(actor, payload string) game.Action {
	return game.Action{ActorID: actor, Payload: []byte(payload)}
}

func apply(t *testing.T, state game.GameState, actor, payload string) State {
	t.Helper()
	a := action(actor, payload)
	if err := g.Validate(state, a); err != nil {
		t.Fatalf("Validate(%s, %q) = %v", actor, payload, err)
	}
	return g.Apply(state, a).(State)
}

func TestNewStateAndViewEncoding(t *testing.T) {
	s := newState()
	if s.Players != [2]string{alice, bob} || s.Turn != seatX || s.NextBoard != noBoard {
		t.Fatalf("initial seats/turn/board = %v %d %d", s.Players, s.Turn, s.NextBoard)
	}
	if len(g.ViewFor(s, alice)) != ViewLen {
		t.Fatalf("view length = %d, want %d", len(g.ViewFor(s, alice)), ViewLen)
	}
	want := make([]byte, ViewLen)
	for i := 0; i < 91; i++ {
		want[i] = '.'
	}
	want[91], want[92] = 'X', 'X'
	if got := g.ViewFor(s, alice); !reflect.DeepEqual(got, want) {
		t.Fatalf("initial X view = %q, want %q", got, want)
	}
	view := g.ViewFor(s, watcher)
	if view[91] != 'X' || view[92] != 'S' {
		t.Fatalf("spectator tail = %q, want XS", view[91:])
	}
}

func TestActionsEnforceNextBoardAndCopyState(t *testing.T) {
	s := newState()
	if got := len(g.LegalActions(s, alice)); got != 82 {
		t.Fatalf("initial legal action count = %d, want 81 moves plus resign", got)
	}
	first := apply(t, s, alice, "48")
	if s.Boards[4][8] != cellEmpty || first.Boards[4][8] != markX {
		t.Fatal("Apply mutated its input or failed to place the mark")
	}
	if first.NextBoard != 8 || first.Turn != seatO {
		t.Fatalf("after 48: NextBoard=%d Turn=%d, want 8 and O", first.NextBoard, first.Turn)
	}
	legal := g.LegalActions(first, bob)
	if len(legal) != 10 || string(legal[0].Payload) != "80" || string(legal[8].Payload) != "88" {
		t.Fatalf("restricted legal actions = %v, want 80..88 and resign", legal)
	}
	if err := g.Validate(first, action(bob, "00")); !errors.Is(err, game.ErrIllegalAction) {
		t.Fatalf("off-board move error = %v, want ErrIllegalAction", err)
	}
	if err := g.Validate(first, action(alice, "80")); !errors.Is(err, game.ErrIllegalAction) {
		t.Fatalf("out-of-turn move error = %v, want ErrIllegalAction", err)
	}
	if err := g.Validate(first, action(bob, "8")); !errors.Is(err, game.ErrMalformedAction) {
		t.Fatalf("malformed move error = %v, want ErrMalformedAction", err)
	}
}

func TestLocalWinMakesResolvedTargetFreeChoice(t *testing.T) {
	s := newState()
	s.NextBoard = 0
	s.Boards[0] = [9]byte{'X', 'X', '.', 'O', 'O', '.', '.', '.', '.'}
	s.LocalStatus[2] = localDraw
	next := apply(t, s, alice, "02")
	if next.LocalStatus[0] != markX {
		t.Fatalf("local board status = %q, want X", next.LocalStatus[0])
	}
	if next.NextBoard != noBoard {
		t.Fatalf("next board = %d, want free choice because board 2 is drawn", next.NextBoard)
	}
	if next.Over || next.Turn != seatO {
		t.Fatalf("match ended or wrong turn after local win: over=%v turn=%d", next.Over, next.Turn)
	}
	legal := g.LegalActions(next, bob)
	if len(legal) != 64 { // seven unfinished boards, 63 cells, and resignation
		t.Fatalf("free-choice legal action count = %d, want 64", len(legal))
	}
	for _, spec := range legal[:len(legal)-1] {
		if spec.Payload[0] == '0' || spec.Payload[0] == '2' {
			t.Fatalf("finished board included in legal actions: %q", spec.Payload)
		}
	}
}

func TestGlobalWinAndWinningLine(t *testing.T) {
	s := newState()
	s.NextBoard = 2
	s.LocalStatus[0], s.LocalStatus[1] = markX, markX
	s.Boards[2] = [9]byte{'X', 'X', '.', 'O', 'O', '.', '.', '.', '.'}
	ended := apply(t, s, alice, "22")
	if !ended.Over || ended.Winner != seatX || ended.Reason != ReasonThreeInARow || ended.NextBoard != noBoard {
		t.Fatalf("outcome = over %v winner %d reason %q next board %d", ended.Over, ended.Winner, ended.Reason, ended.NextBoard)
	}
	if !slices.Equal(ended.Line[:], []int{0, 1, 2}) {
		t.Fatalf("winning line = %v, want [0 1 2]", ended.Line)
	}
	result, ok := g.Outcome(ended)
	if !ok || !slices.Equal(result.GetWinnerIDs(), []string{alice}) || result.TerminationReason() != ReasonThreeInARow {
		t.Fatalf("Outcome = %#v, %v", result, ok)
	}
}

func TestDrawnLocalBoardAndFullMatchDraw(t *testing.T) {
	s := newState()
	s.Turn, s.NextBoard = seatX, 1
	s.Boards[1] = [9]byte{'X', 'O', 'X', 'X', 'O', 'O', 'O', 'X', '.'}
	s.LocalStatus[8] = localDraw
	partial := apply(t, s, alice, "18")
	if partial.LocalStatus[1] != localDraw || partial.NextBoard != noBoard || partial.Over {
		t.Fatalf("drawn local board state = status %q next %d over %v", partial.LocalStatus[1], partial.NextBoard, partial.Over)
	}
	if err := g.Validate(partial, action(bob, "10")); !errors.Is(err, game.ErrIllegalAction) {
		t.Fatalf("move in drawn board error = %v, want ErrIllegalAction", err)
	}

	s = newState()
	s.Turn, s.NextBoard = seatX, 1
	for board := range s.LocalStatus {
		s.LocalStatus[board] = localDraw
	}
	s.LocalStatus[1] = localOpen
	s.Boards[1] = [9]byte{'X', 'O', 'X', 'X', 'O', 'O', 'O', 'X', '.'}
	ended := apply(t, s, alice, "18")
	if !ended.Over || ended.Reason != ReasonDraw || ended.Winner != noSeat || ended.NextBoard != noBoard {
		t.Fatalf("full match outcome = over %v winner %d reason %q next board %d", ended.Over, ended.Winner, ended.Reason, ended.NextBoard)
	}
	if result, ok := g.Outcome(ended); !ok || len(result.GetWinnerIDs()) != 0 {
		t.Fatalf("draw Outcome = %#v, %v", result, ok)
	}
}

func TestResignationAndSpectatorRestrictions(t *testing.T) {
	s := newState()
	if !g.AllowsOutOfTurn(s, action(bob, ResignPayload)) {
		t.Fatal("resignation should be allowed out of turn")
	}
	if err := g.Validate(s, action(bob, ResignPayload)); err != nil {
		t.Fatalf("out-of-turn resignation rejected: %v", err)
	}
	ended := g.Apply(s, action(bob, ResignPayload)).(State)
	if !ended.Over || ended.Winner != seatX || ended.Reason != ReasonResignation {
		t.Fatalf("resignation result = %+v", ended)
	}
	if err := g.Validate(s, action(watcher, "00")); !errors.Is(err, game.ErrIllegalAction) {
		t.Fatalf("spectator move error = %v, want ErrIllegalAction", err)
	}
}

func TestMalformedAndNotEnoughPlayers(t *testing.T) {
	s := newState()
	for _, payload := range []string{"", "0", "000", "99", " 0", "RESIGN"} {
		if err := g.Validate(s, action(alice, payload)); !errors.Is(err, game.ErrMalformedAction) {
			t.Errorf("Validate(%q) = %v, want ErrMalformedAction", payload, err)
		}
	}
	ended := g.NewState([]string{alice}, map[string]game.Role{alice: game.RolePlayer}).(State)
	if result, ok := g.Outcome(ended); !ok || result.TerminationReason() != ReasonNotEnoughPlayers {
		t.Fatalf("one-player state outcome = %#v, %v", result, ok)
	}
}
