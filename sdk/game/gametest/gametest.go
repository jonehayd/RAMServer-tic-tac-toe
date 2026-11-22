// Package gametest checks that a game.Game keeps the contract the RAMServer core relies on. Call Run from
// a ruleset's tests:
//
//	func TestCompliance(t *testing.T) {
//		gametest.Run(t, &Game{}, gametest.Config{})
//	}
//
// The checks play many complete random matches through the same calls the server makes, so they also
// catch crashes and matches that never end. Runs are reproducible: the same Config gives the same matches.
package gametest

import (
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/stevenlagoy/ramserver-core/server/game"
)

// Config describes the roster to test with and how much random play to do. Zero values use the defaults.
type Config struct {
	// Members to seat, in join order. Each gets a role the way the server assigns them: the first role
	// in Roles() that still has room. Default: "p1" and "p2".
	Members []string
	// Number of complete random matches to play. Default 200.
	Matches int
	// Most actions one match may take before it counts as never ending. Default 1000.
	MaxActions int
	// Seed for the random choices. Change it to explore other matches; keep it to reproduce a failure.
	Seed uint64
}

func (c Config) withDefaults() Config {
	if len(c.Members) == 0 {
		c.Members = []string{"p1", "p2"}
	}
	if c.Matches == 0 {
		c.Matches = 200
	}
	if c.MaxActions == 0 {
		c.MaxActions = 1000
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
	return c
}

// Run checks g against each part of the contract as its own subtest. Each failure describes the member,
// action, and match that caused it.
func Run(t *testing.T, g game.Game, cfg Config) {
	t.Helper()
	cfg = cfg.withDefaults()
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			for _, problem := range c.run(g, cfg) {
				t.Error(problem)
			}
		})
	}
}

type check struct {
	name string
	run  func(game.Game, Config) []string
}

var checks = []check{
	{"roles", checkRoles},
	{"ready to start", checkReadyToStart},
	{"random matches", checkRandomMatches},
	{"rejected actions", checkRejectedActions},
}

// Stop reporting after this many problems in one check: the first few are the useful ones
const maxProblems = 10

type problems []string

func (p *problems) addf(format string, args ...any) {
	*p = append(*p, fmt.Sprintf(format, args...))
}

func (p *problems) full() bool { return len(*p) >= maxProblems }

// call runs f, recording a problem instead of crashing the test if it panics. Reports whether f returned
func call(p *problems, what string, f func()) (returned bool) {
	defer func() {
		if r := recover(); r != nil {
			p.addf("%s panicked: %v", what, r)
			returned = false
		}
	}()
	f()
	return true
}

// seat assigns roles in join order the way the server does: each member gets the first role in specs
// with room left. Returns the members no role had room for.
func seat(specs []game.RoleSpec, members []string) (map[string]game.Role, []string) {
	roles := make(map[string]game.Role, len(members))
	counts := make(map[game.Role]int)
	var unseated []string
	for _, m := range members {
		seated := false
		for _, s := range specs {
			if s.Max == 0 || counts[s.Name] < s.Max {
				roles[m] = s.Name
				counts[s.Name]++
				seated = true
				break
			}
		}
		if !seated {
			unseated = append(unseated, m)
		}
	}
	return roles, unseated
}

// setup calls Roles and seats cfg.Members. ok is false if the roster can't be tested.
func setup(g game.Game, cfg Config, p *problems) (specs []game.RoleSpec, roles map[string]game.Role, ok bool) {
	if !call(p, "Roles()", func() { specs = g.Roles() }) {
		return nil, nil, false
	}
	roles, unseated := seat(specs, cfg.Members)
	if len(unseated) > 0 {
		p.addf("members %v could not be seated because every role is full; set Config.Members to a roster the game accepts", unseated)
		return nil, nil, false
	}
	return specs, roles, true
}

