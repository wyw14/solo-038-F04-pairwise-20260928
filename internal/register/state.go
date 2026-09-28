package register

import (
	"fmt"
	"sync"
	"time"

	"github.com/wyw14/solo-038/internal/model"
	"github.com/wyw14/solo-038/internal/sensor"
)

type Window struct {
	JobGeneration     uint64             `json:"job_generation"`
	PlateGeneration   uint64             `json:"plate_generation"`
	StartMeter        float64            `json:"start_meter"`
	LastMeter         float64            `json:"last_meter"`
	StartedAt         time.Time          `json:"started_at"`
	LastAt            time.Time          `json:"last_at"`
	SamplesByColor    map[string]int     `json:"samples_by_color"`
	LastOffsetByColor map[string]float64 `json:"last_offset_by_color"`
	Valid             bool               `json:"valid"`
}

// Samples returns the total number of accepted frames across all colors.
func (window Window) Samples() int {
	total := 0
	for _, count := range window.SamplesByColor {
		total += count
	}
	return total
}

// ColorSamples returns the number of accepted frames for one color.
func (window Window) ColorSamples(color string) int {
	return window.SamplesByColor[color]
}

// MissingColors returns the required colors that have no accepted observations,
// in the canonical process-color order.
func (window Window) MissingColors(required []string) []string {
	missing := []string{}
	for _, color := range required {
		if window.SamplesByColor[color] == 0 {
			missing = append(missing, color)
		}
	}
	return missing
}

// Offsets returns the most recent accepted offset for each observed color.
func (window Window) Offsets() map[string]float64 {
	offsets := make(map[string]float64, len(window.LastOffsetByColor))
	for color, offset := range window.LastOffsetByColor {
		offsets[color] = offset
	}
	return offsets
}

// Stable reports whether the window is valid and every required color has at
// least minimum accepted frames. A window fed by a single camera/color can
// accumulate many samples and still be unstable for the four-color set.
func (window Window) Stable(required []string, minimum int) bool {
	if !window.Valid || minimum <= 0 {
		return false
	}
	for _, color := range required {
		if window.SamplesByColor[color] < minimum {
			return false
		}
	}
	return true
}

type State struct {
	mu     sync.RWMutex
	window Window
}

func NewState() *State {
	return &State{}
}

func (state *State) Start(jobGeneration, plateGeneration uint64, meter float64, now time.Time) error {
	if jobGeneration == 0 || plateGeneration == 0 {
		return fmt.Errorf("register window generations are required")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.window = Window{
		JobGeneration:     jobGeneration,
		PlateGeneration:   plateGeneration,
		StartMeter:        meter,
		LastMeter:         meter,
		StartedAt:         now.UTC(),
		LastAt:            now.UTC(),
		SamplesByColor:    map[string]int{},
		LastOffsetByColor: map[string]float64{},
		Valid:             true,
	}
	return nil
}

func (state *State) Add(frame sensor.CameraFrame) error {
	if err := frame.Validate(); err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.window.Valid || frame.JobGeneration != state.window.JobGeneration || frame.PlateGeneration != state.window.PlateGeneration {
		return fmt.Errorf("register frame generation does not match window")
	}
	if frame.WebMeter < state.window.LastMeter {
		return fmt.Errorf("register frame meter moved backwards")
	}
	state.window.LastMeter = frame.WebMeter
	state.window.LastAt = frame.ReceivedAt
	state.window.SamplesByColor[frame.Color]++
	state.window.LastOffsetByColor[frame.Color] = frame.Offset
	return nil
}

func (state *State) Break() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.window.Valid = false
}

func (state *State) Snapshot() Window {
	state.mu.RLock()
	defer state.mu.RUnlock()
	snapshot := state.window
	if state.window.SamplesByColor != nil {
		snapshot.SamplesByColor = make(map[string]int, len(state.window.SamplesByColor))
		for color, count := range state.window.SamplesByColor {
			snapshot.SamplesByColor[color] = count
		}
	}
	if state.window.LastOffsetByColor != nil {
		snapshot.LastOffsetByColor = make(map[string]float64, len(state.window.LastOffsetByColor))
		for color, offset := range state.window.LastOffsetByColor {
			snapshot.LastOffsetByColor[color] = offset
		}
	}
	return snapshot
}

// MissingColors returns the process colors without a valid observation in the
// current window.
func (state *State) MissingColors() []string {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.window.MissingColors(model.ProcessColors())
}
