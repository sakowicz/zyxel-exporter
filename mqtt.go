package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	c    paho.Client
	cfg  MQTTConfig
	mu   sync.Mutex
	last *SwitchData
}

type haDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
}

type haConfig struct {
	Name                     string   `json:"name"`
	UniqueID                 string   `json:"unique_id"`
	StateTopic               string   `json:"state_topic"`
	UnitOfMeasurement        string   `json:"unit_of_measurement"`
	DeviceClass              string   `json:"device_class,omitempty"`
	StateClass               string   `json:"state_class"`
	SuggestedDisplayPrecision int     `json:"suggested_display_precision"`
	Device                   haDevice `json:"device"`
}

var device = haDevice{
	Identifiers:  []string{"zyxel_poe_switch"},
	Name:         "Zyxel PoE Switch",
	Model:        "Zyxel PoE",
	Manufacturer: "Zyxel",
}

func Connect(cfg MQTTConfig) (*mqttClient, error) {
	mc := &mqttClient{cfg: cfg}

	opts := paho.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)).
		SetClientID("zyxel-poe-bridge").
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetCleanSession(false).
		SetOnConnectHandler(func(_ paho.Client) {
			log.Println("MQTT connected")
			mc.mu.Lock()
			last := mc.last
			mc.mu.Unlock()
			publishDiscovery(mc.c, last)
		})

	mc.c = paho.NewClient(opts)
	if tok := mc.c.Connect(); tok.Wait() && tok.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", tok.Error())
	}

	return mc, nil
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
		publishDiscovery(mc.c, data)
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
		topic := fmt.Sprintf("zyxel/sensor/%s/state", id)
		if tok := mc.c.Publish(topic, 0, false, val); tok.Wait() && tok.Error() != nil {
			return fmt.Errorf("publish %s: %w", topic, tok.Error())
		}
	}
	return nil
}

func publishDiscovery(c paho.Client, data *SwitchData) {
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
		publishSensorConfig(c, s.id, s.name, s.unit, s.class, s.precision)
	}

	if data != nil {
		for _, p := range data.Ports {
			id := fmt.Sprintf("port_%d_power", p.Port)
			name := fmt.Sprintf("Port %d Power", p.Port)
			publishSensorConfig(c, id, name, "W", "power", 1)
		}
	}
}

func publishSensorConfig(c paho.Client, id, name, unit, class string, precision int) {
	cfg := haConfig{
		Name:                      name,
		UniqueID:                  "zyxel_poe_" + id,
		StateTopic:                fmt.Sprintf("zyxel/sensor/%s/state", id),
		UnitOfMeasurement:         unit,
		DeviceClass:               class,
		StateClass:                "measurement",
		SuggestedDisplayPrecision: precision,
		Device:                    device,
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		log.Printf("marshal discovery %s: %v", id, err)
		return
	}
	topic := fmt.Sprintf("homeassistant/sensor/zyxel_poe/%s/config", id)
	if tok := c.Publish(topic, 1, true, payload); tok.Wait() && tok.Error() != nil {
		log.Printf("publish discovery %s: %v", id, tok.Error())
	}
}
