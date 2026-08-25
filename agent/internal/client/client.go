package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/Buco7854/vehinode/agent/internal/model"
	"github.com/Buco7854/vehinode/agent/internal/store"
)

type Client struct {
	serverURL  string
	credential string
	version    string
	http       *http.Client
}

type EnrollmentResponse struct {
	DeviceID   string              `json:"device_id"`
	VehicleID  string              `json:"vehicle_id"`
	Credential string              `json:"credential"`
	Config     store.Configuration `json:"config"`
}

type UploadResponse struct {
	Acknowledged  []string
	ConfigVersion int
}

type enrollmentRequest struct {
	Token        string         `json:"token"`
	AgentVersion string         `json:"agent_version"`
	Hostname     string         `json:"hostname"`
	Hardware     map[string]any `json:"hardware"`
}

func NormalizeServerURL(value string, allowInsecureHTTP bool) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("agent server URL must be an origin without credentials or a path")
	}
	if parsed.Scheme == "https" {
		return strings.TrimSuffix(value, "/"), nil
	}
	host := parsed.Hostname()
	loopback := host == "localhost"
	if address := net.ParseIP(host); address != nil {
		loopback = address.IsLoopback()
	}
	if parsed.Scheme == "http" && (loopback || allowInsecureHTTP) {
		return strings.TrimSuffix(value, "/"), nil
	}
	return "", fmt.Errorf("agent server URL must use HTTPS except when insecure HTTP was explicitly allowed")
}

func New(serverURL, credential, version string, allowInsecureHTTP bool) (*Client, error) {
	normalized, err := NormalizeServerURL(serverURL, allowInsecureHTTP)
	if err != nil {
		return nil, err
	}
	return &Client{
		serverURL: normalized, credential: credential, version: version,
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func Enroll(serverURL, token, hostname, version string, hardware map[string]any, allowInsecureHTTP bool) (EnrollmentResponse, error) {
	api, err := New(serverURL, "", version, allowInsecureHTTP)
	if err != nil {
		return EnrollmentResponse{}, err
	}
	var response EnrollmentResponse
	err = api.request(http.MethodPost, "/api/v1/device/enroll", enrollmentRequest{
		Token: token, AgentVersion: version, Hostname: hostname, Hardware: hardware,
	}, &response, false)
	if err != nil {
		return response, fmt.Errorf("device enrollment failed: %w", err)
	}
	if response.DeviceID == "" || response.VehicleID == "" || response.Credential == "" {
		return response, fmt.Errorf("device enrollment response is incomplete")
	}
	if err := response.Config.Validate(); err != nil {
		return response, fmt.Errorf("device enrollment configuration is invalid: %w", err)
	}
	return response, nil
}

func (client *Client) FetchConfiguration() (store.Configuration, error) {
	var configuration store.Configuration
	if err := client.request(http.MethodGet, "/api/v1/device/config", nil, &configuration, true); err != nil {
		return configuration, fmt.Errorf("configuration sync failed: %w", err)
	}
	return configuration, nil
}

func (client *Client) Upload(bootID string, samples []model.Sample) (UploadResponse, error) {
	payload := struct {
		BootID  string         `json:"boot_id"`
		Samples []model.Sample `json:"samples"`
	}{BootID: bootID, Samples: samples}
	var response struct {
		Accepted      []string `json:"accepted"`
		Duplicates    []string `json:"duplicates"`
		ConfigVersion int      `json:"config_version"`
	}
	if err := client.request(http.MethodPost, "/api/v1/device/telemetry/batch", payload, &response, true); err != nil {
		return UploadResponse{}, fmt.Errorf("telemetry upload failed: %w", err)
	}
	return UploadResponse{
		Acknowledged:  append(response.Accepted, response.Duplicates...),
		ConfigVersion: response.ConfigVersion,
	}, nil
}

func (client *Client) Download(path string, authenticated bool) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, client.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	client.headers(request, authenticated)
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, responseError(response)
	}
	return io.ReadAll(io.LimitReader(response.Body, 100*1024*1024))
}

func (client *Client) request(method, path string, body, destination any, authenticated bool) error {
	var source io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		source = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, client.serverURL+path, source)
	if err != nil {
		return err
	}
	client.headers(request, authenticated)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	if destination == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4*1024*1024)).Decode(destination); err != nil {
		return fmt.Errorf("invalid server response: %w", err)
	}
	return nil
}

func (client *Client) headers(request *http.Request, authenticated bool) {
	request.Header.Set("User-Agent", fmt.Sprintf("VehiNode-Agent/%s (%s/%s)", client.version, runtime.GOOS, runtime.GOARCH))
	if authenticated {
		request.Header.Set("Authorization", "Device "+client.credential)
	}
}

func responseError(response *http.Response) error {
	content, _ := io.ReadAll(io.LimitReader(response.Body, 16*1024))
	var payload struct {
		Detail any `json:"detail"`
	}
	if json.Unmarshal(content, &payload) == nil && payload.Detail != nil {
		return fmt.Errorf("server returned %s: %v", response.Status, payload.Detail)
	}
	return fmt.Errorf("server returned %s", response.Status)
}
