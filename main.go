package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
)

type Config struct {
	Zyxel        ZyxelConfig
	MQTT         MQTTConfig
	CronSchedule string
	ListenAddr   string
}

func loadConfig() (Config, error) {
	var missing []string
	must := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := Config{
		Zyxel: ZyxelConfig{
			Host:     must("ZYXEL_DEVICE_IP"),
			Username: must("ZYXEL_DEVICE_USERNAME"),
			Password: must("ZYXEL_DEVICE_PASSWORD"),
		},
		MQTT: MQTTConfig{
			Host:     os.Getenv("MQTT_BROKER_HOST"),
			Port:     1883,
			Username: os.Getenv("MQTT_BROKER_USERNAME"),
			Password: os.Getenv("MQTT_BROKER_PASSWORD"),
		},
		CronSchedule: "* * * * *",
		ListenAddr:   ":8080",
	}

	if len(missing) > 0 {
		return cfg, fmt.Errorf("required env vars not set: %v", missing)
	}

	if p := os.Getenv("MQTT_BROKER_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			cfg.MQTT.Port = n
		}
	}
	if s := os.Getenv("CRON_SCHEDULE"); s != "" {
		cfg.CronSchedule = s
	}
	if l := os.Getenv("HTTP_LISTEN"); l != "" {
		cfg.ListenAddr = l
	}

	return cfg, nil
}

func httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	RegisterMetrics()

	info, err := FetchSystemInfo(cfg.Zyxel)
	if err != nil {
		log.Printf("system-information fetch failed (continuing with generic device): %v", err)
	} else {
		log.Printf("switch: model=%q name=%q mac=%s serial=%s fw=%s hw=%s",
			info.Model, info.SystemName, info.MAC, info.SerialNumber, info.FirmwareVersion, info.HardwareVersion)
	}
	SetSystemInfo(info)

	var mc *mqttClient
	if cfg.MQTT.Host != "" {
		mc, err = Connect(cfg.MQTT, info, cfg.Zyxel.Host)
		if err != nil {
			log.Fatalf("mqtt: %v", err)
		}
	} else {
		log.Println("MQTT disabled (MQTT_BROKER_HOST not set)")
	}

	collect := func() {
		data, err := Fetch(cfg.Zyxel)
		if err != nil {
			log.Printf("fetch error: %v", err)
			return
		}
		log.Printf("consuming=%.1fW total=%.1fW ports=%d cpu=%.2f%% mem=%d%%",
			data.ConsumingPower, data.TotalPower, len(data.Ports), data.CPUUsagePercent, data.MemoryUsagePercent)
		UpdateMetrics(data)
		if mc != nil {
			if err := Publish(mc, data); err != nil {
				log.Printf("publish error: %v", err)
			}
		}
	}

	go collect()

	c := cron.New()
	if _, err := c.AddFunc(cfg.CronSchedule, collect); err != nil {
		log.Fatalf("cron: %v", err)
	}
	c.Start()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		log.Println("shutting down")
		c.Stop()
		if mc != nil {
			mc.c.Disconnect(250)
		}
		os.Exit(0)
	}()

	log.Printf("started, schedule=%q listen=%s", cfg.CronSchedule, cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, httpHandler()); err != nil {
		log.Fatalf("http: %v", err)
	}
}
