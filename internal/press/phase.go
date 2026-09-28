package press

import (
	"fmt"
	"math"
	"sync"
)

type PhaseController struct {
	mu       sync.RWMutex
	state    *State
	maximum  float64
	commands map[string]float64
}

func NewPhaseController(state *State, maximum float64) *PhaseController {
	return &PhaseController{state: state, maximum: maximum, commands: map[string]float64{}}
}

func (controller *PhaseController) Correct(unit string, offset float64, plateGeneration uint64) (float64, error) {
	if math.IsNaN(offset) || math.IsInf(offset, 0) || controller.maximum <= 0 {
		return 0, fmt.Errorf("phase correction input is invalid")
	}
	correction, _ := limitCorrection(offset, controller.maximum)
	if err := controller.state.SetPhase(unit, correction, plateGeneration); err != nil {
		return 0, err
	}
	controller.mu.Lock()
	controller.commands[unit] = correction
	controller.mu.Unlock()
	return correction, nil
}

// limitCorrection converts an observed offset into a correction bounded by
// the unit travel limit, reporting whether the limit was reached.
func limitCorrection(offset, maximum float64) (float64, bool) {
	correction := -offset
	switch {
	case correction > maximum:
		return maximum, true
	case correction < -maximum:
		return -maximum, true
	default:
		return correction, false
	}
}

func (controller *PhaseController) Last(unit string) (float64, bool) {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	value, ok := controller.commands[unit]
	return value, ok
}
