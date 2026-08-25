package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func configuration(version, sample int) Configuration {
	profileID := "owner-profile"
	definition, _ := json.Marshal(map[string]any{
		"id": profileID,
		"signals": []any{map[string]any{
			"name": "battery.soc", "source": map[string]any{"type": "can", "can_id": 884},
			"decoder": map[string]any{"data_type": "uint8"}, "status": "experimental",
		}},
	})
	return Configuration{Version: version, Sampling: Interval{DefaultSeconds: sample}, Upload: Interval{DefaultSeconds: 30}, VehicleProfile: &profileID, VehicleProfileDefinition: definition}
}

func TestConfigurationLastKnownGood(t *testing.T) {
	store := ConfigurationStore{Path: filepath.Join(t.TempDir(), "config.json")}
	if _, err := store.InstallIfNewer(configuration(1, 5)); err != nil {
		t.Fatal(err)
	}
	invalid := configuration(2, 0)
	if _, err := store.InstallIfNewer(invalid); err == nil {
		t.Fatal("invalid configuration was accepted")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 1 {
		t.Fatalf("version = %d", loaded.Version)
	}
	if _, err := store.InstallIfNewer(configuration(0, 5)); err == nil {
		t.Fatal("rollback was accepted")
	}
}

func TestProfileDefinitionMustMatchReference(t *testing.T) {
	value := configuration(1, 5)
	value.VehicleProfileDefinition = json.RawMessage(`{"id":"different","signals":[]}`)
	if err := value.Validate(); err == nil {
		t.Fatal("mismatched profile was accepted")
	}
}

func TestParkedIntervalSupportsNewAndLegacyConfiguration(t *testing.T) {
	legacy := Interval{DefaultSeconds: 30}
	if legacy.EffectiveParkedSeconds() != 30 {
		t.Fatalf("legacy parked interval = %d", legacy.EffectiveParkedSeconds())
	}
	configured := Interval{DefaultSeconds: 30, ParkedSeconds: 900}
	if configured.EffectiveParkedSeconds() != 900 {
		t.Fatalf("configured parked interval = %d", configured.EffectiveParkedSeconds())
	}
}
