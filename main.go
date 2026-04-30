package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/robfig/cron/v3"
)

type Config struct {
	Zyxel        ZyxelConfig
	MQTT         MQTTConfig
	CronSchedule string
}

func loadConfig() Config {
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			log.Fatalf("required env var %s is not set", key)
		}
		return v
	}

	port := 1883
	if p := os.Getenv("MQTT_BROKER_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	schedule := os.Getenv("CRON_SCHEDULE")
	if schedule == "" {
		schedule = "* * * * *"
	}

	return Config{
		Zyxel: ZyxelConfig{
			Host:     required("ZYXEL_DEVICE_IP"),
			Username: required("ZYXEL_DEVICE_USERNAME"),
			Password: required("ZYXEL_DEVICE_PASSWORD"),
		},
		MQTT: MQTTConfig{
			Host:     required("MQTT_BROKER_HOST"),
			Port:     port,
			Username: os.Getenv("MQTT_BROKER_USERNAME"),
			Password: os.Getenv("MQTT_BROKER_PASSWORD"),
		},
		CronSchedule: schedule,
	}
}

func main() {
	cfg := loadConfig()

	mc, err := Connect(cfg.MQTT)
	if err != nil {
		log.Fatalf("mqtt: %v", err)
	}

	collect := func() {
		data, err := Fetch(cfg.Zyxel)
		if err != nil {
			log.Printf("fetch error: %v", err)
			return
		}
		log.Printf("consuming=%.1fW total=%.1fW ports=%d", data.ConsumingPower, data.TotalPower, len(data.Ports))
		if err := Publish(mc, data); err != nil {
			log.Printf("publish error: %v", err)
		}
	}

	go collect()

	c := cron.New()
	if _, err := c.AddFunc(cfg.CronSchedule, collect); err != nil {
		log.Fatalf("cron: %v", err)
	}
	c.Start()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		log.Println("shutting down")
		c.Stop()
		mc.c.Disconnect(250)
		os.Exit(0)
	}()

	log.Printf("started, schedule=%q", cfg.CronSchedule)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("http: %v", err)
	}
}
