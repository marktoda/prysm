package stategen

import (
	"context"

	"github.com/prysmaticlabs/prysm/beacon-chain/state"
	"go.opencensus.io/trace"
)

// SaveState saves the state in the DB.
// It knows which cold and hot state section the input state should belong to.
func (s *State) SaveState(ctx context.Context, root [32]byte, state *state.BeaconState) error {
	ctx, span := trace.StartSpan(ctx, "stateGen.SaveState")
	defer span.End()

	// The state belongs to the cold section if it's below the split slot threshold.
	if state.Slot() < s.splitInfo.slot {
		return s.saveColdState(ctx, root, state)
	}

	return s.saveHotState(ctx, root, state)
}
