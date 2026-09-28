package press

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/wyw14/solo-038/internal/model"
)

type registerSample struct {
	plateGeneration uint64
	unit            string
	offset          float64
}

type Coordinator struct {
	mu               sync.RWMutex
	controller       *Controller
	state            *State
	stopCommand      model.Command
	stopProof        map[string]model.Acknowledgement
	registerSamples  []registerSample
	registerCommand  model.Command
	registerExpected map[string]float64
	registerProof    map[string]model.Acknowledgement
}

func NewCoordinator(controller *Controller, state *State) *Coordinator {
	return &Coordinator{controller: controller, state: state, stopProof: map[string]model.Acknowledgement{}}
}

func (coordinator *Coordinator) BeginStop(job model.Job, epoch uint64, now time.Time) model.Command {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.stopCommand = model.NewCommand(job, epoch, model.CommandStop, now)
	coordinator.stopProof = map[string]model.Acknowledgement{}
	return coordinator.stopCommand
}

func (coordinator *Coordinator) AcknowledgeStop(ack model.Acknowledgement) error {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if ack.Operation != coordinator.stopCommand.Operation || ack.Epoch != coordinator.stopCommand.Epoch {
		return fmt.Errorf("stop acknowledgement belongs to another command")
	}
	if ack.Value != 0 {
		return fmt.Errorf("press unit has not reached zero speed")
	}
	if _, ok := coordinator.controller.Unit(ack.Unit); !ok {
		return fmt.Errorf("press unit is unknown")
	}
	coordinator.stopProof[ack.Unit] = ack
	return nil
}

func (coordinator *Coordinator) CanSplice() bool {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	if !coordinator.stopCommand.Operation.Valid() {
		return false
	}
	for _, unit := range coordinator.controller.Names() {
		proof, ok := coordinator.stopProof[unit]
		if !ok || proof.Operation != coordinator.stopCommand.Operation || proof.Epoch != coordinator.stopCommand.Epoch || proof.Value != 0 {
			return false
		}
	}
	return true
}

func (coordinator *Coordinator) ChangePlate(plate string) (uint64, error) {
	generation, err := coordinator.state.ChangePlate(plate)
	if err != nil {
		return 0, err
	}
	coordinator.mu.Lock()
	coordinator.registerSamples = nil
	coordinator.registerCommand = model.Command{}
	coordinator.registerExpected = nil
	coordinator.registerProof = nil
	coordinator.mu.Unlock()
	return generation, nil
}

func (coordinator *Coordinator) RecordRegister(plateGeneration uint64, unit string, offset float64) error {
	if _, ok := coordinator.controller.Unit(unit); !ok {
		return fmt.Errorf("register sample unit is unknown")
	}
	_, active, _, _ := coordinator.state.Snapshot()
	if plateGeneration != active {
		return fmt.Errorf("register sample belongs to another plate")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.registerSamples = append(coordinator.registerSamples, registerSample{plateGeneration: plateGeneration, unit: unit, offset: offset})
	if len(coordinator.registerSamples) > 20 {
		coordinator.registerSamples = coordinator.registerSamples[len(coordinator.registerSamples)-20:]
	}
	return nil
}

// RegisterStable reports the set stable only when every unit has enough
// in-tolerance samples for the active plate generation. A color without
// valid observations keeps the whole set from passing as stable.
func (coordinator *Coordinator) RegisterStable(tolerance float64, minimum int) bool {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	_, active, _, _ := coordinator.state.Snapshot()
	counts := map[string]int{}
	for _, sample := range coordinator.registerSamples {
		if sample.plateGeneration == active && math.Abs(sample.offset) <= tolerance {
			counts[sample.unit]++
		}
	}
	for _, unit := range coordinator.controller.Names() {
		if counts[unit] < minimum {
			return false
		}
	}
	return true
}

// BeginRegisterCorrection opens a per-unit confirmation round for a joint
// correction set: every unit must acknowledge its commanded correction.
func (coordinator *Coordinator) BeginRegisterCorrection(job model.Job, epoch uint64, corrections map[string]float64, now time.Time) model.Command {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	coordinator.registerCommand = model.NewCommand(job, epoch, model.CommandPhase, now)
	coordinator.registerExpected = map[string]float64{}
	for unit, correction := range corrections {
		coordinator.registerExpected[unit] = correction
	}
	coordinator.registerProof = map[string]model.Acknowledgement{}
	return coordinator.registerCommand
}

func (coordinator *Coordinator) AcknowledgeRegister(ack model.Acknowledgement) error {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if ack.Operation != coordinator.registerCommand.Operation || ack.Epoch != coordinator.registerCommand.Epoch {
		return fmt.Errorf("register acknowledgement belongs to another command")
	}
	if _, ok := coordinator.controller.Unit(ack.Unit); !ok {
		return fmt.Errorf("press unit is unknown")
	}
	expected, ok := coordinator.registerExpected[ack.Unit]
	if !ok {
		return fmt.Errorf("no register correction was commanded for unit %s", ack.Unit)
	}
	if ack.Value != expected {
		return fmt.Errorf("press unit %s did not apply the commanded register correction", ack.Unit)
	}
	coordinator.registerProof[ack.Unit] = ack
	return nil
}

// RegisterConfirmed is true only when every unit acknowledged the correction
// it was commanded for the current register epoch.
func (coordinator *Coordinator) RegisterConfirmed() bool {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	if !coordinator.registerCommand.Operation.Valid() {
		return false
	}
	for _, unit := range coordinator.controller.Names() {
		expected, ok := coordinator.registerExpected[unit]
		if !ok {
			return false
		}
		proof, ok := coordinator.registerProof[unit]
		if !ok || proof.Operation != coordinator.registerCommand.Operation || proof.Epoch != coordinator.registerCommand.Epoch || proof.Value != expected {
			return false
		}
	}
	return true
}
