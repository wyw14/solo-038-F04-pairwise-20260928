package press

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/model"
	"github.com/wyw14/solo-038/internal/sensor"
)

func newTestPress(t *testing.T) (*Controller, *Coordinator, *PhaseController) {
	t.Helper()
	controller, err := NewController("cyan", "magenta", "yellow", "black")
	if err != nil {
		t.Fatal(err)
	}
	state := NewState()
	if _, err := state.ChangePlate("plate-1"); err != nil {
		t.Fatal(err)
	}
	return controller, NewCoordinator(controller, state), NewPhaseController(state, 2.5)
}

func testFrame(unit string, offset float64, jobGeneration, plateGeneration uint64) sensor.CameraFrame {
	now := time.Now()
	return sensor.CameraFrame{ID: "frame-" + unit, Camera: "camera-" + unit, Unit: unit, JobGeneration: jobGeneration, PlateGeneration: plateGeneration, WebMeter: 1, Offset: offset, CapturedAt: now, ReceivedAt: now}
}

func TestPlanJointCoversEveryUnit(t *testing.T) {
	controller, _, phase := newTestPress(t)
	frames := map[string]sensor.CameraFrame{
		"cyan":    testFrame("cyan", 0.04, 7, 1),
		"magenta": testFrame("magenta", -0.05, 7, 1),
		"yellow":  testFrame("yellow", 0.07, 7, 1),
		"black":   testFrame("black", 3.2, 7, 1),
	}
	joint, err := phase.PlanJoint(controller.Names(), frames, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !joint.Complete() {
		t.Fatalf("expected a complete set, missing %v", joint.Missing)
	}
	byUnit := map[string]UnitCorrection{}
	for _, correction := range joint.Corrections {
		byUnit[correction.Unit] = correction
	}
	if byUnit["cyan"].Correction != -0.04 || byUnit["magenta"].Correction != 0.05 || byUnit["yellow"].Correction != -0.07 {
		t.Fatalf("corrections should mirror the observed offsets: %+v", byUnit)
	}
	if byUnit["black"].Correction != -2.5 || !byUnit["black"].Clamped {
		t.Fatalf("black correction must clamp to the unit travel limit: %+v", byUnit["black"])
	}
}

func TestPlanJointFlagsMissingUnit(t *testing.T) {
	controller, _, phase := newTestPress(t)
	frames := map[string]sensor.CameraFrame{
		"cyan":    testFrame("cyan", 0.04, 7, 1),
		"magenta": testFrame("magenta", -0.05, 7, 1),
		"black":   testFrame("black", -0.03, 7, 1),
	}
	joint, err := phase.PlanJoint(controller.Names(), frames, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if joint.Complete() {
		t.Fatal("a set without a yellow observation must not be complete")
	}
	if len(joint.Missing) != 1 || joint.Missing[0] != "yellow" {
		t.Fatalf("expected yellow to be reported missing, got %v", joint.Missing)
	}
	if _, err := phase.ApplyJoint(joint); err == nil || !strings.Contains(err.Error(), "yellow") {
		t.Fatalf("applying an incomplete set must fail naming the missing unit, got %v", err)
	}
}

func TestPlanJointRejectsInvalidObservations(t *testing.T) {
	controller, _, phase := newTestPress(t)
	stale := testFrame("yellow", 0.07, 7, 99)
	nan := testFrame("black", math.NaN(), 7, 1)
	frames := map[string]sensor.CameraFrame{
		"cyan":    testFrame("cyan", 0.04, 7, 1),
		"magenta": testFrame("magenta", -0.05, 7, 1),
		"yellow":  stale,
		"black":   nan,
	}
	joint, err := phase.PlanJoint(controller.Names(), frames, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if joint.Complete() {
		t.Fatal("stale or invalid observations must not complete a set")
	}
	missing := strings.Join(joint.Missing, ",")
	if !strings.Contains(missing, "yellow") || !strings.Contains(missing, "black") {
		t.Fatalf("expected yellow and black to be missing, got %v", joint.Missing)
	}
}

func TestApplyJointAppliesEveryUnit(t *testing.T) {
	controller, _, phase := newTestPress(t)
	frames := map[string]sensor.CameraFrame{
		"cyan":    testFrame("cyan", 0.04, 7, 1),
		"magenta": testFrame("magenta", -0.05, 7, 1),
		"yellow":  testFrame("yellow", 0.07, 7, 1),
		"black":   testFrame("black", -0.03, 7, 1),
	}
	joint, err := phase.PlanJoint(controller.Names(), frames, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	adjustments, err := phase.ApplyJoint(joint)
	if err != nil {
		t.Fatal(err)
	}
	if len(adjustments) != 4 {
		t.Fatalf("expected four adjustments, got %d", len(adjustments))
	}
	for _, adjustment := range adjustments {
		last, ok := phase.Last(adjustment.Unit)
		if !ok || last != adjustment.Correction {
			t.Fatalf("unit %s correction %f was not recorded", adjustment.Unit, adjustment.Correction)
		}
	}
}

func TestRegisterStableRequiresEveryUnit(t *testing.T) {
	_, coordinator, _ := newTestPress(t)
	for _, unit := range []string{"cyan", "magenta", "black"} {
		if err := coordinator.RecordRegister(1, unit, 0.05); err != nil {
			t.Fatal(err)
		}
	}
	if coordinator.RegisterStable(0.1, 1) {
		t.Fatal("three colors must not pass as a stable set while yellow has no observation")
	}
	if err := coordinator.RecordRegister(1, "yellow", 0.05); err != nil {
		t.Fatal(err)
	}
	if !coordinator.RegisterStable(0.1, 1) {
		t.Fatal("all four colors observed within tolerance should be stable")
	}
	if err := coordinator.RecordRegister(1, "orange", 0.05); err == nil {
		t.Fatal("a sample for an unknown unit must be rejected")
	}
	if err := coordinator.RecordRegister(99, "cyan", 0.05); err == nil {
		t.Fatal("a sample for another plate generation must be rejected")
	}
}

func TestRegisterConfirmationRequiresEveryUnit(t *testing.T) {
	_, coordinator, _ := newTestPress(t)
	job := model.Job{ID: model.NewJobID(), State: model.JobRegistering, Generation: 3, PlateGeneration: 1}
	now := time.Now()
	corrections := map[string]float64{"cyan": -0.04, "magenta": 0.05, "yellow": -0.07, "black": 0.03}
	command := coordinator.BeginRegisterCorrection(job, 9, corrections, now)
	for _, unit := range []string{"cyan", "magenta", "black"} {
		ack := model.Acknowledgement{Unit: unit, Operation: command.Operation, Epoch: command.Epoch, Value: corrections[unit], At: now}
		if err := coordinator.AcknowledgeRegister(ack); err != nil {
			t.Fatal(err)
		}
	}
	if coordinator.RegisterConfirmed() {
		t.Fatal("confirmation must wait for every unit")
	}
	wrong := model.Acknowledgement{Unit: "yellow", Operation: command.Operation, Epoch: command.Epoch, Value: 0, At: now}
	if err := coordinator.AcknowledgeRegister(wrong); err == nil {
		t.Fatal("a unit acknowledging a different correction must be rejected")
	}
	stray := model.Acknowledgement{Unit: "yellow", Operation: model.NewOperationID(), Epoch: command.Epoch, Value: corrections["yellow"], At: now}
	if err := coordinator.AcknowledgeRegister(stray); err == nil {
		t.Fatal("an acknowledgement for another command must be rejected")
	}
	good := model.Acknowledgement{Unit: "yellow", Operation: command.Operation, Epoch: command.Epoch, Value: corrections["yellow"], At: now}
	if err := coordinator.AcknowledgeRegister(good); err != nil {
		t.Fatal(err)
	}
	if !coordinator.RegisterConfirmed() {
		t.Fatal("every unit acknowledged the commanded correction")
	}
}
