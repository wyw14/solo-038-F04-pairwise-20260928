package register

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wyw14/solo-038/internal/journal"
	"github.com/wyw14/solo-038/internal/model"
)

func newTestCoordinator(t *testing.T, offsets map[string]float64) *Coordinator {
	t.Helper()
	_, state := windowWith(t, offsets)
	store := journal.NewStore(filepath.Join(t.TempDir(), "events.jsonl"))
	return NewCoordinator(state, store)
}

func TestJointBatchAcknowledgesUnitByUnit(t *testing.T) {
	offsets := map[string]float64{model.Cyan: 0.04, model.Magenta: -0.12, model.Yellow: 0.08, model.Black: 0.05}
	coordinator := newTestCoordinator(t, offsets)

	plan, err := coordinator.BeginJointCorrection(1, 2.5)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !plan.Complete() {
		t.Fatal("four observed colors should yield a complete plan")
	}

	// Acknowledging only cyan leaves the rest pending.
	cyan, _ := plan.Unit(model.Cyan)
	if err := coordinator.AcknowledgeUnit(1, model.Cyan, cyan.Applied); err != nil {
		t.Fatalf("ack cyan: %v", err)
	}
	if err := coordinator.RequireJointComplete(); err == nil {
		t.Fatal("set must not be complete while three units are pending")
	}

	// Wrong applied value is rejected.
	magenta, _ := plan.Unit(model.Magenta)
	if err := coordinator.AcknowledgeUnit(1, model.Magenta, magenta.Applied+0.01); err == nil {
		t.Fatal("acknowledgement with mismatched value must error")
	}
	// Duplicate acknowledgement is rejected.
	if err := coordinator.AcknowledgeUnit(1, model.Cyan, cyan.Applied); err == nil {
		t.Fatal("duplicate acknowledgement must error")
	}

	for _, unit := range plan.Units {
		if unit.Color == model.Cyan {
			continue
		}
		if err := coordinator.AcknowledgeUnit(1, unit.Color, unit.Applied); err != nil {
			t.Fatalf("ack %s: %v", unit.Color, err)
		}
	}
	if err := coordinator.RequireJointComplete(); err != nil {
		t.Fatalf("all four acknowledged must complete the set: %v", err)
	}
	status, ok := coordinator.JointBatchStatus()
	if !ok || !status.Complete || len(status.PendingUnits) != 0 || len(status.Acknowledged) != 4 {
		t.Fatalf("status = %+v ok=%v, want complete with 4 acks", status, ok)
	}
}

func TestJointBatchMissingColorIsNotStable(t *testing.T) {
	// Black has no observation: the other three corrections must not be
	// reported as a stable four-color set.
	offsets := map[string]float64{model.Cyan: 0.04, model.Magenta: -0.12, model.Yellow: 0.08}
	coordinator := newTestCoordinator(t, offsets)

	plan, err := coordinator.BeginJointCorrection(1, 2.5)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if plan.Complete() {
		t.Fatal("partial plan must not be complete")
	}
	if coordinator.Stable(1) {
		t.Fatal("three observed colors must not stabilize the register set")
	}
	if missing := coordinator.MissingColors(); len(missing) != 1 || missing[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", missing)
	}

	for _, unit := range plan.Units {
		if !unit.HasObservation {
			// An unobserved unit cannot be confirmed against a planned value.
			if err := coordinator.AcknowledgeUnit(1, unit.Color, 0); err == nil {
				t.Fatalf("acknowledging unobserved %s must error", unit.Color)
			}
			continue
		}
		if err := coordinator.AcknowledgeUnit(1, unit.Color, unit.Applied); err != nil {
			t.Fatalf("ack %s: %v", unit.Color, err)
		}
	}

	err = coordinator.RequireJointComplete()
	if err == nil {
		t.Fatal("three confirmed colors must not complete the four-color set")
	}
	if !strings.Contains(err.Error(), model.Black) {
		t.Fatalf("error must name the missing color, got %q", err.Error())
	}

	status, ok := coordinator.JointBatchStatus()
	if !ok || status.Complete || len(status.MissingColors) != 1 {
		t.Fatalf("status = %+v ok=%v, want incomplete with black missing", status, ok)
	}
}

func TestJointBatchRejectsStaleAcknowldegement(t *testing.T) {
	coordinator := newTestCoordinator(t, map[string]float64{model.Cyan: 0.04})
	if _, err := coordinator.BeginJointCorrection(1, 2.5); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := coordinator.AcknowledgeUnit(2, model.Cyan, -0.04); err == nil {
		t.Fatal("acknowledgement for another plate must error")
	}
	coordinator.ClearJointBatch()
	if err := coordinator.AcknowledgeUnit(1, model.Cyan, -0.04); err == nil {
		t.Fatal("acknowledgement with no open batch must error")
	}
	if _, ok := coordinator.JointBatchStatus(); ok {
		t.Fatal("status must report no open batch after clear")
	}
}
