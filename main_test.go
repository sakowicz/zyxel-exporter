package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- loadConfig ---

func setZyxelEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ZYXEL_DEVICE_IP", "10.0.0.1")
	t.Setenv("ZYXEL_DEVICE_USERNAME", "admin")
	t.Setenv("ZYXEL_DEVICE_PASSWORD", "secret")
}

func TestLoadConfig_FullEnv(t *testing.T) {
	setZyxelEnv(t)
	t.Setenv("MQTT_BROKER_HOST", "mqtt.local")
	t.Setenv("MQTT_BROKER_PORT", "8883")
	t.Setenv("MQTT_BROKER_USERNAME", "mqttuser")
	t.Setenv("MQTT_BROKER_PASSWORD", "mqttpass")
	t.Setenv("CRON_SCHEDULE", "*/5 * * * *")
	t.Setenv("HTTP_LISTEN", ":9090")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if cfg.Zyxel.Host != "10.0.0.1" {
		t.Errorf("Zyxel.Host = %q, want 10.0.0.1", cfg.Zyxel.Host)
	}
	if cfg.Zyxel.Username != "admin" {
		t.Errorf("Zyxel.Username = %q, want admin", cfg.Zyxel.Username)
	}
	if cfg.Zyxel.Password != "secret" {
		t.Errorf("Zyxel.Password = %q, want secret", cfg.Zyxel.Password)
	}
	if cfg.MQTT.Host != "mqtt.local" {
		t.Errorf("MQTT.Host = %q, want mqtt.local", cfg.MQTT.Host)
	}
	if cfg.MQTT.Port != 8883 {
		t.Errorf("MQTT.Port = %d, want 8883", cfg.MQTT.Port)
	}
	if cfg.MQTT.Username != "mqttuser" {
		t.Errorf("MQTT.Username = %q, want mqttuser", cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "mqttpass" {
		t.Errorf("MQTT.Password = %q, want mqttpass", cfg.MQTT.Password)
	}
	if cfg.CronSchedule != "*/5 * * * *" {
		t.Errorf("CronSchedule = %q, want '*/5 * * * *'", cfg.CronSchedule)
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want :9090", cfg.ListenAddr)
	}
}

func TestLoadConfig_MQTTDisabled(t *testing.T) {
	// Critical: MQTT_BROKER_HOST unset → cfg.MQTT.Host empty → main() skips
	// Connect() and the exporter runs in Prometheus-only mode.
	setZyxelEnv(t)
	t.Setenv("MQTT_BROKER_HOST", "")
	t.Setenv("MQTT_BROKER_USERNAME", "")
	t.Setenv("MQTT_BROKER_PASSWORD", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if cfg.MQTT.Host != "" {
		t.Errorf("MQTT.Host = %q, want empty (MQTT disabled)", cfg.MQTT.Host)
	}
	// Port still defaults to 1883 even when MQTT is disabled; harmless.
	if cfg.MQTT.Port != 1883 {
		t.Errorf("MQTT.Port = %d, want default 1883", cfg.MQTT.Port)
	}
}

func TestLoadConfig_MQTTHostWithoutCreds(t *testing.T) {
	// Anonymous MQTT broker: host set, username/password empty. Should be
	// accepted (creds are optional).
	setZyxelEnv(t)
	t.Setenv("MQTT_BROKER_HOST", "mqtt.local")
	t.Setenv("MQTT_BROKER_USERNAME", "")
	t.Setenv("MQTT_BROKER_PASSWORD", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.MQTT.Host != "mqtt.local" {
		t.Errorf("MQTT.Host = %q, want mqtt.local", cfg.MQTT.Host)
	}
	if cfg.MQTT.Username != "" || cfg.MQTT.Password != "" {
		t.Errorf("MQTT credentials should be empty, got user=%q pass=%q", cfg.MQTT.Username, cfg.MQTT.Password)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	cases := []struct {
		name      string
		setEnv    func(*testing.T)
		wantInErr string
	}{
		{
			name: "no IP",
			setEnv: func(t *testing.T) {
				t.Setenv("ZYXEL_DEVICE_IP", "")
				t.Setenv("ZYXEL_DEVICE_USERNAME", "u")
				t.Setenv("ZYXEL_DEVICE_PASSWORD", "p")
			},
			wantInErr: "ZYXEL_DEVICE_IP",
		},
		{
			name: "no username",
			setEnv: func(t *testing.T) {
				t.Setenv("ZYXEL_DEVICE_IP", "1.2.3.4")
				t.Setenv("ZYXEL_DEVICE_USERNAME", "")
				t.Setenv("ZYXEL_DEVICE_PASSWORD", "p")
			},
			wantInErr: "ZYXEL_DEVICE_USERNAME",
		},
		{
			name: "no password",
			setEnv: func(t *testing.T) {
				t.Setenv("ZYXEL_DEVICE_IP", "1.2.3.4")
				t.Setenv("ZYXEL_DEVICE_USERNAME", "u")
				t.Setenv("ZYXEL_DEVICE_PASSWORD", "")
			},
			wantInErr: "ZYXEL_DEVICE_PASSWORD",
		},
		{
			name: "all missing",
			setEnv: func(t *testing.T) {
				t.Setenv("ZYXEL_DEVICE_IP", "")
				t.Setenv("ZYXEL_DEVICE_USERNAME", "")
				t.Setenv("ZYXEL_DEVICE_PASSWORD", "")
			},
			wantInErr: "ZYXEL_DEVICE_IP",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.setEnv(t)
			_, err := loadConfig()
			if err == nil {
				t.Fatal("loadConfig should return error when required env vars missing")
			}
			if !strings.Contains(err.Error(), c.wantInErr) {
				t.Errorf("error = %q, want to contain %q", err, c.wantInErr)
			}
		})
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	setZyxelEnv(t)
	t.Setenv("MQTT_BROKER_PORT", "")
	t.Setenv("CRON_SCHEDULE", "")
	t.Setenv("HTTP_LISTEN", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	if cfg.MQTT.Port != 1883 {
		t.Errorf("MQTT.Port = %d, want default 1883", cfg.MQTT.Port)
	}
	if cfg.CronSchedule != "* * * * *" {
		t.Errorf("CronSchedule = %q, want default '* * * * *'", cfg.CronSchedule)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want default :8080", cfg.ListenAddr)
	}
}

func TestLoadConfig_InvalidPortFallsBack(t *testing.T) {
	setZyxelEnv(t)
	t.Setenv("MQTT_BROKER_PORT", "not-a-number")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.MQTT.Port != 1883 {
		t.Errorf("MQTT.Port = %d, want fallback 1883 on unparseable input", cfg.MQTT.Port)
	}
}

// --- HTTP startup ---

func TestHTTPEndpoints(t *testing.T) {
	RegisterMetrics() // idempotent via sync.Once

	ts := httptest.NewServer(httpHandler())
	defer ts.Close()

	t.Run("health returns ok", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "ok" {
			t.Errorf("body = %q, want %q", body, "ok")
		}
	})

	t.Run("metrics exposes zyxel_ gauges", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/metrics")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		out := string(body)

		// Spot-check a handful of registered gauges — the scalar ones show
		// up immediately (per-port GaugeVecs only appear once a label is set
		// via a successful fetch, which we can't do without a real switch).
		mustContain := []string{
			"zyxel_cpu_usage_percent",
			"zyxel_memory_usage_percent",
			"zyxel_poe_total_watts",
			"zyxel_junction_temperature_celsius",
			"zyxel_mac_count",
		}
		for _, name := range mustContain {
			if !strings.Contains(out, name) {
				t.Errorf("/metrics output missing %q", name)
			}
		}
	})

	t.Run("unknown path returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// TestStartup is an end-to-end smoke test that combines config loading,
// metrics registration and HTTP serving — i.e. everything main() does
// before it starts talking to the switch / broker. If this passes, the
// app can boot.
func TestStartup(t *testing.T) {
	setZyxelEnv(t)
	// MQTT explicitly disabled; we don't have a broker in the test env.
	t.Setenv("MQTT_BROKER_HOST", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.MQTT.Host != "" {
		t.Fatalf("MQTT should be disabled, got host=%q", cfg.MQTT.Host)
	}

	RegisterMetrics()
	SetSystemInfo(nil) // simulate failed system-info fetch — must not panic

	ts := httptest.NewServer(httpHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}
}
