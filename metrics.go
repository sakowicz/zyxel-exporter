package main

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_info",
		Help: "Static information about the Zyxel switch (always 1).",
	}, []string{"model", "name", "serial", "mac", "fw_version", "hw_version"})

	metricTotalPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_poe_total_watts",
		Help: "Total PoE power budget in watts.",
	})
	metricConsumingPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_poe_consuming_watts",
		Help: "PoE power currently being consumed in watts.",
	})
	metricRemainingPower = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_poe_remaining_watts",
		Help: "PoE power budget remaining in watts.",
	})
	metricPoEUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_poe_usage_percent",
		Help: "PoE power usage percentage (0-100).",
	})
	metricJunctionTemp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_junction_temperature_celsius",
		Help: "Switch junction temperature in degrees Celsius.",
	})
	metricCPUUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_cpu_usage_percent",
		Help: "Switch CPU usage percentage (0-100).",
	})
	metricMemoryTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_memory_total_bytes",
		Help: "Total switch memory in bytes.",
	})
	metricMemoryUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_memory_used_bytes",
		Help: "Used switch memory in bytes.",
	})
	metricMemoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_memory_usage_percent",
		Help: "Switch memory usage percentage (0-100).",
	})
	metricPortPower = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_poe_port_watts",
		Help: "Per-port PoE power consumption in watts.",
	}, []string{"port"})
)

func RegisterMetrics() {
	prometheus.MustRegister(
		metricInfo,
		metricTotalPower,
		metricConsumingPower,
		metricRemainingPower,
		metricPoEUsage,
		metricJunctionTemp,
		metricCPUUsage,
		metricMemoryTotal,
		metricMemoryUsed,
		metricMemoryUsage,
		metricPortPower,
	)
}

func SetSystemInfo(info *SystemInfo) {
	if info == nil {
		return
	}
	metricInfo.WithLabelValues(
		info.Model,
		info.SystemName,
		info.SerialNumber,
		info.MAC,
		info.FirmwareVersion,
		info.HardwareVersion,
	).Set(1)
}

func UpdateMetrics(data *SwitchData) {
	metricTotalPower.Set(data.TotalPower)
	metricConsumingPower.Set(data.ConsumingPower)
	metricRemainingPower.Set(data.RemainingPower)
	metricPoEUsage.Set(float64(data.PoEUsagePercent))
	metricJunctionTemp.Set(float64(data.JunctionTempC))
	metricCPUUsage.Set(data.CPUUsagePercent)
	metricMemoryTotal.Set(float64(data.MemoryTotalBytes))
	metricMemoryUsed.Set(float64(data.MemoryUsedBytes))
	metricMemoryUsage.Set(float64(data.MemoryUsagePercent))

	metricPortPower.Reset()
	for _, p := range data.Ports {
		metricPortPower.WithLabelValues(strconv.Itoa(p.Port)).Set(p.Consumption)
	}
}
