package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Buco7854/vehinode/agent/internal/capture"
	"github.com/Buco7854/vehinode/agent/internal/client"
	"github.com/Buco7854/vehinode/agent/internal/model"
	"github.com/Buco7854/vehinode/agent/internal/profile"
	"github.com/Buco7854/vehinode/agent/internal/providers"
	agentruntime "github.com/Buco7854/vehinode/agent/internal/runtime"
	"github.com/Buco7854/vehinode/agent/internal/store"
	agentsystem "github.com/Buco7854/vehinode/agent/internal/system"
)

var (
	version     = "dev"
	buildTarget = "dev"
)

type paths struct{ config, data string }

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func execute(arguments []string) error {
	locations, remaining, err := globalArguments(arguments)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		usage()
		return fmt.Errorf("a command is required")
	}
	command, arguments := remaining[0], remaining[1:]
	switch command {
	case "version", "--version":
		fmt.Printf("VehiNode agent %s (%s)\n", version, buildTarget)
		return nil
	case "install":
		return commandInstall(locations, arguments)
	case "update":
		return commandUpdate(locations, arguments)
	case "uninstall":
		return commandUninstall(arguments)
	case "run":
		return commandRun(locations, arguments)
	case "status":
		return commandStatus(locations)
	case "doctor":
		return commandDoctor(locations)
	case "logs":
		return runAttached("journalctl", "-u", agentsystem.ServiceName, "-n", "200", "--no-pager")
	case "config":
		return printFile(filepath.Join(locations.config, "config.json"))
	case "devices":
		return commandDevices(locations, arguments)
	case "gps-info":
		return commandGPS(locations, arguments)
	case "obd-info":
		return commandOBD(locations, arguments)
	case "can-record":
		return commandRecord(locations, arguments)
	case "replay-can":
		return commandReplay(arguments)
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func globalArguments(arguments []string) (paths, []string, error) {
	locations := paths{config: agentsystem.ConfigDir, data: agentsystem.DataDir}
	for len(arguments) > 0 {
		switch arguments[0] {
		case "--config-dir", "--data-dir":
			if len(arguments) < 2 {
				return locations, nil, fmt.Errorf("%s requires a value", arguments[0])
			}
			if arguments[0] == "--config-dir" {
				locations.config = arguments[1]
			} else {
				locations.data = arguments[1]
			}
			arguments = arguments[2:]
		default:
			return locations, arguments, nil
		}
	}
	return locations, arguments, nil
}

func usage() {
	fmt.Print(`Usage: vehinode-agent [--config-dir PATH] [--data-dir PATH] COMMAND

Commands:
  install       Enroll this host and install its systemd service
  update        Download and verify another agent version
  uninstall     Remove the service, credentials, configuration, and queue
  run           Run the telemetry service
  status        Show credentials and queued telemetry
  doctor        Show local hardware and installation diagnostics
  logs          Show recent service logs
  config        Print the accepted last-known-good configuration
  devices       Show device choices; use "devices set" to change them
  gps-info      Read and print valid NMEA position fixes
  obd-info      Read adapter identity, VIN, and diagnostic codes
  can-record    Record CAN frames to a portable JSONL capture
  replay-can    Replay a capture, optionally through a profile
  version       Print build version and target
`)
}

func commandInstall(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	server := flags.String("server", "", "VehiNode server origin")
	token := flags.String("token", "", "one-time enrollment token")
	allowHTTP := flags.Bool("allow-insecure-http", false, "allow clear-text HTTP")
	updateOnly := flags.Bool("update-only", false, "refresh service without enrollment")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *server == "" {
		return fmt.Errorf("--server is required")
	}
	if !*updateOnly && *token == "" {
		return fmt.Errorf("--token is required for initial enrollment")
	}
	if err := agentsystem.SetupIdentityAndDirectories(); err != nil {
		return err
	}
	if !*updateOnly {
		hostname, _ := os.Hostname()
		modelName := strings.TrimSpace(strings.TrimRight(string(readOptional("/proc/device-tree/model")), "\x00"))
		response, err := client.Enroll(*server, *token, hostname, version, map[string]any{
			"os": runtime.GOOS, "architecture": runtime.GOARCH, "target": buildTarget, "model": modelName,
		}, *allowHTTP)
		if err != nil {
			return err
		}
		normalized, _ := client.NormalizeServerURL(*server, *allowHTTP)
		credentials := store.Credentials{ServerURL: normalized, DeviceID: response.DeviceID, VehicleID: response.VehicleID, Credential: response.Credential, AllowInsecureHTTP: *allowHTTP}
		credentialsPath := filepath.Join(locations.config, "credentials.json")
		configPath := filepath.Join(locations.config, "config.json")
		if err := store.WriteJSONAtomic(credentialsPath, credentials, 0o600); err != nil {
			return err
		}
		if _, err := (store.ConfigurationStore{Path: configPath}).InstallIfNewer(response.Config); err != nil {
			return err
		}
		if err := agentsystem.ChownAgent(credentialsPath); err != nil {
			return err
		}
		if err := agentsystem.ChownAgent(configPath); err != nil {
			return err
		}
		fmt.Printf("Enrolled device %s\n", response.DeviceID)
	}
	if err := agentsystem.InstallService(); err != nil {
		return err
	}
	fmt.Printf("VehiNode agent %s installed. Run: sudo vehinode-agent doctor\n", version)
	return nil
}

func commandUpdate(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	requested := flags.String("version", "", "version to install")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *requested == "" {
		return fmt.Errorf("--version is required")
	}
	credentials, err := loadCredentials(locations)
	if err != nil {
		return err
	}
	api, err := client.New(credentials.ServerURL, credentials.Credential, version, credentials.AllowInsecureHTTP)
	if err != nil {
		return err
	}
	target, err := agentsystem.DetectTarget(buildTarget)
	if err != nil {
		return err
	}
	if err := agentsystem.Update(api, *requested, target); err != nil {
		return err
	}
	fmt.Printf("VehiNode agent updated to %s\n", *requested)
	return nil
}

func commandUninstall(arguments []string) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := flags.Bool("yes", false, "skip confirmation")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	return agentsystem.Uninstall(*yes)
}

