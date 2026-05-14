package main

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerOnce sync.Once

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

	metricPortLinkUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_link_up",
		Help: "Port link state (1 = up, 0 = down).",
	}, []string{"port"})
	metricPortSpeed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_speed_mbps",
		Help: "Negotiated link speed in megabits per second (0 when down).",
	}, []string{"port"})
	metricPortUptime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_uptime_seconds",
		Help: "Seconds since the port last came up.",
	}, []string{"port"})
	metricPortTxBps = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_tx_bytes_per_second",
		Help: "Transmit rate in bytes/sec as sampled by the switch.",
	}, []string{"port"})
	metricPortRxBps = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_rx_bytes_per_second",
		Help: "Receive rate in bytes/sec as sampled by the switch.",
	}, []string{"port"})
	metricPortTxUtil = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_tx_utilization_percent",
		Help: "Transmit link utilization 0-100.",
	}, []string{"port"})
	metricPortRxUtil = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "zyxel_port_rx_utilization_percent",
		Help: "Receive link utilization 0-100.",
	}, []string{"port"})
	metricMacCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zyxel_mac_count",
		Help: "Total MAC addresses learned by the switch.",
	})
)

func RegisterMetrics() {
	registerOnce.Do(func() {
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
			metricPortLinkUp,
			metricPortSpeed,
			metricPortUptime,
			metricPortTxBps,
			metricPortRxBps,
			metricPortTxUtil,
			metricPortRxUtil,
			metricMacCount,
		)
	})
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

	metricMacCount.Set(float64(data.MacCount))

	metricPortLinkUp.Reset()
	metricPortSpeed.Reset()
	metricPortUptime.Reset()
	metricPortTxBps.Reset()
	metricPortRxBps.Reset()
	metricPortTxUtil.Reset()
	metricPortRxUtil.Reset()
	for _, iface := range data.Interfaces {
		port := strconv.Itoa(iface.Port)
		up := 0.0
		if iface.LinkUp {
			up = 1
		}
		metricPortLinkUp.WithLabelValues(port).Set(up)
		metricPortSpeed.WithLabelValues(port).Set(float64(iface.LinkSpeedMbps))
		metricPortUptime.WithLabelValues(port).Set(float64(iface.UptimeSeconds))
		metricPortTxBps.WithLabelValues(port).Set(iface.TxKBps * 1024)
		metricPortRxBps.WithLabelValues(port).Set(iface.RxKBps * 1024)
		metricPortTxUtil.WithLabelValues(port).Set(iface.TxUtilPercent)
		metricPortRxUtil.WithLabelValues(port).Set(iface.RxUtilPercent)
	}
}
