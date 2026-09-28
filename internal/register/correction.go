package register

import (
	"fmt"
	"math"

	"github.com/wyw14/solo-038/internal/model"
)

// UnitCorrection is the planned correction for one press unit (process color).
type UnitCorrection struct {
	Color          string  `json:"color"`
	Observed       float64 `json:"observed"`
	Requested      float64 `json:"requested"`
	Applied        float64 `json:"applied"`
	Capped         bool    `json:"capped"`
	HasObservation bool    `json:"has_observation"`
}

// JointCorrection is the combined, travel-limited correction for the active
// plate generation. Colors without a valid observation are listed in
// MissingColors instead of being silently treated as zero error.
type JointCorrection struct {
	PlateGeneration uint64           `json:"plate_generation"`
	TravelLimit     float64          `json:"travel_limit"`
	Units           []UnitCorrection `json:"units"`
	MissingColors   []string         `json:"missing_colors"`
}

// PlanJointCorrection builds one clamped correction per process color from the
// latest observation in the window. Colors without observations receive no
// correction and are reported as missing rather than assumed correct.
func PlanJointCorrection(window Window, plateGeneration uint64, travelLimit float64) (JointCorrection, error) {
	if !window.Valid {
		return JointCorrection{}, fmt.Errorf("register window is not active")
	}
	if plateGeneration == 0 || plateGeneration != window.PlateGeneration {
		return JointCorrection{}, fmt.Errorf("joint correction does not match active plate")
	}
	if math.IsNaN(travelLimit) || math.IsInf(travelLimit, 0) || travelLimit <= 0 {
		return JointCorrection{}, fmt.Errorf("unit travel limit is invalid")
	}
	joint := JointCorrection{PlateGeneration: plateGeneration, TravelLimit: travelLimit, Units: []UnitCorrection{}, MissingColors: window.MissingColors(model.ProcessColors())}
	offsets := window.Offsets()
	for _, color := range model.ProcessColors() {
		offset, ok := offsets[color]
		if !ok {
			joint.Units = append(joint.Units, UnitCorrection{Color: color, HasObservation: false})
			continue
		}
		requested := -offset
		applied := clampCorrection(requested, travelLimit)
		joint.Units = append(joint.Units, UnitCorrection{
			Color:          color,
			Observed:       offset,
			Requested:      requested,
			Applied:        applied,
			Capped:         applied != requested,
			HasObservation: true,
		})
	}
	return joint, nil
}

// Complete reports whether every process color had an observation. A partial
// plan for three colors must never be presented as a stable four-color set.
func (joint JointCorrection) Complete() bool {
	return len(joint.MissingColors) == 0 && len(joint.Units) == len(model.ProcessColors())
}

// Unit returns the planned correction for one color.
func (joint JointCorrection) Unit(color string) (UnitCorrection, bool) {
	for _, unit := range joint.Units {
		if unit.Color == color {
			return unit, true
		}
	}
	return UnitCorrection{}, false
}

// AppliedMap returns the corrections keyed by unit color.
func (joint JointCorrection) AppliedMap() map[string]float64 {
	applied := make(map[string]float64, len(joint.Units))
	for _, unit := range joint.Units {
		if unit.HasObservation {
			applied[unit.Color] = unit.Applied
		}
	}
	return applied
}

func clampCorrection(value, limit float64) float64 {
	if value > limit {
		return limit
	}
	if value < -limit {
		return -limit
	}
	return value
}
