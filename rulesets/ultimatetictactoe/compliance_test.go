package ultimatetictactoe

import (
	"testing"

	"github.com/stevenlagoy/ramserver-core/server/game/gametest"
)

func TestCompliance(t *testing.T) {
	gametest.Run(t, &Game{}, gametest.Config{Members: []string{"p1", "p2", "watcher1", "watcher2"}})
}

func TestComplianceOtherSeeds(t *testing.T) {
	for _, seed := range []uint64{2, 3, 42} {
		gametest.Run(t, &Game{}, gametest.Config{Members: []string{"a", "b"}, Seed: seed, Matches: 100})
	}
}
