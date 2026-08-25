package providers

import (
	"time"

	"github.com/Buco7854/vehinode/agent/internal/model"
	"github.com/Buco7854/vehinode/agent/internal/profile"
)

type ProfileProvider struct {
	adapter   *OBDAdapter
	decoder   *profile.DecoderEngine
	connected bool
	metrics   map[string]any
}

func NewProfileProvider(adapter *OBDAdapter, decoder *profile.DecoderEngine) *ProfileProvider {
	return &ProfileProvider{adapter: adapter, decoder: decoder, metrics: map[string]any{}}
}

func (provider *ProfileProvider) ReadMetrics() (map[string]any, error) {
	if !provider.connected {
		if err := provider.adapter.Connect(); err != nil {
			return observedMetrics(provider.metrics, nil), nil
		}
		if err := provider.adapter.SelectProtocol("6"); err != nil {
			provider.adapter.Close()
			return observedMetrics(provider.metrics, nil), nil
		}
		provider.connected = true
	}
	freshMovement := map[string]bool{}
	err := provider.adapter.Monitor(time.Second, func(frame model.CANFrame) {
		for _, decoded := range provider.decoder.Decode(frame, provider.metrics) {
			provider.metrics[decoded.Name] = decoded.Value
			if decoded.Name == "vehicle.speed" || decoded.Name == "vehicle.ready" || decoded.Name == "vehicle.ignition" {
				freshMovement[decoded.Name] = true
			}
		}
	})
	if err != nil {
		provider.adapter.Close()
		provider.connected = false
	}
	return observedMetrics(provider.metrics, freshMovement), nil
}

func observedMetrics(source map[string]any, freshMovement map[string]bool) map[string]any {
	result := copyMetrics(source)
	for _, name := range []string{"vehicle.speed", "vehicle.ready", "vehicle.ignition"} {
		if !freshMovement[name] {
			delete(result, name)
		}
	}
	return result
}

func (provider *ProfileProvider) Close() { provider.adapter.Close() }

func copyMetrics(source map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range source {
		result[key] = value
	}
	return result
}
