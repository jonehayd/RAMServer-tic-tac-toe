package gametest

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stevenlagoy/ramserver-core/server/game"
)

// nim is a small correct game for testing the checks: players take 1 or 2 from a pile of 7, and whoever
// takes the last one wins. Spectators may watch. The pile is a slice so the broken variants below have
// something shared to modify.
type nim struct{}

type nimState struct {
	players [2]string
	pile    []int // one element: the count left
	turn    int   // index into players
	winner  string
}

func (nim) Roles() []game.RoleSpec {
	return []game.RoleSpec{{Name: game.RolePlayer, Min: 2, Max: 2}, {Name: game.RoleSpectator}}
}

func (nim) ReadyToStart(roster map[string]game.Role, ready map[string]bool) bool {
	players := 0
	for id, role := range roster {
		if role == game.RolePlayer {
			if !ready[id] {
				return false
			}
			players++
		}
	}
	return players == 2
}

func (nim) NewState(order []string, roles map[string]game.Role) game.GameState {
	var s nimState
	seat := 0
	for _, id := range order {
		if roles[id] == game.RolePlayer && seat < 2 {
			s.players[seat] = id
			seat++
		}
	}
	s.pile = []int{7}
	return s
}

func (nim) ActiveTurn(st game.GameState) []string {
	s := st.(nimState)
	if s.winner != "" {
		return []string{}
	}
	return []string{s.players[s.turn]}
}

func (g nim) LegalActions(st game.GameState, id string) []game.ActionSpec {
	s := st.(nimState)
	if s.winner != "" || s.players[s.turn] != id {
		return nil
	}
	var specs []game.ActionSpec
	for take := 1; take <= 2 && take <= s.pile[0]; take++ {
		specs = append(specs, game.ActionSpec{Payload: []byte(strconv.Itoa(take)), Description: fmt.Sprintf("take %d", take)})
	}
	return specs
}

func (nim) Validate(st game.GameState, a game.Action) error {
	s := st.(nimState)
	take, err := strconv.Atoi(string(a.Payload))
	if err != nil || take < 1 || take > 2 {
		return fmt.Errorf("%w: send 1 or 2", game.ErrMalformedAction)
	}
	switch {
	case s.winner != "":
		return fmt.Errorf("%w: the game is over", game.ErrIllegalAction)
	case s.players[s.turn] != a.ActorID:
		return fmt.Errorf("%w: not your turn", game.ErrIllegalAction)
	case take > s.pile[0]:
		return fmt.Errorf("%w: only %d left", game.ErrIllegalAction, s.pile[0])
	}
	return nil
}

func (nim) Apply(st game.GameState, a game.Action) game.GameState {
	s := st.(nimState)
	take, _ := strconv.Atoi(string(a.Payload))
	s.pile = []int{s.pile[0] - take} // a new slice: the old state keeps its pile
	if s.pile[0] == 0 {
		s.winner = a.ActorID
	}
	s.turn = 1 - s.turn
	return s
}

func (nim) Outcome(st game.GameState) (game.MatchResult, bool) {
	s := st.(nimState)
	if s.winner == "" {
		return nil, false
	}
	return game.Result{WinnerIDs: []string{s.winner}, Reason: "LAST_TAKEN"}, true
}

func (nim) ViewFor(st game.GameState, _ string) []byte {
	return []byte(strconv.Itoa(st.(nimState).pile[0]))
}

// problemsOf runs every check and returns all the problems found
func problemsOf(g game.Game, cfg Config) []string {
	cfg = cfg.withDefaults()
	var all []string
	for _, c := range checks {
		all = append(all, c.run(g, cfg)...)
	}
	return all
}

func TestCorrectGamePasses(t *testing.T) {
	Run(t, nim{}, Config{Members: []string{"alice", "bob", "watcher"}})
}

// Each broken game breaks one rule, and the checks must report it
type mutatingApply struct{ nim }

func (mutatingApply) Apply(st game.GameState, a game.Action) game.GameState {
	s := st.(nimState)
	take, _ := strconv.Atoi(string(a.Payload))
	s.pile[0] -= take // writes through the slice the old state shares
	if s.pile[0] == 0 {
		s.winner = a.ActorID
	}
	s.turn = 1 - s.turn
	return s
}

type mutatingValidate struct{ nim }

func (g mutatingValidate) Validate(st game.GameState, a game.Action) error {
	st.(nimState).pile[0]++
	return g.nim.Validate(st, a)
}

type nilActiveTurn struct{ nim }

func (nilActiveTurn) ActiveTurn(st game.GameState) []string {
	if s := st.(nimState); s.winner == "" {
		return []string{s.players[s.turn]}
	}
	return nil
}

type neverEnds struct{ nim }

