package runtime

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Buco7854/vehinode/agent/internal/client"
	"github.com/Buco7854/vehinode/agent/internal/model"
	"github.com/Buco7854/vehinode/agent/internal/store"
)

type PositionProvider interface {
	Read() (*model.PositionFix, error)
}
type VehicleProvider interface {
	ReadMetrics() (map[string]any, error)
	Close()
}

type EmptyPosition struct{}

func (EmptyPosition) Read() (*model.PositionFix, error) { return nil, nil }

type EmptyVehicle struct{}

func (EmptyVehicle) ReadMetrics() (map[string]any, error) { return map[string]any{}, nil }
func (EmptyVehicle) Close()                               {}

type Agent struct {
	Queue    *store.Queue
	Client   *client.Client
	Position PositionProvider
	Vehicle  VehicleProvider
	BootID   string
	Sequence int64
}

type Observation struct {
	Position *model.PositionFix
	Metrics  map[string]any
}

type UploadResult struct {
	Acknowledged  int64
	ConfigVersion int
}

func (agent *Agent) Collect() (model.Sample, error) {
	return agent.EnqueueObservation(agent.Observe())
}

func (agent *Agent) Observe() Observation {
	position, _ := agent.Position.Read()
	metrics, _ := agent.Vehicle.ReadMetrics()
	return Observation{Position: position, Metrics: metrics}
}

func (agent *Agent) EnqueueObservation(observation Observation) (model.Sample, error) {
	depth, _ := agent.Queue.Depth()
	sample := model.NewSample(agent.Sequence+1, observation.Position, observation.Metrics, SystemHealth(depth))
	agent.Sequence++
	return sample, agent.Queue.Enqueue(sample)
}

const (
	movingSpeedKMH = 3.0
	minMovementM   = 15.0
	parkedAfter    = 5 * time.Minute
)

type ParkedDetector struct {
	lastMovement time.Time
	lastPosition *model.PositionFix
	parked       bool
}

func NewParkedDetector(now time.Time) *ParkedDetector {
	return &ParkedDetector{lastMovement: now}
}

func (detector *ParkedDetector) Observe(now time.Time, observation Observation) (parked, changed bool) {
	wasParked := detector.parked
	position := observation.Position
	moving := position != nil && position.Speed != nil && *position.Speed > movingSpeedKMH
	if speed, ok := numericMetric(observation.Metrics["vehicle.speed"]); ok && speed > movingSpeedKMH {
		moving = true
	}
	if position != nil && detector.lastPosition != nil {
		threshold := minMovementM
		if position.Accuracy != nil && *position.Accuracy*2 > threshold {
			threshold = *position.Accuracy * 2
		}
		if distanceMeters(detector.lastPosition, position) >= threshold {
			moving = true
		}
	}
	if position != nil {
		copy := *position
		detector.lastPosition = &copy
	}
	ready, readyKnown := booleanMetric(observation.Metrics["vehicle.ready"])
	if !readyKnown {
		ready, readyKnown = booleanMetric(observation.Metrics["vehicle.ignition"])
	}
	if moving || readyKnown && ready {
		detector.lastMovement = now
		detector.parked = false
	} else if readyKnown {
		detector.parked = true
	} else if now.Sub(detector.lastMovement) >= parkedAfter {
		detector.parked = true
	}
	return detector.parked, detector.parked != wasParked
}

func numericMetric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func booleanMetric(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(typed) {
		case "on", "ready", "running", "true":
			return true, true
		case "off", "not_ready", "stopped", "false":
			return false, true
		}
	}
	return false, false
}

func distanceMeters(left, right *model.PositionFix) float64 {
	const earthRadiusM = 6371000.0
	lat1 := left.Latitude * math.Pi / 180
	lat2 := right.Latitude * math.Pi / 180
	deltaLat := (right.Latitude - left.Latitude) * math.Pi / 180
	deltaLon := (right.Longitude - left.Longitude) * math.Pi / 180
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (agent *Agent) Upload(limit int) (UploadResult, error) {
	samples, err := agent.Queue.Pending(limit)
	if err != nil || len(samples) == 0 {
		return UploadResult{}, err
	}
	response, err := agent.Client.Upload(agent.BootID, samples)
	if err != nil {
		return UploadResult{}, err
	}
	acknowledged, err := agent.Queue.Acknowledge(response.Acknowledged)
	return UploadResult{Acknowledged: acknowledged, ConfigVersion: response.ConfigVersion}, err
}

func SystemHealth(queueDepth int) map[string]any {
	hostname, _ := os.Hostname()
	result := map[string]any{"hostname": hostname, "queue_depth": queueDepth}
	if source, err := os.Open("/proc/loadavg"); err == nil {
		scanner := bufio.NewScanner(source)
		scanner.Split(bufio.ScanWords)
		if scanner.Scan() {
			if value, err := strconv.ParseFloat(scanner.Text(), 64); err == nil {
				result["load_1m"] = value
			}
		}
		source.Close()
	}
	if content, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		if value, err := strconv.ParseFloat(strings.TrimSpace(string(content)), 64); err == nil {
			result["cpu_temperature"] = float64(int(value/100)) / 10
		}
	}
	return result
}

func LastSequence(queue *store.Queue) (int64, error) {
	return queue.LastSequence()
}

func ValidateDistinctDevices(gps, obd string) error {
	if gps == "" || obd == "" {
		return nil
	}
	gpsPath, gpsErr := filepath.EvalSymlinks(gps)
	if gpsErr != nil {
		gpsPath = filepath.Clean(gps)
	}
	obdPath, obdErr := filepath.EvalSymlinks(obd)
	if obdErr != nil {
		obdPath = filepath.Clean(obd)
	}
	if gpsPath == obdPath {
		return fmt.Errorf("GPS and OBD cannot use the same serial device")
	}
	return nil
}
