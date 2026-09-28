package register

import (
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/model"
	"github.com/wyw14/solo-038/internal/sensor"
)

func frameFor(color string, jobGen, plateGen uint64, meter float64, offset float64, at time.Time) sensor.CameraFrame {
	return sensor.CameraFrame{
		ID:              "frame-" + color,
		Camera:          "register-camera-" + color,
		Color:           color,
		JobGeneration:   jobGen,
		PlateGeneration: plateGen,
		WebMeter:        meter,
		Offset:          offset,
		CapturedAt:      at,
		ReceivedAt:      at.Add(time.Millisecond),
	}
}

// windowWith builds a valid window containing one frame per given color.
func windowWith(t *testing.T, offsets map[string]float64) (Window, *State) {
	t.Helper()
	now := time.Now().UTC()
	state := NewState()
	if err := state.Start(1, 1, 0, now); err != nil {
		t.Fatalf("start: %v", err)
	}
	meter := 0.0
	for _, color := range model.ProcessColors() {
		offset, ok := offsets[color]
		if !ok {
			continue
		}
		meter++
		if err := state.Add(frameFor(color, 1, 1, meter, offset, now.Add(time.Duration(meter)*time.Millisecond))); err != nil {
			t.Fatalf("add %s: %v", color, err)
		}
	}
	return state.Snapshot(), state
}

func TestWindowStableRequiresEveryColor(t *testing.T) {
	partial := map[string]float64{model.Cyan: 0.01, model.Magenta: 0.02, model.Yellow: 0.03}
	window, state := windowWith(t, partial)

	if got := window.MissingColors(model.ProcessColors()); len(got) != 1 || got[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", got)
	}
	if state.MissingColors()[0] != model.Black {
		t.Fatalf("state missing colors = %v, want [black]", state.MissingColors())
	}
	if window.Stable(model.ProcessColors(), 1) {
		t.Fatal("three observed colors must not stabilize the four-color window")
	}
	if window.Samples() != 3 {
		t.Fatalf("total samples = %d, want 3", window.Samples())
	}

	// Adding the fourth color completes the set.
	if err := state.Add(frameFor(model.Black, 1, 1, 4, 0.04, time.Now().Add(5*time.Millisecond))); err != nil {
		t.Fatalf("add black: %v", err)
	}
	if !state.Snapshot().Stable(model.ProcessColors(), 1) {
		t.Fatal("window with one frame per color must be stable")
	}
}

func TestPlanJointCorrectionClampsToTravel(t *testing.T) {
	offsets := map[string]float64{
		model.Cyan:    3.0,   // requests -3.0, must cap at -2.5
		model.Magenta: -0.12, // requests +0.12
		model.Yellow:  0.08,
		model.Black:   -4.0, // requests +4.0, must cap at +2.5
	}
	window, _ := windowWith(t, offsets)
	plan, err := PlanJointCorrection(window, 1, 2.5)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Complete() {
		t.Fatal("all four colors observed, plan should be complete")
	}
	cyan, ok := plan.Unit(model.Cyan)
	if !ok || !cyan.Capped || cyan.Applied != -2.5 || cyan.Requested != -3.0 {
		t.Fatalf("cyan correction = %+v, want capped -2.5 from -3.0", cyan)
	}
	black, _ := plan.Unit(model.Black)
	if !black.Capped || black.Applied != 2.5 {
		t.Fatalf("black correction = %+v, want capped +2.5", black)
	}
	magenta, _ := plan.Unit(model.Magenta)
	if magenta.Capped || magenta.Applied != 0.12 {
		t.Fatalf("magenta correction = %+v, want uncapped 0.12", magenta)
	}
}

func TestPlanJointCorrectionKeepsMissingColor(t *testing.T) {
	partial := map[string]float64{model.Cyan: 0.04, model.Magenta: -0.1, model.Yellow: 0.06}
	window, _ := windowWith(t, partial)

	plan, err := PlanJointCorrection(window, 1, 2.5)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Complete() {
		t.Fatal("plan without black observation must not be complete")
	}
	if len(plan.MissingColors) != 1 || plan.MissingColors[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", plan.MissingColors)
	}
	black, ok := plan.Unit(model.Black)
	if !ok || black.HasObservation || black.Applied != 0 {
		t.Fatalf("black unit = %+v, want an unobserved, uncorrected entry", black)
	}
	if _, present := plan.AppliedMap()[model.Black]; present {
		t.Fatal("unobserved color must not receive an applied correction")
	}
}

func TestPlanJointCorrectionRejectsBadInputs(t *testing.T) {
	window, _ := windowWith(t, map[string]float64{model.Cyan: 0.04})

	if _, err := PlanJointCorrection(window, 2, 2.5); err == nil {
		t.Error("plate generation mismatch must error")
	}
	if _, err := PlanJointCorrection(window, 1, 0); err == nil {
		t.Error("non-positive travel limit must error")
	}
	if _, err := PlanJointCorrection(window, 1, -1); err == nil {
		t.Error("negative travel limit must error")
	}
	state := NewState()
	broken := state.Snapshot()
	if _, err := PlanJointCorrection(broken, 1, 2.5); err == nil {
		t.Error("inactive window must error")
	}
}