func commandStatus(locations paths) error {
	credentials := filepath.Join(locations.config, "credentials.json")
	queue, err := store.OpenQueue(filepath.Join(locations.data, "queue.sqlite3"))
	if err != nil {
		return err
	}
	defer queue.Close()
	depth, err := queue.Depth()
	if err != nil {
		return err
	}
	installed := fileExists(credentials)
	fmt.Printf("VehiNode agent %s\nCredentials: %s\nQueued telemetry: %d\n", version, map[bool]string{true: "installed", false: "missing"}[installed], depth)
	if !installed {
		return fmt.Errorf("credentials are missing")
	}
	return nil
}

func commandDoctor(locations paths) error {
	hardware, hardwareErr := (store.HardwareStore{Path: filepath.Join(locations.config, "hardware.json")}).Load()
	gpsCandidates, obdCandidates := store.GPSCandidates(), store.OBDCandidates()
	result := map[string]any{
		"version": version, "target": buildTarget, "credentials": fileExists(filepath.Join(locations.config, "credentials.json")),
		"queue_directory_writable": directoryWritable(locations.data), "gps_candidates": gpsCandidates, "obd_candidates": obdCandidates,
	}
	if hardwareErr != nil {
		result["hardware_error"] = hardwareErr.Error()
	} else {
		result["hardware_selection"] = hardware
		result["gps_device"] = nullIfEmpty(store.Resolve(hardware.GPS, gpsCandidates))
		result["obd_device"] = nullIfEmpty(store.Resolve(hardware.OBD, obdCandidates))
	}
	printJSON(result)
	if hardwareErr != nil || result["credentials"] != true || result["queue_directory_writable"] != true {
		return fmt.Errorf("one or more diagnostic checks failed")
	}
	return nil
}

