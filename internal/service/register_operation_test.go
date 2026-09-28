package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/model"
)

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	runtime, err := NewRuntime(Config{
		DataDir:           t.TempDir(),
		FestoonCapacity:   120,
		RegisterGap:       time.Second,
		RegisterTolerance: 0.25,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	return runtime
}

func TestAdjustPhaseAppliesAndConfirmsFourColors(t *testing.T) {
	runtime := newTestRuntime(t)

	if err := runtime.ExecuteOperation(context.Background(), "adjust_phase"); err != nil {
		t.Fatalf("adjust_phase: %v", err)
	}

	// Every press unit received a phase correction.
	_, _, _, phases := runtime.press.Snapshot()
	if len(phases) != len(model.ProcessColors()) {
		t.Fatalf("phase corrections = %v, want one per process color", phases)
	}

	// The joint batch is fully observed and acknowledged unit by unit.
	status, ok := runtime.registerControl.JointBatchStatus()
	if !ok {
		t.Fatal("expected an open joint correction batch")
	}
	if !status.Complete {
		t.Fatalf("joint batch not complete: %+v", status)
	}
	if len(status.Acknowledged) != len(model.ProcessColors()) || len(status.PendingUnits) != 0 {
		t.Fatalf("batch status = %+v, want 4 acknowledged and none pending", status)
	}
	if len(status.MissingColors) != 0 {
		t.Fatalf("unexpected missing colors: %v", status.MissingColors)
	}
	if err := runtime.registerControl.RequireJointComplete(); err != nil {
		t.Fatalf("joint set should be complete: %v", err)
	}
}

func TestAdjustPhaseMissingColorIsNotReportedStable(t *testing.T) {
	runtime := newTestRuntime(t)
	current, ok := runtime.CurrentJob()
	if !ok {
		t.Fatal("expected an active job")
	}

	// Black has no valid observation; the other three do.
	partial := map[string]float64{
		model.Cyan:    0.04,
		model.Magenta: -0.12,
		model.Yellow:  0.08,
	}
	err := runtime.runRegisterOperation(current, partial)
	if err == nil {
		t.Fatal("a three-color observation set must fail rather than be reported stable")
	}
	if !strings.Contains(err.Error(), model.Black) {
		t.Fatalf("error must name the unobserved color, got %q", err.Error())
	}

	// The register window itself is explicit about the missing color.
	if missing := runtime.registerControl.MissingColors(); len(missing) != 1 || missing[0] != model.Black {
		t.Fatalf("missing colors = %v, want [black]", missing)
	}
	if runtime.registerControl.Stable(1) {
		t.Fatal("register set must not be stable without a black observation")
	}
}
