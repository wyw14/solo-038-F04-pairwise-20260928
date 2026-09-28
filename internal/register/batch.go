package register

import (
	"fmt"
	"strings"
)

// CorrectionBatch tracks the per-unit acknowledgement state of one joint
// correction for a single plate generation.
type CorrectionBatch struct {
	PlateGeneration uint64
	Plan            JointCorrection
	Acknowledged    map[string]bool
}

// BeginJointCorrection plans a travel-limited joint correction for the active
// window and opens a confirmation batch. A plan may be partial when some
// colors lack observations; the missing colors are carried on the plan and
// must be surfaced by the caller rather than treated as zero error.
func (coordinator *Coordinator) BeginJointCorrection(plateGeneration uint64, travelLimit float64) (JointCorrection, error) {
	window := coordinator.state.Snapshot()
	plan, err := PlanJointCorrection(window, plateGeneration, travelLimit)
	if err != nil {
		return JointCorrection{}, err
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.batch = &CorrectionBatch{
		PlateGeneration: plateGeneration,
		Plan:            plan,
		Acknowledged:    map[string]bool{},
	}
	return plan, nil
}

// AcknowledgeUnit records a press unit's confirmation of the correction it was
// commanded to apply. Unknown units, stale plate generations and units without
// an observation in the plan are rejected.
func (coordinator *Coordinator) AcknowledgeUnit(plateGeneration uint64, color string, applied float64) error {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	batch := coordinator.batch
	if batch == nil {
		return fmt.Errorf("no joint correction batch is open")
	}
	if plateGeneration != batch.PlateGeneration {
		return fmt.Errorf("unit acknowledgement belongs to another plate")
	}
	unit, ok := batch.Plan.Unit(color)
	if !ok {
		return fmt.Errorf("register unit %q is not part of the joint correction", color)
	}
	if !unit.HasObservation {
		return fmt.Errorf("register unit %q has no valid observation to confirm", color)
	}
	if applied != unit.Applied {
		return fmt.Errorf("register unit %q confirmed %v, expected %v", color, applied, unit.Applied)
	}
	if batch.Acknowledged[color] {
		return fmt.Errorf("register unit %q already acknowledged", color)
	}
	batch.Acknowledged[color] = true
	return nil
}

// BatchStatus is a point-in-time view of an open correction batch.
type BatchStatus struct {
	PlateGeneration uint64   `json:"plate_generation"`
	MissingColors   []string `json:"missing_colors"`
	PendingUnits    []string `json:"pending_units"`
	Acknowledged    []string `json:"acknowledged"`
	Complete        bool     `json:"complete"`
}

// JointBatchStatus reports confirmation progress. Complete is true only when
// every process color was observed and every observed unit acknowledged, so a
// three-color partial can never read as a stable four-color set.
func (coordinator *Coordinator) JointBatchStatus() (BatchStatus, bool) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	batch := coordinator.batch
	if batch == nil {
		return BatchStatus{}, false
	}
	status := BatchStatus{
		PlateGeneration: batch.PlateGeneration,
		MissingColors:   append([]string(nil), batch.Plan.MissingColors...),
		Acknowledged:    []string{},
		PendingUnits:    []string{},
	}
	for _, unit := range batch.Plan.Units {
		if !unit.HasObservation {
			continue
		}
		if batch.Acknowledged[unit.Color] {
			status.Acknowledged = append(status.Acknowledged, unit.Color)
		} else {
			status.PendingUnits = append(status.PendingUnits, unit.Color)
		}
	}
	status.Complete = batch.Plan.Complete() && len(status.PendingUnits) == 0
	return status, true
}

// RequireJointComplete returns an error naming any missing color or
// unacknowledged unit. Callers use it before reporting the set stable.
func (coordinator *Coordinator) RequireJointComplete() error {
	status, ok := coordinator.JointBatchStatus()
	if !ok {
		return fmt.Errorf("no joint correction batch is open")
	}
	if len(status.MissingColors) > 0 {
		return fmt.Errorf("register set is not stable: colors without valid observation: %s", strings.Join(status.MissingColors, ", "))
	}
	if len(status.PendingUnits) > 0 {
		return fmt.Errorf("register set is not confirmed: units pending acknowledgement: %s", strings.Join(status.PendingUnits, ", "))
	}
	if !status.Complete {
		return fmt.Errorf("register set is incomplete")
	}
	return nil
}

// ClearJointBatch closes the open correction batch.
func (coordinator *Coordinator) ClearJointBatch() {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.batch = nil
}
