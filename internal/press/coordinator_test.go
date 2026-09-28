package press

import (
	"testing"

	"github.com/wyw14/solo-038/internal/model"
)

func newRegisterCoordinator(t *testing.T) *Coordinator {
	t.Helper()
	controller, err := NewController(model.ProcessColors()...)
	if err != nil {
		t.Fatalf("controller: %v", err)
	}
	return NewCoordinator(controller, NewState())
}

func TestRegisterStableRequiresEveryColor(t *testing.T) {
	coordinator := newRegisterCoordinator(t)
	plate, err := coordinator.ChangePlate("plate-A")
	if err != nil {
		t.Fatalf("change plate: %v", err)
	}

	// Only three colors within tolerance; black is unobserved.
	for _, sample := range []struct {
		color  string
		offset float64
	}{
		{model.Cyan, 0.01},
		{model.Magenta, -0.02},
		{model.Yellow, 0.03},
	} {
		if err := coordinator.RecordRegister(plate, sample.color, sample.offset); err != nil {
			t.Fatalf("record %s: %v", sample.color, err)
		}
	}
	if coordinator.RegisterStable(0.1, 1) {
		t.Fatal("three in-tolerance colors must not stabilize the four-color set")
	}
	missing := coordinator.RegisterMissingColors(0.1)
	if len(missing) != 1 || missing[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", missing)
	}

	if err := coordinator.RecordRegister(plate, model.Black, 0.05); err != nil {
		t.Fatalf("record black: %v", err)
	}
	if !coordinator.RegisterStable(0.1, 1) {
		t.Fatal("all four colors in tolerance must stabilize the set")
	}
	if missing := coordinator.RegisterMissingColors(0.1); len(missing) != 0 {
		t.Fatalf("missing colors = %v, want none", missing)
	}
}

func TestRegisterStableTreatsOutOfToleranceAsMissing(t *testing.T) {
	coordinator := newRegisterCoordinator(t)
	plate, _ := coordinator.ChangePlate("plate-B")
	for _, color := range model.ProcessColors() {
		offset := 0.05
		if color == model.Black {
			offset = 0.5 // observed but out of tolerance
		}
		if err := coordinator.RecordRegister(plate, color, offset); err != nil {
			t.Fatalf("record %s: %v", color, err)
		}
	}
	if coordinator.RegisterStable(0.1, 1) {
		t.Fatal("an out-of-tolerance color must keep the set unstable")
	}
	missing := coordinator.RegisterMissingColors(0.1)
	if len(missing) != 1 || missing[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", missing)
	}
}

func TestRecordRegisterRejectsUnknownColor(t *testing.T) {
	coordinator := newRegisterCoordinator(t)
	plate, _ := coordinator.ChangePlate("plate-C")
	if err := coordinator.RecordRegister(plate, "spot", 0.01); err == nil {
		t.Fatal("recording a non-process color must error")
	}
}
