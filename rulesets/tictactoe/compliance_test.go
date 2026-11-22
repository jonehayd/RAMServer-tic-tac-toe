package tictactoe

import (
	"testing"

	"github.com/stevenlagoy/ramserver-core/server/game/gametest"
)

// TestCompliance runs the server's compliance checks with every kind of member the game allows: two
// players and spectators.
func TestCompliance(t *testing.T) {
	gametest.Run(t, &Game{}, gametest.Config{
		Members: []string{"p1", "p2", "watcher1", "watcher2"},
	})
}

// TestComplianceOtherSeeds explores more random matches, with players only.
func TestComplianceOtherSeeds(t *testing.T) {
	for _, seed := range []uint64{2, 3, 42} {
		gametest.Run(t, &Game{}, gametest.Config{Members: []string{"a", "b"}, Seed: seed, Matches: 300})
	}
}