func (neverEnds) Apply(st game.GameState, _ game.Action) game.GameState {
	s := st.(nimState)
	s.turn = 1 - s.turn
	return s // the pile never shrinks
}

type unwrappedErrors struct{ nim }

func (g unwrappedErrors) Validate(st game.GameState, a game.Action) error {
	if err := g.nim.Validate(st, a); err != nil {
		return errors.New("no")
	}
	return nil
}

type offersIllegal struct{ nim }

func (g offersIllegal) LegalActions(st game.GameState, id string) []game.ActionSpec {
	specs := g.nim.LegalActions(st, id)
	if len(specs) > 0 {
		specs = append(specs, game.ActionSpec{Payload: []byte("3")})
	}
	return specs
}

type acceptsUnlisted struct{ nim }

// Offers "2" only when the pile is even, but Validate accepts it whenever two are left
func (g acceptsUnlisted) LegalActions(st game.GameState, id string) []game.ActionSpec {
	specs := g.nim.LegalActions(st, id)
	if len(specs) > 1 && st.(nimState).pile[0]%2 == 1 {
		specs = specs[:1]
	}
	return specs
}

type startsUnready struct{ nim }

func (startsUnready) ReadyToStart(map[string]game.Role, map[string]bool) bool { return true }

type ignoresMinimum struct{ nim }

func (ignoresMinimum) ReadyToStart(roster map[string]game.Role, ready map[string]bool) bool {
	return len(ready) > 0 && !slices.Contains(slices.Collect(func(yield func(bool) bool) {
		for id := range roster {
			if !yield(ready[id]) {
				return
			}
		}
	}), false)
}

type panickingView struct{ nim }

func (panickingView) ViewFor(game.GameState, string) []byte { panic("view broke") }

type acceptsEmpty struct{ nim }

func (g acceptsEmpty) Validate(st game.GameState, a game.Action) error {
	if len(a.Payload) == 0 {
		return nil
	}
	return g.nim.Validate(st, a)
}

type noReason struct{ nim }

func (g noReason) Outcome(st game.GameState) (game.MatchResult, bool) {
	if res, ended := g.nim.Outcome(st); ended {
		return game.Result{WinnerIDs: res.GetWinnerIDs()}, true
	}
	return nil, false
}

type seatsOne struct{ nim }

func (seatsOne) Roles() []game.RoleSpec {
	return []game.RoleSpec{{Name: game.RolePlayer, Min: 2, Max: 1}}
}

func TestBrokenGamesAreCaught(t *testing.T) {
	tests := []struct {
		name string
		game game.Game
		want string // part of the expected problem
	}{
		{"Apply modifies its input", mutatingApply{}, "changed the state it was given"},
		{"Validate modifies its input", mutatingValidate{}, "Validate changed the state"},
		{"ActiveTurn returns nil", nilActiveTurn{}, "ActiveTurn returned nil"},
		{"the match never ends", neverEnds{}, "did not end"},
		{"errors wrap neither game error", unwrappedErrors{}, "wraps neither"},
		{"LegalActions offers an illegal action", offersIllegal{}, "which LegalActions offered"},
		{"Validate accepts an unlisted action", acceptsUnlisted{}, "doesn't offer it"},
		{"ReadyToStart ignores readiness", startsUnready{}, "before anyone is ready"},
		{"ReadyToStart ignores a role minimum", ignoresMinimum{}, "below Roles()'s minimum"},
		{"a method panics", panickingView{}, "panicked"},
		{"an empty payload is accepted", acceptsEmpty{}, "should be rejected"},
		{"a result has no reason", noReason{}, "empty TerminationReason"},
		{"impossible role bounds", seatsOne{}, "requires at least 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := problemsOf(tt.game, Config{Matches: 20, MaxActions: 50})
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tt.want) }) {
				t.Fatalf("expected a problem containing %q, got %q", tt.want, problems)
			}
		})
	}
}

func TestDefaultRosterMustBeSeatable(t *testing.T) {
	problems := problemsOf(nim{}, Config{Members: []string{"a", "b", "c"}}) // spectator "c" is fine
	if len(problems) != 0 {
		t.Fatalf("expected no problems, got %q", problems)
	}
	problems = problemsOf(seatsOne{}, Config{})
	if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "could not be seated") }) {
		t.Fatalf("expected an unseatable roster to be reported, got %q", problems)
	}
}

func TestRunsAreReproducible(t *testing.T) {
	cfg := Config{Matches: 30, Seed: 7}
	if a, b := problemsOf(neverEnds{}, cfg), problemsOf(neverEnds{}, cfg); !slices.Equal(a, b) {
		t.Fatalf("the same Config gave different results:\n%q\n%q", a, b)
	}
}
