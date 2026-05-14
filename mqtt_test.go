package main

import (
	"testing"
)

func TestBuildDevice_Nil(t *testing.T) {
	// Fallback path: system-information fetch failed at startup.
	d := buildDevice(nil, "192.168.1.1")

	if d.Manufacturer != "Zyxel" {
		t.Errorf("Manufacturer = %q, want Zyxel", d.Manufacturer)
	}
	if d.Model != "Zyxel PoE" {
		t.Errorf("Model = %q, want fallback 'Zyxel PoE'", d.Model)
	}
	if d.Name != "Zyxel PoE Switch" {
		t.Errorf("Name = %q, want fallback 'Zyxel PoE Switch'", d.Name)
	}
	if len(d.Identifiers) != 1 || d.Identifiers[0] != "zyxel_poe_switch" {
		t.Errorf("Identifiers = %v, want [zyxel_poe_switch]", d.Identifiers)
	}
	if d.ConfigurationURL != "http://192.168.1.1" {
		t.Errorf("ConfigurationURL = %q, want http://192.168.1.1", d.ConfigurationURL)
	}
	if d.SerialNumber != "" {
		t.Errorf("SerialNumber = %q, want empty", d.SerialNumber)
	}
	if len(d.Connections) != 0 {
		t.Errorf("Connections = %v, want empty", d.Connections)
	}
}

func TestBuildDevice_FullInfo(t *testing.T) {
	info := &SystemInfo{
		Model:           "XMG1915-10EP",
		SystemName:      "branch",
		MAC:             "70:49:a2:56:bc:30",
		SerialNumber:    "S252L23001041",
		FirmwareVersion: "V4.80(ACGP.3)",
		HardwareVersion: "V1.16",
	}
	d := buildDevice(info, "192.168.1.1")

	if d.Model != "XMG1915-10EP" {
		t.Errorf("Model = %q, want XMG1915-10EP", d.Model)
	}
	// Name should use System Name when set.
	if d.Name != "branch" {
		t.Errorf("Name = %q, want branch", d.Name)
	}
	if d.SerialNumber != "S252L23001041" {
		t.Errorf("SerialNumber = %q, want S252L23001041", d.SerialNumber)
	}
	if d.SwVersion != "V4.80(ACGP.3)" {
		t.Errorf("SwVersion = %q, want V4.80(ACGP.3)", d.SwVersion)
	}
	if d.HwVersion != "V1.16" {
		t.Errorf("HwVersion = %q, want V1.16", d.HwVersion)
	}
	if len(d.Identifiers) != 1 || d.Identifiers[0] != "zyxel_poe_S252L23001041" {
		t.Errorf("Identifiers = %v, want [zyxel_poe_S252L23001041]", d.Identifiers)
	}
	if len(d.Connections) != 1 {
		t.Fatalf("Connections = %v, want 1 entry", d.Connections)
	}
	if d.Connections[0][0] != "mac" || d.Connections[0][1] != "70:49:a2:56:bc:30" {
		t.Errorf("Connections[0] = %v, want [mac, 70:49:a2:56:bc:30]", d.Connections[0])
	}
}

func TestBuildDevice_NoSystemName(t *testing.T) {
	// System Name empty → fall back to "Zyxel <Model>".
	info := &SystemInfo{
		Model:        "XMG1915-10EP",
		SystemName:   "",
		SerialNumber: "S123",
	}
	d := buildDevice(info, "10.0.0.1")

	if d.Name != "Zyxel XMG1915-10EP" {
		t.Errorf("Name = %q, want 'Zyxel XMG1915-10EP' (model fallback)", d.Name)
	}
}

func TestDeviceSlug(t *testing.T) {
	cases := []struct {
		name string
		info *SystemInfo
		want string
	}{
		{"nil", nil, "switch"},
		{"empty SystemInfo", &SystemInfo{}, "switch"},
		{"serial preferred", &SystemInfo{SerialNumber: "S123", MAC: "AA:BB:CC:DD:EE:FF"}, "s123"},
		{"MAC fallback when no serial", &SystemInfo{MAC: "70:49:A2:56:BC:30"}, "7049a256bc30"},
		{"serial lowercased", &SystemInfo{SerialNumber: "S252L23001041"}, "s252l23001041"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deviceSlug(c.info); got != c.want {
				t.Errorf("deviceSlug = %q, want %q", got, c.want)
			}
		})
	}
}

func TestMQTTTopics(t *testing.T) {
	mc := &mqttClient{slug: "s123"}

	if got := mc.stateTopic("total_power"); got != "zyxel/s123/total_power/state" {
		t.Errorf("stateTopic = %q, want zyxel/s123/total_power/state", got)
	}
	if got := mc.discoveryTopic("port_1_power"); got != "homeassistant/sensor/zyxel_poe_s123/port_1_power/config" {
		t.Errorf("discoveryTopic = %q, want homeassistant/sensor/zyxel_poe_s123/port_1_power/config", got)
	}
	if got := mc.uniqueID("cpu_usage_percent"); got != "zyxel_poe_s123_cpu_usage_percent" {
		t.Errorf("uniqueID = %q, want zyxel_poe_s123_cpu_usage_percent", got)
	}
}
