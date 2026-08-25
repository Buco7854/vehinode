package runtime

import (
	"testing"
	"time"

	"github.com/Buco7854/vehinode/agent/internal/model"
)

func floatPointer(value float64) *float64 { return &value }

func TestParkedDetectorKeepsTrafficStopsInDrivingState(t *testing.T) {
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	detector := NewParkedDetector(start)
	moving := &model.PositionFix{Latitude: 48.8566, Longitude: 2.3522, Speed: floatPointer(30)}
	stopped := &model.PositionFix{Latitude: 48.8566, Longitude: 2.3522, Speed: floatPointer(0)}

	if parked, _ := detector.Observe(start, Observation{Position: moving}); parked {
		t.Fatal("moving vehicle was classified as parked")
	}
	if parked, _ := detector.Observe(start.Add(4*time.Minute+59*time.Second), Observation{Position: stopped}); parked {
		t.Fatal("short traffic stop was classified as parked")
	}
	if parked, changed := detector.Observe(start.Add(5*time.Minute), Observation{Position: stopped}); !parked || !changed {
		t.Fatalf("parked=%v changed=%v, want parked transition", parked, changed)
	}
	moving.Speed = floatPointer(4)
	if parked, changed := detector.Observe(start.Add(5*time.Minute+time.Second), Observation{Position: moving}); parked || !changed {
		t.Fatalf("parked=%v changed=%v, want immediate moving transition", parked, changed)
	}
}

func TestParkedDetectorUsesMeaningfulPositionChangeAndIgnoresJitter(t *testing.T) {
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	detector := NewParkedDetector(start)
	stationary := &model.PositionFix{Latitude: 48.8566, Longitude: 2.3522, Accuracy: floatPointer(5)}
	jitter := &model.PositionFix{Latitude: 48.85662, Longitude: 2.3522, Accuracy: floatPointer(5)}
	moved := &model.PositionFix{Latitude: 48.8568, Longitude: 2.3522, Accuracy: floatPointer(5)}

	detector.Observe(start, Observation{Position: stationary})
	if parked, _ := detector.Observe(start.Add(5*time.Minute), Observation{Position: jitter}); !parked {
		t.Fatal("GPS jitter prevented the parked transition")
	}
	if parked, changed := detector.Observe(start.Add(5*time.Minute+time.Second), Observation{Position: moved}); parked || !changed {
		t.Fatalf("parked=%v changed=%v, want displacement to restore driving state", parked, changed)
	}
}

func TestParkedDetectorPrefersReadyAndFreshVehicleSpeed(t *testing.T) {
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	detector := NewParkedDetector(start)

	if parked, changed := detector.Observe(start, Observation{Metrics: map[string]any{"vehicle.ready": false}}); !parked || !changed {
		t.Fatalf("parked=%v changed=%v, want explicit READY-off transition", parked, changed)
	}
	if parked, changed := detector.Observe(start.Add(time.Second), Observation{Metrics: map[string]any{"vehicle.ready": true}}); parked || !changed {
		t.Fatalf("parked=%v changed=%v, want READY-on driving transition", parked, changed)
	}
	if parked, _ := detector.Observe(start.Add(2*time.Second), Observation{Metrics: map[string]any{"vehicle.speed": 0.0}}); parked {
		t.Fatal("zero CAN speed immediately classified the vehicle as parked")
	}
	if parked, _ := detector.Observe(start.Add(3*time.Second), Observation{Metrics: map[string]any{"vehicle.speed": 8.0}}); parked {
		t.Fatal("fresh CAN speed did not keep the vehicle in driving state")
	}
}
