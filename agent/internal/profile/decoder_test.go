package profile

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/Buco7854/vehinode/agent/internal/model"
)

func TestBuiltInProfileDecode(t *testing.T) {
	decoder, err := FromFile(filepath.Join("..", "..", "profiles", "citroen-c-zero-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	decoded := decoder.Decode(model.CANFrame{CANID: 0x374, Data: []byte{150, 140, 0, 0, 90, 80, 32, 0}}, nil)
	values := map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	for name, expected := range map[string]float64{
		"battery.soc":                  70,
		"battery.soc_secondary":        65,
		"battery.cell_temperature_max": 40,
		"battery.cell_temperature_min": 30,
		"battery.capacity_full":        16,
	} {
		if math.Abs(values[name]-expected) > .001 {
			t.Fatalf("%s=%v, want %v (all values=%#v)", name, values[name], expected, values)
		}
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x373, Data: []byte{120, 105, 0x80, 0x64, 0x0c, 0xe4, 0, 0}}, nil)
	values = map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	if math.Abs(values["battery.current"]-1) > .001 ||
		math.Abs(values["battery.pack_voltage"]-330) > .001 ||
		math.Abs(values["battery.cell_voltage_max"]-3.3) > .001 ||
		math.Abs(values["battery.cell_voltage_min"]-3.15) > .001 {
		t.Fatalf("values=%#v", values)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x389, Data: []byte{165, 230, 120, 90, 91, 0, 160, 0}}, nil)
	values = map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	for name, expected := range map[string]float64{
		"charging.dc_voltage":    330.5,
		"charging.ac_voltage":    230,
		"charging.dc_current":    12,
		"charging.temperature_1": 40,
		"charging.temperature_2": 41,
		"charging.ac_current":    16,
	} {
		if math.Abs(values[name]-expected) > .001 {
			t.Fatalf("%s=%v, want %v (all values=%#v)", name, values[name], expected, values)
		}
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x412, Data: []byte{0, 42, 0x01, 0xe2, 0x40, 0, 0, 0}}, nil)
	values = map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	if values["vehicle.speed"] != 42 || values["vehicle.odometer"] != 123456 {
		t.Fatalf("values=%#v", values)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x286, Data: []byte{0, 0, 0, 65, 0, 0, 0, 0}}, nil)
	if len(decoded) != 1 || decoded[0].Name != "environment.outdoor_temperature" || decoded[0].Value.(float64) != 15 {
		t.Fatalf("decoded=%#v", decoded)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x101, Data: []byte{4, 0, 0, 0, 0, 0, 0, 0}}, nil)
	state := map[string]any{}
	for _, signal := range decoded {
		state[signal.Name] = signal.Value
	}
	if state["vehicle.ready"] != true || state["vehicle.operating_state"] != "ready" {
		t.Fatalf("state=%#v", state)
	}
	decoded = decoder.Decode(model.CANFrame{CANID: 0x101, Data: []byte{0, 0, 0, 0, 0, 0, 0, 0}}, nil)
	state = map[string]any{}
	for _, signal := range decoded {
		state[signal.Name] = signal.Value
	}
	if state["vehicle.ready"] != false || state["vehicle.operating_state"] != "charging" {
		t.Fatalf("state=%#v", state)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x298, Data: []byte{0, 0, 0, 90, 0, 0, 0x2e, 0xe0}}, nil)
	values = map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	if values["motor.temperature"] != 50 || values["motor.rpm"] != 2000 {
		t.Fatalf("values=%#v", values)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x346, Data: []byte{0, 0, 0, 0, 0, 0, 0, 123}}, nil)
	if len(decoded) != 1 || decoded[0].Value.(float64) != 123 {
		t.Fatalf("decoded=%#v", decoded)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x384, Data: []byte{0, 0, 0, 0x11, 0, 0, 0, 0}}, nil)
	state = map[string]any{}
	for _, signal := range decoded {
		state[signal.Name] = signal.Value
	}
	if state["warning.tpms"] != true || state["vehicle.parking_brake"] != true {
		t.Fatalf("state=%#v", state)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x424, Data: []byte{0, 0x64, 1, 0, 0, 0, 0, 0}}, nil)
	state = map[string]any{}
	for _, signal := range decoded {
		state[signal.Name] = signal.Value
	}
	if state["body.door_open"] != true || state["lighting.mode"] != "low_beam" || state["lighting.high_beam"] != true {
		t.Fatalf("state=%#v", state)
	}

	decoded = decoder.Decode(model.CANFrame{CANID: 0x3d3, Data: []byte{159, 75, 160, 76, 161, 77, 162, 78}}, nil)
	values = map[string]float64{}
	for _, signal := range decoded {
		values[signal.Name] = signal.Value.(float64)
	}
	for name, expected := range map[string]float64{
		"tire.front_left.pressure":     159 * 0.0157,
		"tire.front_left.temperature":  25,
		"tire.front_right.pressure":    160 * 0.0157,
		"tire.front_right.temperature": 26,
		"tire.rear_right.pressure":     161 * 0.0157,
		"tire.rear_right.temperature":  27,
		"tire.rear_left.pressure":      162 * 0.0157,
		"tire.rear_left.temperature":   28,
	} {
		if math.Abs(values[name]-expected) > .001 {
			t.Fatalf("%s=%v, want %v (all values=%#v)", name, values[name], expected, values)
		}
	}
}
