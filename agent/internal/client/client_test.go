package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Buco7854/vehinode/agent/internal/model"
)

func TestExplicitHTTPEnrollmentAndBatchTransport(t *testing.T) {
	var authorization string
	var enrollment enrollmentRequest
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/device/enroll":
			json.NewDecoder(request.Body).Decode(&enrollment)
			json.NewEncoder(response).Encode(map[string]any{"device_id": "device-1", "vehicle_id": "vehicle-1", "credential": "secret", "config": map[string]any{"version": 1, "sampling": map[string]any{"default_seconds": 5}, "upload": map[string]any{"default_seconds": 30}, "vehicle_profile": nil}})
		case "/api/v1/device/telemetry/batch":
			authorization = request.Header.Get("Authorization")
			var body struct {
				Samples []model.Sample `json:"samples"`
			}
			json.NewDecoder(request.Body).Decode(&body)
			json.NewEncoder(response).Encode(map[string]any{"accepted": []string{body.Samples[0].ID}, "duplicates": []string{}, "config_version": 7})
		}
	}))
	defer server.Close()
	enrolled, err := Enroll(server.URL, "one-time-token-value", "tracker", "test", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	api, err := New(server.URL, enrolled.Credential, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	sample := model.NewSample(1, nil, nil, nil)
	result, err := api.Upload(model.NewUUID(), []model.Sample{sample})
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.AgentVersion != "test" || enrollment.Token != "one-time-token-value" {
		t.Fatalf("unexpected enrollment metadata: %#v", enrollment)
	}
	if len(result.Acknowledged) != 1 || result.Acknowledged[0] != sample.ID || result.ConfigVersion != 7 || authorization != "Device secret" {
		t.Fatalf("unexpected upload: %#v %q", result, authorization)
	}
}

func TestServerURLValidation(t *testing.T) {
	for _, value := range []string{"http://192.168.1.151:8000", "http://localhost.evil.example", "https://user:password@example.com", "https://example.com/path", "ftp://example.com"} {
		if _, err := NormalizeServerURL(value, false); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
	if normalized, err := NormalizeServerURL("http://192.168.1.151:8000", true); err != nil || normalized != "http://192.168.1.151:8000" {
		t.Fatalf("explicit LAN HTTP was rejected: %q %v", normalized, err)
	}
	for _, value := range []string{"https://cars.example/", "http://localhost:8000", "http://[::1]:8000"} {
		if _, err := NormalizeServerURL(value, false); err != nil {
			t.Errorf("rejected %s: %v", value, err)
		}
	}
}