func checkRoles(g game.Game, cfg Config) []string {
	var p problems
	var specs []game.RoleSpec
	if !call(&p, "Roles()", func() { specs = g.Roles() }) {
		return p
	}
	if len(specs) == 0 {
		p.addf("Roles() returned no roles, so no one could ever join")
	}
	seen := make(map[game.Role]bool)
	for _, s := range specs {
		switch {
		case s.Name == "":
			p.addf("Roles() has a role with an empty name")
		case seen[s.Name]:
			p.addf("Roles() lists role %q more than once", s.Name)
		}
		seen[s.Name] = true
		if s.Min < 0 || s.Max < 0 {
			p.addf("role %q has a negative bound (Min %d, Max %d)", s.Name, s.Min, s.Max)
		}
		if s.Max > 0 && s.Min > s.Max {
			p.addf("role %q requires at least %d members but allows at most %d", s.Name, s.Min, s.Max)
		}
	}
	setup(g, cfg, &p)
	return p
}

func checkReadyToStart(g game.Game, cfg Config) []string {
	var p problems
	specs, roles, ok := setup(g, cfg, &p)
	if !ok {
		return p
	}
	readyToStart := func(what string, roster map[string]game.Role, ready map[string]bool) (start, returned bool) {
		returned = call(&p, what, func() { start = g.ReadyToStart(roster, ready) })
		return start, returned
	}
	allReady := func(roster map[string]game.Role) map[string]bool {
		ready := make(map[string]bool, len(roster))
		for m := range roster {
			ready[m] = true
		}
		return ready
	}

	if start, ok := readyToStart("ReadyToStart with everyone ready", roles, allReady(roles)); ok && !start {
		p.addf("ReadyToStart is false for the roster %v with everyone ready; set Config.Members to a roster that can start", roles)
	}
	if start, ok := readyToStart("ReadyToStart with no one ready", roles, map[string]bool{}); ok && start {
		p.addf("ReadyToStart is true for the roster %v before anyone is ready", roles)
	}
	if start, ok := readyToStart("ReadyToStart with an empty roster", map[string]game.Role{}, map[string]bool{}); ok && start {
		p.addf("ReadyToStart is true for an empty roster")
	}

	// One member short of a role's minimum must not start, however ready everyone is
	for _, s := range specs {
		var holders []string
		for _, m := range cfg.Members {
			if roles[m] == s.Name {
				holders = append(holders, m)
			}
		}
		if s.Min == 0 || len(holders) != s.Min {
			continue
		}
		short := maps.Clone(roles)
		delete(short, holders[len(holders)-1])
		if start, ok := readyToStart("ReadyToStart below a role's minimum", short, allReady(short)); ok && start {
			p.addf("ReadyToStart is true with %d %q member(s), below Roles()'s minimum of %d", s.Min-1, s.Name, s.Min)
		}
	}
	return p
}

// Everything the server can observe about a state. If any of it differs after Validate or Apply, the
// state was modified in place.
type observation struct {
	Active      []string
	Legal       map[string][]game.ActionSpec
	Views       map[string][]byte
	Annotations map[string]map[string]string
	Ended       bool
	Winners     []string
	Reason      game.TerminationReason
}

// observe records everything the server can observe about state, for every member. ok is false if a
// method panicked.
func observe(g game.Game, state game.GameState, members []string, p *problems, where string) (o observation, ok bool) {
	o.Legal = make(map[string][]game.ActionSpec, len(members))
	o.Views = make(map[string][]byte, len(members))
	ok = call(p, "ActiveTurn "+where, func() { o.Active = g.ActiveTurn(state) })
	for _, m := range members {
		ok = call(p, fmt.Sprintf("LegalActions for %s %s", m, where), func() { o.Legal[m] = g.LegalActions(state, m) }) && ok
		ok = call(p, fmt.Sprintf("ViewFor %s %s", m, where), func() { o.Views[m] = g.ViewFor(state, m) }) && ok
	}
	if annotator, isAnnotator := g.(game.Annotator); isAnnotator {
		o.Annotations = make(map[string]map[string]string, len(members))
		for _, m := range members {
			ok = call(p, fmt.Sprintf("Annotations for %s %s", m, where), func() { o.Annotations[m] = annotator.Annotations(state, m) }) && ok
		}
	}
	var result game.MatchResult
	ok = call(p, "Outcome "+where, func() { result, o.Ended = g.Outcome(state) }) && ok
	if o.Ended && result != nil {
		ok = call(p, "the MatchResult's methods "+where, func() {
			o.Winners = result.GetWinnerIDs()
			o.Reason = result.TerminationReason()
		}) && ok
	}
	if o.Ended && result == nil {
		p.addf("Outcome reported a finished match with a nil MatchResult %s", where)
		ok = false
	}
	return o, ok
}

