package press

import (
	"fmt"
)

func RestorePhases(state *State, plate string, generation uint64, phases map[string]float64) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if plate == "" || generation == 0 {
		return fmt.Errorf("plate recovery identity is incomplete")
	}
	state.plate = plate
	state.plateGeneration = generation
	state.phaseByUnit = map[string]float64{}
	for unit, phase := range phases {
		state.phaseByUnit[unit] = phase
	}
	return nil
}