func commandDevices(locations paths, arguments []string) error {
	hardwareStore := store.HardwareStore{Path: filepath.Join(locations.config, "hardware.json")}
	hardware, err := hardwareStore.Load()
	if err != nil {
		return err
	}
	if len(arguments) > 0 && arguments[0] == "set" {
		flags := flag.NewFlagSet("devices set", flag.ContinueOnError)
		gps := flags.String("gps", "", "auto, off, or /dev path")
		obd := flags.String("obd", "", "auto, off, or /dev path")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if *gps == "" && *obd == "" {
			return fmt.Errorf("choose --gps and/or --obd")
		}
		if *gps != "" {
			hardware.GPS = *gps
		}
		if *obd != "" {
			hardware.OBD = *obd
		}
		if err := hardware.Validate(); err != nil {
			return err
		}
		for source, value := range map[string]string{"GPS": hardware.GPS, "OBD": hardware.OBD} {
			if value != store.Auto && value != store.Off && !fileExists(value) {
				return fmt.Errorf("%s device does not exist: %s", source, value)
			}
		}
		if err := hardwareStore.Save(hardware); err != nil {
			return err
		}
		printJSON(hardware)
		fmt.Println("Saved. Restart with: sudo systemctl restart vehinode-agent")
		return nil
	}
	if len(arguments) > 0 {
		return fmt.Errorf("unknown devices action %q", arguments[0])
	}
	gps, obd := store.GPSCandidates(), store.OBDCandidates()
	printJSON(map[string]any{"selection": hardware, "resolved": map[string]any{"gps": nullIfEmpty(store.Resolve(hardware.GPS, gps)), "obd": nullIfEmpty(store.Resolve(hardware.OBD, obd))}, "candidates": map[string]any{"gps": gps, "obd": obd}})
	return nil
}

func commandGPS(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("gps-info", flag.ContinueOnError)
	device := flags.String("device", "", "serial device")
	seconds := flags.Int("seconds", 10, "read duration in seconds")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *seconds <= 0 {
		return fmt.Errorf("--seconds must be greater than zero")
	}
	if *device == "" {
		hardware, err := loadHardware(locations)
		if err != nil {
			return err
		}
		*device = store.Resolve(hardware.GPS, store.GPSCandidates())
	}
	if *device == "" {
		return fmt.Errorf("no NMEA serial device selected")
	}
	provider := providers.NewNMEAProvider(*device)
	defer provider.Close()
	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	for time.Now().Before(deadline) {
		fix, err := provider.Read()
		if err != nil {
			continue
		}
		if fix != nil {
			printJSON(fix)
		}
	}
	return nil
}

func commandOBD(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("obd-info", flag.ContinueOnError)
	device := flags.String("device", "", "serial device")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *device == "" {
		hardware, err := loadHardware(locations)
		if err != nil {
			return err
		}
		*device = store.Resolve(hardware.OBD, store.OBDCandidates())
	}
	if *device == "" {
		return fmt.Errorf("no OBD serial device selected")
	}
	adapter := providers.NewOBDAdapter(*device)
	if err := adapter.Connect(); err != nil {
		return err
	}
	defer adapter.Close()
	identity, err := adapter.Identity()
	if err != nil {
		return err
	}
	result := map[string]any{"device": *device, "adapter": identity["adapter"], "firmware": identity["firmware"]}
	if lines, err := adapter.Command("0902", 0); err == nil {
		result["vin"] = nullIfEmpty(providers.ParseVIN(lines))
	}
	if lines, err := adapter.Command("03", 0); err == nil {
		result["dtcs"] = providers.ParseDTC(lines)
	} else {
		result["dtcs"] = []string{}
	}
	printJSON(result)
	return nil
}

func commandRecord(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("can-record", flag.ContinueOnError)
	device := flags.String("device", "", "serial device")
	profileID := flags.String("profile", "", "profile id")
	seconds := flags.Int("seconds", 30, "record duration in seconds")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *seconds <= 0 {
		return fmt.Errorf("--seconds must be greater than zero")
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("capture output path is required")
	}
	if *device == "" {
		hardware, err := loadHardware(locations)
		if err != nil {
			return err
		}
		*device = store.Resolve(hardware.OBD, store.OBDCandidates())
	}
	if *device == "" {
		return fmt.Errorf("no OBD serial device selected")
	}
	adapter := providers.NewOBDAdapter(*device)
	if err := adapter.Connect(); err != nil {
		return err
	}
	defer adapter.Close()
	if err := adapter.SelectProtocol("6"); err != nil {
		return err
	}
	output, err := os.Create(flags.Arg(0))
	if err != nil {
		return err
	}
	defer output.Close()
	recorder, err := capture.NewRecorder(output, map[string]any{"adapter": *device, "vehicle_profile": *profileID})
	if err != nil {
		return err
	}
	if err := adapter.Monitor(time.Duration(*seconds)*time.Second, func(frame model.CANFrame) {
		if err := recorder.Write(frame); err != nil {
			fmt.Fprintln(os.Stderr, "capture write failed:", err)
		}
	}); err != nil {
		return err
	}
	fmt.Printf("Capture written to %s\n", flags.Arg(0))
	return nil
}

