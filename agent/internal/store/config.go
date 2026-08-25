package store

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Buco7854/vehinode/agent/internal/profile"
)

type Configuration struct {
	Version                  int             `json:"version"`
	Sampling                 Interval        `json:"sampling"`
	Upload                   Interval        `json:"upload"`
	VehicleProfile           *string         `json:"vehicle_profile"`
	VehicleProfileDefinition json.RawMessage `json:"vehicle_profile_definition,omitempty"`
}

type Interval struct {
	DefaultSeconds int `json:"default_seconds"`
	ParkedSeconds  int `json:"parked_seconds,omitempty"`
}

func (configuration Configuration) Validate() error {
	if configuration.Version < 1 || !configuration.Sampling.valid() || !configuration.Upload.valid() {
		return fmt.Errorf("remote configuration values are outside safe bounds")
	}
	if len(configuration.VehicleProfileDefinition) > 0 && string(configuration.VehicleProfileDefinition) != "null" {
		if configuration.VehicleProfile == nil || *configuration.VehicleProfile == "" {
			return fmt.Errorf("profile definition has no profile reference")
		}
		decoder, err := profile.ParseJSON(configuration.VehicleProfileDefinition)
		if err != nil {
			return fmt.Errorf("vehicle profile definition is invalid: %w", err)
		}
		if decoder.ID() != *configuration.VehicleProfile {
			return fmt.Errorf("vehicle profile definition ID does not match its reference")
		}
	} else if configuration.VehicleProfile != nil {
		return fmt.Errorf("vehicle profile reference has no definition")
	}
	return nil
}

func (interval Interval) valid() bool {
	return interval.DefaultSeconds >= 1 && interval.DefaultSeconds <= 86400 &&
		interval.ParkedSeconds >= 0 && interval.ParkedSeconds <= 86400
}

func (interval Interval) EffectiveParkedSeconds() int {
	if interval.ParkedSeconds > 0 {
		return interval.ParkedSeconds
	}
	return interval.DefaultSeconds
}

type ConfigurationStore struct{ Path string }

func (store ConfigurationStore) Load() (Configuration, error) {
	var configuration Configuration
	if err := ReadJSON(store.Path, &configuration); err != nil {
		return configuration, fmt.Errorf("cannot load last-known-good configuration: %w", err)
	}
	if err := configuration.Validate(); err != nil {
		return configuration, fmt.Errorf("cannot load last-known-good configuration: %w", err)
	}
	return configuration, nil
}

func (store ConfigurationStore) InstallIfNewer(configuration Configuration) (Configuration, error) {
	if err := configuration.Validate(); err != nil {
		return Configuration{}, err
	}
	current, err := store.Load()
	if err == nil {
		if configuration.Version < current.Version {
			return Configuration{}, fmt.Errorf("refusing configuration version rollback")
		}
		if configuration.Version == current.Version {
			return current, nil
		}
	} else if !os.IsNotExist(rootCause(err)) {
		// Invalid last-known-good data may be repaired by a valid server response.
	}
	if err := WriteJSONAtomic(store.Path, configuration, 0o600); err != nil {
		return Configuration{}, err
	}
	return configuration, nil
}

func rootCause(err error) error {
	for {
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok || unwrapped.Unwrap() == nil {
			return err
		}
		err = unwrapped.Unwrap()
	}
}

type Credentials struct {
	ServerURL         string `json:"server_url"`
	DeviceID          string `json:"device_id"`
	VehicleID         string `json:"vehicle_id"`
	Credential        string `json:"credential"`
	AllowInsecureHTTP bool   `json:"allow_insecure_http,omitempty"`
}
