package sensor

import (
	"fmt"
	"time"

	"github.com/wyw14/solo-038/internal/model"
)

type CameraFrame struct {
	ID              string    `json:"id"`
	Camera          string    `json:"camera"`
	Color           string    `json:"color"`
	JobGeneration   uint64    `json:"job_generation"`
	PlateGeneration uint64    `json:"plate_generation"`
	WebMeter        float64   `json:"web_meter"`
	Offset          float64   `json:"offset"`
	CapturedAt      time.Time `json:"captured_at"`
	ReceivedAt      time.Time `json:"received_at"`
}

func (frame CameraFrame) Validate() error {
	if frame.ID == "" || frame.Camera == "" {
		return fmt.Errorf("camera frame identity is incomplete")
	}
	if !model.IsProcessColor(frame.Color) {
		return fmt.Errorf("camera frame color %q is not a process color", frame.Color)
	}
	if frame.JobGeneration == 0 || frame.PlateGeneration == 0 {
		return fmt.Errorf("camera frame generation is incomplete")
	}
	if frame.CapturedAt.IsZero() || frame.ReceivedAt.IsZero() {
		return fmt.Errorf("camera frame timestamps are incomplete")
	}
	if frame.Offset != frame.Offset { // NaN check without importing math
		return fmt.Errorf("camera frame offset is invalid")
	}
	return nil
}

func (frame CameraFrame) BelongsTo(jobGeneration, plateGeneration uint64) bool {
	return frame.JobGeneration == jobGeneration && frame.PlateGeneration == plateGeneration
}

func (frame CameraFrame) DeliveryDelay() time.Duration {
	return frame.ReceivedAt.Sub(frame.CapturedAt)
}
