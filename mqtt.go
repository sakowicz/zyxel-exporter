package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type MQTTConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

type mqttClient struct {
	c      paho.Client
	cfg    MQTTConfig
	device haDevice
	slug   string
	mu     sync.Mutex
	last   *SwitchData
}

type haDevice struct {
	Identifiers      []string   `json:"identifiers"`
	Connections      [][]string `json:"connections,omitempty"`
	Name             string     `json:"name"`
	Model            string     `json:"model"`
	Manufacturer     string     `json:"manufacturer"`
	SwVersion        string     `json:"sw_version,omitempty"`
	HwVersion        string     `json:"hw_version,omitempty"`
	SerialNumber    string     `json:"serial_number,omitempty"`
	ConfigurationURL string     `json:"configuration_url,omitempty"`
}

type haConfig struct {
	Name                      string   `json:"name"`
	UniqueID                  string   `json:"unique_id"`
	StateTopic                string   `json:"state_topic"`
	UnitOfMeasurement         string   `json:"unit_of_measurement"`
	DeviceClass               string   `json:"device_class,omitempty"`
	StateClass                string   `json:"state_class"`
	SuggestedDisplayPrecision int      `json:"suggested_display_precision"`
	Device                    haDevice `json:"device"`
}

func Connect(cfg MQTTConfig, info *SystemInfo, switchHost string) (*mqttClient, error) {
	mc := &mqttClient{
		cfg:    cfg,
		device: buildDevice(info, switchHost),
		slug:   deviceSlug(info),
	}

	opts := paho.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)).
		SetClientID("zyxel-poe-bridge-" + mc.slug).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetCleanSession(false).
		SetOnConnectHandler(func(_ paho.Client) {
			log.Println("MQTT connected")
			mc.mu.Lock()
			last := mc.last
			mc.mu.Unlock()
			mc.publishDiscovery(last)
		})

	mc.c = paho.NewClient(opts)
	if tok := mc.c.Connect(); tok.Wait() && tok.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", tok.Error())
	}

	return mc, nil
}

func buildDevice(info *SystemInfo, switchHost string) haDevice {
	d := haDevice{
		Manufacturer:     "Zyxel",
		Model:            "Zyxel PoE",
		Name:             "Zyxel PoE Switch",
		Identifiers:      []string{"zyxel_poe_switch"},
		ConfigurationURL: fmt.Sprintf("http://%s", switchHost),
	}
	if info == nil {
		return d
	}
	if info.SerialNumber != "" {
		d.Identifiers = []string{"zyxel_poe_" + info.SerialNumber}
		d.SerialNumber = info.SerialNumber
	}
	if info.MAC != "" {
		d.Connections = [][]string{{"mac", info.MAC}}
	}
	if info.Model != "" {
		d.Model = info.Model
	}
	if info.FirmwareVersion != "" {
		d.SwVersion = info.FirmwareVersion
	}
	if info.HardwareVersion != "" {
		d.HwVersion = info.HardwareVersion
	}
	switch {
	case info.SystemName != "":
		d.Name = info.SystemName
	case info.Model != "":
		d.Name = "Zyxel " + info.Model
	}
	return d
}

func deviceSlug(info *SystemInfo) string {
	if info != nil && info.SerialNumber != "" {
		return strings.ToLower(info.SerialNumber)
	}
	if info != nil && info.MAC != "" {
		return strings.ReplaceAll(strings.ToLower(info.MAC), ":", "")
	}
	return "switch"
}

func (mc *mqttClient) stateTopic(id string) string {
	return fmt.Sprintf("zyxel/%s/%s/state", mc.slug, id)
}

func (mc *mqttClient) discoveryTopic(id string) string {
	return fmt.Sprintf("homeassistant/sensor/zyxel_poe_%s/%s/config", mc.slug, id)
}

func (mc *mqttClient) uniqueID(id string) string {
	return fmt.Sprintf("zyxel_poe_%s_%s", mc.slug, id)
}

func Publish(mc *mqttClient, data *SwitchData) error {
	mc.mu.Lock()
	var prevPorts int
	if mc.last != nil {
		prevPorts = len(mc.last.Ports)
	}
	mc.last = data
	mc.mu.Unlock()

	// publish discovery when port count changes (includes first run: 0 → N)
	if prevPorts != len(data.Ports) {
		mc.publishDiscovery(data)
	}

	states := map[string]string{
		"total_power":       fmt.Sprintf("%.1f", data.TotalPower),
		"consuming_power":   fmt.Sprintf("%.1f", data.ConsumingPower),
		"remaining_power":   fmt.Sprintf("%.1f", data.RemainingPower),
		"poe_usage_percent": fmt.Sprintf("%d", data.PoEUsagePercent),
		"junction_temp":     fmt.Sprintf("%d", data.JunctionTempC),
	}
	for _, p := range data.Ports {
		states[fmt.Sprintf("port_%d_power", p.Port)] = fmt.Sprintf("%.1f", p.Consumption)
	}

	for id, val := range states {
		topic := mc.stateTopic(id)
		if tok := mc.c.Publish(topic, 0, false, val); tok.Wait() && tok.Error() != nil {
			return fmt.Errorf("publish %s: %w", topic, tok.Error())
		}
	}
	return nil
}

func (mc *mqttClient) publishDiscovery(data *SwitchData) {
	static := []struct {
		id        string
		name      string
		unit      string
		class     string
		precision int
	}{
		{"total_power", "Total Power", "W", "power", 1},
		{"consuming_power", "Consuming Power", "W", "power", 1},
		{"remaining_power", "Remaining Power", "W", "power", 1},
		{"poe_usage_percent", "PoE Usage", "%", "", 0},
		{"junction_temp", "Junction Temperature", "°C", "temperature", 0},
	}

	for _, s := range static {
		mc.publishSensorConfig(s.id, s.name, s.unit, s.class, s.precision)
	}

	if data != nil {
		for _, p := range data.Ports {
			id := fmt.Sprintf("port_%d_power", p.Port)
			name := fmt.Sprintf("Port %d Power", p.Port)
			mc.publishSensorConfig(id, name, "W", "power", 1)
		}
	}
}

func (mc *mqttClient) publishSensorConfig(id, name, unit, class string, precision int) {
	cfg := haConfig{
		Name:                      name,
		UniqueID:                  mc.uniqueID(id),
		StateTopic:                mc.stateTopic(id),
		UnitOfMeasurement:         unit,
		DeviceClass:               class,
		StateClass:                "measurement",
		SuggestedDisplayPrecision: precision,
		Device:                    mc.device,
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("marshal discovery %s: %v", id, err)
		return
	}
	topic := mc.discoveryTopic(id)
	if tok := mc.c.Publish(topic, 1, true, payload); tok.Wait() && tok.Error() != nil {
		log.Printf("publish discovery %s: %v", id, tok.Error())
	}
}
