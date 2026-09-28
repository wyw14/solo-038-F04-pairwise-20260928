package register

import (
	"fmt"
	"time"

	"github.com/wyw14/solo-038/internal/model"
	"github.com/wyw14/solo-038/internal/sensor"
)

type Receiver struct {
	state  *State
	maxGap time.Duration
}

func NewReceiver(state *State, maxGap time.Duration) *Receiver {
	return &Receiver{state: state, maxGap: maxGap}
}

func (receiver *Receiver) Receive(frame sensor.CameraFrame) error {
	window := receiver.state.Snapshot()
	if !window.Valid {
		return fmt.Errorf("register window is not active")
	}
	if frame.ReceivedAt.Sub(window.LastAt) > receiver.maxGap {
		receiver.state.Break()
		return fmt.Errorf("camera gap exceeded register dwell")
	}
	return receiver.state.Add(frame)
}

// Stable reports whether every process color has at least minimum frames.
func (receiver *Receiver) Stable(minimum int) bool {
	window := receiver.state.Snapshot()
	return window.Stable(model.ProcessColors(), minimum)
}

// MissingColors returns the process colors without an accepted observation.
func (receiver *Receiver) MissingColors() []string {
	return receiver.state.MissingColors()
}