// validate calls Validate, recording a panic or an error that doesn't wrap either game error.
func validate(g game.Game, state game.GameState, action game.Action, p *problems, where string) (err error, returned bool) {
	returned = call(p, fmt.Sprintf("Validate(%q from %s) %s", action.Payload, action.ActorID, where), func() {
		err = g.Validate(state, action)
	})
	if err != nil && !errors.Is(err, game.ErrIllegalAction) && !errors.Is(err, game.ErrMalformedAction) {
		p.addf("Validate(%q from %s) %s returned %q, which wraps neither game.ErrIllegalAction nor game.ErrMalformedAction",
			action.Payload, action.ActorID, where, err)
	}
	return err, returned
}

func allowsOutOfTurn(g game.Game, state game.GameState, action game.Action) bool {
	o, ok := g.(game.OutOfTurnActions)
	return ok && o.AllowsOutOfTurn(state, action)
}

func listed(specs []game.ActionSpec, payload []byte) bool {
	return slices.ContainsFunc(specs, func(s game.ActionSpec) bool { return string(s.Payload) == string(payload) })
}

func checkRandomMatches(g game.Game, cfg Config) []string {
	var p problems
	_, roles, ok := setup(g, cfg, &p)
	if !ok {
		return p
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, 0))
	offered := make(map[string]bool) // every payload LegalActions has offered anyone, in any match so far
	for match := 1; match <= cfg.Matches && !p.full(); match++ {
		playMatch(g, cfg, roles, rng, match, offered, &p)
	}
	return p
}

