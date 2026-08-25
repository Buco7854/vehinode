package providers

import "testing"

func TestObservedMetricsKeepsOnlyFreshMovementEvidence(t *testing.T) {
	metrics := map[string]any{
		"battery.soc":      70.0,
		"vehicle.speed":    42.0,
		"vehicle.ready":    true,
		"vehicle.ignition": false,
	}
	observed := observedMetrics(metrics, map[string]bool{"vehicle.ready": true})

	if observed["battery.soc"] != 70.0 || observed["vehicle.ready"] != true {
		t.Fatalf("fresh metrics missing: %#v", observed)
	}
	if _, present := observed["vehicle.speed"]; present {
		t.Fatalf("stale speed leaked into observation: %#v", observed)
	}
	if _, present := observed["vehicle.ignition"]; present {
		t.Fatalf("stale ignition leaked into observation: %#v", observed)
	}
}
