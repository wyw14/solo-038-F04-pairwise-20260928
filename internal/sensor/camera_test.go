package sensor

import (
	"testing"
	"time"

	"github.com/wyw14/solo-038/internal/model"
)

func TestCameraFrameRequiresProcessColor(t *testing.T) {
	now := time.Now().UTC()
	valid := CameraFrame{
		ID:              "f1",
		Camera:          "cam",
		Color:           model.Magenta,
		JobGeneration:   1,
		PlateGeneration: 1,
		CapturedAt:      now,
		ReceivedAt:      now.Add(time.Millisecond),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid frame rejected: %v", err)
	}
	missing := valid
	missing.Color = ""
	if err := missing.Validate(); err == nil {
		t.Fatal("frame without a color must be invalid")
	}
	unknown := valid
	unknown.Color = "spot"
	if err := unknown.Validate(); err == nil {
		t.Fatal("frame with a non-process color must be invalid")
	}
}
