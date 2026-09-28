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
	correction := -offset
	if correction > controller.maximum {
		correction = controller.maximum
	}
	if correction < -controller.maximum {
		correction = -controller.maximum
	}
	if err := controller.state.SetPhase(unit, correction, plateGeneration); err != nil {
		return 0, err
	}
	controller.mu.Lock()
	controller.commands[unit] = correction
	controller.mu.Unlock()
	return correction, nil
}

func (controller *PhaseController) Last(unit string) (float64, bool) {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	value, ok := controller.commands[unit]
	return value, ok
}

// Maximum returns the per-unit phase travel limit the controller clamps to.
func (controller *PhaseController) Maximum() float64 {
	controller.mu.RLock()
	defer controller.mu.RUnlock()
	return controller.maximum
}
