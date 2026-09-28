package press

import (
	"fmt"
	"math"
	"strings"

	"github.com/wyw14/solo-038/internal/sensor"
)

// RegisterAdjustment records one unit's applied register correction.
type RegisterAdjustment struct {
	FrameID         string  `json:"frame_id"`
	Unit            string  `json:"unit"`
	PlateGeneration uint64  `json:"plate_generation"`
	Correction      float64 `json:"correction"`
}

// UnitCorrection is one unit's planned correction within its travel limit.
type UnitCorrection struct {
	FrameID    string  `json:"frame_id"`
	Unit       string  `json:"unit"`
	Offset     float64 `json:"offset"`
	Correction float64 `json:"correction"`
	Clamped    bool    `json:"clamped"`
}

// JointCorrection is the joint per-unit correction set for one plate
// generation. Units without a valid observation are named in Missing so an
// incomplete set can never be treated as a full one.
type JointCorrection struct {
	PlateGeneration uint64           `json:"plate_generation"`
	Corrections     []UnitCorrection `json:"corrections"`
	Missing         []string         `json:"missing"`
}

func (joint JointCorrection) Complete() bool {
	return len(joint.Missing) == 0 && len(joint.Corrections) > 0
}

// PlanJoint builds the joint correction set for the active plate generation.
// Every unit must contribute a valid observation for the same generation; a
// unit whose frame is absent, stale, or invalid is listed as missing.
func (controller *PhaseController) PlanJoint(units []string, frames map[string]sensor.CameraFrame, activeJob, activePlate uint64) (JointCorrection, error) {
	if controller.maximum <= 0 {
		return JointCorrection{}, fmt.Errorf("unit travel limit is invalid")
	}
	joint := JointCorrection{PlateGeneration: activePlate}
	for _, unit := range units {
		frame, ok := frames[unit]
		if !ok || !validObservation(frame, activeJob, activePlate) {
			joint.Missing = append(joint.Missing, unit)
			continue
		}
		correction, clamped := limitCorrection(frame.Offset, controller.maximum)
		joint.Corrections = append(joint.Corrections, UnitCorrection{FrameID: frame.ID, Unit: unit, Offset: frame.Offset, Correction: correction, Clamped: clamped})
	}
	return joint, nil
}

func validObservation(frame sensor.CameraFrame, activeJob, activePlate uint64) bool {
	if frame.Validate() != nil || !frame.BelongsTo(activeJob, activePlate) {
		return false
	}
	return !math.IsNaN(frame.Offset) && !math.IsInf(frame.Offset, 0)
}

// ApplyJoint applies a complete joint correction set to every unit. An
// incomplete set is refused so partial corrections are never reported as a
// finished four-color adjustment.
func (controller *PhaseController) ApplyJoint(joint JointCorrection) ([]RegisterAdjustment, error) {
	if !joint.Complete() {
		return nil, fmt.Errorf("register correction set is incomplete: units %s have no valid observation", strings.Join(joint.Missing, ", "))
	}
	adjustments := make([]RegisterAdjustment, 0, len(joint.Corrections))
	for _, planned := range joint.Corrections {
		correction, err := controller.Correct(planned.Unit, planned.Offset, joint.PlateGeneration)
		if err != nil {
			return nil, err
		}
		adjustments = append(adjustments, RegisterAdjustment{FrameID: planned.FrameID, Unit: planned.Unit, PlateGeneration: joint.PlateGeneration, Correction: correction})
	}
	return adjustments, nil
}