func commandReplay(arguments []string) error {
	flags := flag.NewFlagSet("replay-can", flag.ContinueOnError)
	profilePath := flags.String("profile", "", "YAML or JSON profile")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("capture path is required")
	}
	recording, err := capture.Read(flags.Arg(0))
	if err != nil {
		return err
	}
	var decoder *profile.DecoderEngine
	if *profilePath != "" {
		decoder, err = profile.FromFile(*profilePath)
		if err != nil {
			return err
		}
	}
	metrics := map[string]any{}
	for _, frame := range recording.Frames {
		row := map[string]any{"type": "frame", "timestamp": frame.Timestamp, "can_id": fmt.Sprintf("0x%03X", frame.CANID), "data": strings.ToUpper(hex.EncodeToString(frame.Data)), "metadata": recording.Metadata}
		if decoder != nil {
			decoded := decoder.Decode(frame, metrics)
			row["signals"] = decoded
			for _, signal := range decoded {
				metrics[signal.Name] = signal.Value
			}
		}
		printJSON(row)
	}
	fmt.Fprintf(os.Stderr, "Replayed %d frames\n", len(recording.Frames))
	return nil
}

func commandRun(locations paths, arguments []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	gpsOverride := flags.String("gps-device", "", "override GPS path")
	obdOverride := flags.String("obd-device", "", "override OBD path")
	syncSeconds := flags.Int("config-sync-seconds", 21600, "configuration fallback sync interval in seconds")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *syncSeconds <= 0 {
		return fmt.Errorf("--config-sync-seconds must be greater than zero")
	}
	credentials, err := loadCredentials(locations)
	if err != nil {
		return err
	}
	configurationStore := store.ConfigurationStore{Path: filepath.Join(locations.config, "config.json")}
	configuration, err := configurationStore.Load()
	if err != nil {
		return err
	}
	hardware, err := loadHardware(locations)
	if err != nil {
		return err
	}
	gpsDevice, obdDevice := *gpsOverride, *obdOverride
	if gpsDevice == "" {
		gpsDevice = store.Resolve(hardware.GPS, store.GPSCandidates())
	}
	if obdDevice == "" {
		obdDevice = store.Resolve(hardware.OBD, store.OBDCandidates())
	}
	if err := agentruntime.ValidateDistinctDevices(gpsDevice, obdDevice); err != nil {
		return err
	}
	api, err := client.New(credentials.ServerURL, credentials.Credential, version, credentials.AllowInsecureHTTP)
	if err != nil {
		return err
	}
	queue, err := store.OpenQueue(filepath.Join(locations.data, "queue.sqlite3"))
	if err != nil {
		return err
	}
	defer queue.Close()
	position := agentruntime.PositionProvider(agentruntime.EmptyPosition{})
	if gpsDevice != "" {
		provider := providers.NewNMEAProvider(gpsDevice)
		defer provider.Close()
		position = provider
	}
	vehicle, err := vehicleProvider(obdDevice, configuration)
	if err != nil {
		return err
	}
	defer vehicle.Close()
	sequence, _ := agentruntime.LastSequence(queue)
	agent := &agentruntime.Agent{Queue: queue, Client: api, Position: position, Vehicle: vehicle, BootID: model.NewUUID(), Sequence: sequence}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	now := time.Now()
	nextObservation, nextSample, nextUpload, nextSync := now, now, now, now
	detector := agentruntime.NewParkedDetector(now)
	parked := false
	latestObservation := agentruntime.Observation{}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		now = time.Now()
		if !now.Before(nextSync) {
			remote, fetchErr := api.FetchConfiguration()
			if fetchErr == nil {
				candidate, installErr := configurationStore.InstallIfNewer(remote)
				if installErr == nil {
					if !reflect.DeepEqual(candidate, configuration) {
						replacement, replacementErr := vehicleProvider(obdDevice, candidate)
						if replacementErr == nil {
							agent.Vehicle.Close()
							agent.Vehicle = replacement
							configuration = candidate
							nextSample = now
							nextUpload = now
						} else {
							fmt.Fprintln(os.Stderr, "Configuration sync retained last-known-good:", replacementErr)
						}
					}
				} else {
					fmt.Fprintln(os.Stderr, "Configuration sync retained last-known-good:", installErr)
				}
			} else {
				fmt.Fprintln(os.Stderr, fetchErr)
			}
			nextSync = now.Add(time.Duration(*syncSeconds) * time.Second)
		}
		if !now.Before(nextObservation) {
			observation := agent.Observe()
			if observation.Position != nil {
				latestObservation.Position = observation.Position
			}
			latestObservation.Metrics = observation.Metrics
			var changed bool
			parked, changed = detector.Observe(now, observation)
			if changed {
				nextSample = now
				nextUpload = now
			}
			nextObservation = now.Add(time.Second)
		}
		samplingSeconds := configuration.Sampling.DefaultSeconds
		uploadSeconds := configuration.Upload.DefaultSeconds
		if parked {
			samplingSeconds = configuration.Sampling.EffectiveParkedSeconds()
			uploadSeconds = configuration.Upload.EffectiveParkedSeconds()
		}
		if !now.Before(nextSample) {
			if _, err := agent.EnqueueObservation(latestObservation); err != nil {
				fmt.Fprintln(os.Stderr, "Collection failed:", err)
			} else {
				nextUpload = now
			}
			nextSample = now.Add(time.Duration(samplingSeconds) * time.Second)
		}
		if !now.Before(nextUpload) {
			result, err := agent.Upload(500)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				retrySeconds := min(uploadSeconds, 30)
				nextUpload = now.Add(time.Duration(retrySeconds) * time.Second)
			} else if result.ConfigVersion > configuration.Version {
				nextSync = now
				nextUpload = now.Add(time.Duration(uploadSeconds) * time.Second)
			} else {
				nextUpload = now.Add(time.Duration(uploadSeconds) * time.Second)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func vehicleProvider(device string, configuration store.Configuration) (agentruntime.VehicleProvider, error) {
	if device == "" {
		return agentruntime.EmptyVehicle{}, nil
	}
	adapter := providers.NewOBDAdapter(device)
	if configuration.VehicleProfile == nil {
		return providers.NewStandardOBDProvider(adapter), nil
	}
	if len(configuration.VehicleProfileDefinition) == 0 || string(configuration.VehicleProfileDefinition) == "null" {
		return nil, fmt.Errorf("selected profile has no server definition")
	}
	decoder, err := profile.ParseJSON(configuration.VehicleProfileDefinition)
	if err != nil {
		return nil, err
	}
	return providers.NewProfileProvider(adapter, decoder), nil
}

func loadCredentials(locations paths) (store.Credentials, error) {
	return agentsystem.LoadCredentials(filepath.Join(locations.config, "credentials.json"))
}
func loadHardware(locations paths) (store.Hardware, error) {
	return (store.HardwareStore{Path: filepath.Join(locations.config, "hardware.json")}).Load()
}
func readOptional(path string) []byte { content, _ := os.ReadFile(path); return content }
func fileExists(path string) bool     { _, err := os.Stat(path); return err == nil }
func directoryWritable(path string) bool {
	if err := os.MkdirAll(path, 0o750); err != nil {
		return false
	}
	file, err := os.CreateTemp(path, ".doctor-*")
	if err != nil {
		return false
	}
	name := file.Name()
	file.Close()
	os.Remove(name)
	return true
}
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func printJSON(value any) {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println(string(encoded))
}
func printFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Print(string(content))
	return nil
}
func runAttached(name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