// playMatch plays one match of random legal actions from NewState to an outcome, checking every step.
// offered collects every payload seen in LegalActions, so later states can check Validate rejects
// payloads that aren't legal for them. Payloads a game never offers can't be guessed, so they aren't tried.
func playMatch(g game.Game, cfg Config, roles map[string]game.Role, rng *rand.Rand, match int, offered map[string]bool, p *problems) {
	members := cfg.Members
	var state game.GameState
	if !call(p, fmt.Sprintf("NewState in match %d", match), func() { state = g.NewState(slices.Clone(members), maps.Clone(roles)) }) {
		return
	}
	for step := 0; !p.full(); step++ {
		where := fmt.Sprintf("in match %d after %d actions", match, step)
		before, ok := observe(g, state, members, p, where)
		if !ok {
			return
		}
		if again, _ := observe(g, state, members, p, where); !reflect.DeepEqual(before, again) {
			p.addf("observing the same state twice gave different results %s; methods must not change the state or depend on anything outside it", where)
			return
		}
		if before.Active == nil {
			p.addf("ActiveTurn returned nil %s; return an empty slice when no one may act", where)
		}
		if before.Ended {
			if len(before.Active) > 0 {
				p.addf("ActiveTurn still lists %v %s, after Outcome reported the match finished", before.Active, where)
			}
			for _, w := range before.Winners {
				if _, member := roles[w]; !member {
					p.addf("the MatchResult %s names winner %q, who is not a member", where, w)
				}
			}
			if before.Reason == "" {
				p.addf("the MatchResult %s has an empty TerminationReason; clients receive it as termination_reason", where)
			}
			return
		}
		if step == cfg.MaxActions {
			p.addf("match %d did not end after %d actions", match, cfg.MaxActions)
			return
		}
		for _, m := range members {
			for _, spec := range before.Legal[m] {
				offered[string(spec.Payload)] = true
			}
		}

		// Every listed action must validate, and must be one the server would let through its turn check
		for _, m := range members {
			for _, spec := range before.Legal[m] {
				action := game.Action{ActorID: m, Payload: spec.Payload}
				if !slices.Contains(before.Active, m) && !allowsOutOfTurn(g, state, action) {
					p.addf("LegalActions offers %s the action %q %s, but it isn't their turn and the game doesn't allow it out of turn", m, spec.Payload, where)
				}
				if err, ok := validate(g, state, action, p, where); ok && err != nil {
					p.addf("Validate rejects %q from %s %s, which LegalActions offered: %v", spec.Payload, m, where, err)
				}
			}
		}
		// Validate must accept nothing LegalActions doesn't list, for any member who could send it
		for _, m := range members {
			for _, payload := range slices.Sorted(maps.Keys(offered)) {
				action := game.Action{ActorID: m, Payload: []byte(payload)}
				if listed(before.Legal[m], action.Payload) || (!slices.Contains(before.Active, m) && !allowsOutOfTurn(g, state, action)) {
					continue
				}
				if err, ok := validate(g, state, action, p, where); ok && err == nil {
					p.addf("Validate accepts %q from %s %s, but LegalActions doesn't offer it to them", payload, m, where)
				}
			}
		}
		if after, _ := observe(g, state, members, p, where); !reflect.DeepEqual(before, after) {
			p.addf("Validate changed the state %s; it must not modify the state it is given", where)
			return
		}

		var choices []game.Action
		for _, m := range before.Active {
			for _, spec := range before.Legal[m] {
				choices = append(choices, game.Action{ID: uint64(step + 1), ActorID: m, Payload: spec.Payload})
			}
		}
		if len(choices) == 0 {
			p.addf("no active member has a legal action %s, but Outcome says the match isn't over, so it can never finish", where)
			return
		}
		action := choices[rng.IntN(len(choices))]

		var next, replay game.GameState
		if !call(p, fmt.Sprintf("Apply(%q from %s) %s", action.Payload, action.ActorID, where), func() {
			next = g.Apply(state, action)
			replay = g.Apply(state, action)
		}) {
			return
		}
		if after, _ := observe(g, state, members, p, where); !reflect.DeepEqual(before, after) {
			p.addf("Apply(%q from %s) changed the state it was given %s; it must return a new state instead", action.Payload, action.ActorID, where)
			return
		}
		nextWhere := fmt.Sprintf("in match %d after %d actions", match, step+1)
		first, ok1 := observe(g, next, members, p, nextWhere)
		second, ok2 := observe(g, replay, members, p, nextWhere)
		if ok1 && ok2 && !reflect.DeepEqual(first, second) {
			p.addf("Apply(%q from %s) %s gave different states when called twice; it must be deterministic", action.Payload, action.ActorID, where)
			return
		}
		state = next
	}
}

func checkRejectedActions(g game.Game, cfg Config) []string {
	var p problems
	_, roles, ok := setup(g, cfg, &p)
	if !ok {
		return p
	}
	var state game.GameState
	if !call(&p, "NewState", func() { state = g.NewState(slices.Clone(cfg.Members), maps.Clone(roles)) }) {
		return p
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, 1))
	junk := [][]byte{nil, {}, []byte("not an action"), make([]byte, 70<<10)}
	for range 20 {
		b := make([]byte, 1+rng.IntN(16))
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		junk = append(junk, b)
	}
	for _, m := range cfg.Members {
		for _, payload := range junk {
			action := game.Action{ActorID: m, Payload: payload}
			err, ok := validate(g, state, action, &p, "at the start of a match")
			if ok && err == nil && (len(payload) == 0 || len(payload) > 64<<10) {
				p.addf("Validate accepts a %d-byte payload from %s; empty and oversized payloads should be rejected", len(payload), m)
			}
		}
	}
	return p
}
