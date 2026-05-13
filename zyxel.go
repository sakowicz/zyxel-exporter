package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type ZyxelConfig struct {
	Host     string
	Username string
	Password string
}

type PortData struct {
	Port        int
	Consumption float64
}

type InterfaceData struct {
	Port          int
	LinkUp        bool
	LinkSpeedMbps int
	State         string
	UptimeSeconds int64
	TxKBps        float64
	RxKBps        float64
	TxUtilPercent float64
	RxUtilPercent float64
}

type SwitchData struct {
	TotalPower        float64
	ConsumingPower    float64
	RemainingPower    float64
	PoEUsagePercent   int
	JunctionTempC     int
	Ports             []PortData
	CPUUsagePercent   float64
	MemoryTotalBytes  int64
	MemoryUsedBytes   int64
	MemoryUsagePercent int
	Interfaces        []InterfaceData
	MacCount          int
}

type SystemInfo struct {
	Model           string
	SystemName      string
	MAC             string
	SerialNumber    string
	FirmwareVersion string
	HardwareVersion string
}

var (
	reTotalPower     = regexp.MustCompile(`Total Power:\s*([\d.]+)\(W\)`)
	reConsumingPower = regexp.MustCompile(`Consuming Power:\s*([\d.]+)\(W\)`)
	reRemainingPower = regexp.MustCompile(`Remaining Power:\s*([\d.]+)\(W\)`)
	rePoEUsage       = regexp.MustCompile(`PoE Usage:\s*(\d+)\(%\)`)
	reJunctionTemp   = regexp.MustCompile(`Averaged Junction Temperature:\s*(\d+)\s*\(c\)`)
	rePortRow        = regexp.MustCompile(`^\s+(\d+)\s+(Enable|Disable)\s+(On|Off)\s+\d+\s+\S+\s+\S+\s+([\d.]+)\s`)
	rePortHeader     = regexp.MustCompile(`Port\s+State\s+PD`)
	reMemory         = regexp.MustCompile(`common\s+(\d+)\(B\)\s+(\d+)\(B\)\s+(\d+)\(%\)`)
	reCPUUsage       = regexp.MustCompile(`(\d+)\s+(\d+)\s+(\d+\.\d+)`)
	reIfaceStatus    = regexp.MustCompile(`^\s+(\d+)(?:\s+\S+)?\s+(Down|[\d.]+[MG]/F)\s+(STOP|FORWARDING)\s+\S+\s+(\d+):(\d+):(\d+)`)
	reIfaceUtil      = regexp.MustCompile(`^\s+(\d+)\s+(?:Down|[\d.]+[MG]/F)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)`)
	reMacCount       = regexp.MustCompile(`No\s*:\s*(\d+)`)

	reModel     = regexp.MustCompile(`Product Model\s*:\s*(.+)`)
	reSysName   = regexp.MustCompile(`System Name\s*:\s*(.+)`)
	reEthAddr   = regexp.MustCompile(`Ethernet Address\s*:\s*(.+)`)
	reSerial    = regexp.MustCompile(`Serial Number\s*:\s*(.+)`)
	reFwVersion = regexp.MustCompile(`F/W Version\s*:\s*(.+)`)
	reHwVersion = regexp.MustCompile(`Hardware Version\s*:\s*(.+)`)
)

func Fetch(cfg ZyxelConfig) (*SwitchData, error) {
	// Each command runs in its own SSH session so they don't influence each
	// other's measurements (notably: show cpu-utilization, which reports a
	// rolling average over the last ~60 seconds and would otherwise capture
	// the CPU spike caused by sibling commands in the same session).
	// cpu-utilization runs first so it observes the switch in its idlest state.
	commands := []string{
		"show cpu-utilization",
		"show pwr",
		"show memory",
		"show interfaces status",
		"show interfaces utilization",
		"show mac-count",
	}

	var combined strings.Builder
	var anySuccess bool
	for _, cmd := range commands {
		out, err := runCommand(cfg, cmd)
		if err != nil {
			log.Printf("command %q failed: %v", cmd, err)
			continue
		}
		anySuccess = true
		combined.WriteString(out)
		combined.WriteString("\n")
	}
	if !anySuccess {
		return nil, fmt.Errorf("all switch commands failed")
	}

	return parse(combined.String()), nil
}

func parseLinkSpeed(s string) int {
	if s == "Down" {
		return 0
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	if len(s) == 0 {
		return 0
	}
	unit := s[len(s)-1]
	val, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0
	}
	switch unit {
	case 'G':
		return int(val * 1000)
	case 'M':
		return int(val)
	}
	return 0
}

func FetchSystemInfo(cfg ZyxelConfig) (*SystemInfo, error) {
	out, err := runCommand(cfg, "show system-information")
	if err != nil {
		return nil, err
	}
	return parseSystemInfo(out), nil
}

func runCommand(cfg ZyxelConfig, cmd string) (string, error) {
	clientCfg := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		// #nosec G106 — LAN-only device, no known-hosts store available
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", cfg.Host+":22", clientCfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	if err := session.RequestPty("xterm", 200, 10000, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		return "", fmt.Errorf("pty request: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		return "", fmt.Errorf("shell start: %w", err)
	}

	// Wait for banner/prompt before sending commands
	time.Sleep(500 * time.Millisecond)

	if _, err := fmt.Fprintf(stdin, "%s\nexit\n", cmd); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := io.ReadAll(stdout)
		ch <- result{b, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return "", fmt.Errorf("read output: %w", r.err)
		}
		out := strings.ReplaceAll(string(r.data), "\r", "")
		if os.Getenv("DEBUG_FETCH") == "1" {
			log.Printf("DEBUG_FETCH raw output for %q:\n%s\n--- end output ---", cmd, out)
		}
		return out, nil
	case <-ctx.Done():
		return "", fmt.Errorf("read timeout")
	}
}

func parse(output string) *SwitchData {
	data := &SwitchData{}
	inPortTable := false
	ifaceMap := make(map[int]*InterfaceData)
	getIface := func(port int) *InterfaceData {
		if iface, ok := ifaceMap[port]; ok {
			return iface
		}
		iface := &InterfaceData{Port: port}
		ifaceMap[port] = iface
		return iface
	}

	for _, line := range strings.Split(output, "\n") {
		if m := reTotalPower.FindStringSubmatch(line); m != nil {
			data.TotalPower, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := reConsumingPower.FindStringSubmatch(line); m != nil {
			data.ConsumingPower, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := reRemainingPower.FindStringSubmatch(line); m != nil {
			data.RemainingPower, _ = strconv.ParseFloat(m[1], 64)
		}
		if m := rePoEUsage.FindStringSubmatch(line); m != nil {
			data.PoEUsagePercent, _ = strconv.Atoi(m[1])
		}
		if m := reJunctionTemp.FindStringSubmatch(line); m != nil {
			data.JunctionTempC, _ = strconv.Atoi(m[1])
		}
		if m := reMemory.FindStringSubmatch(line); m != nil {
			data.MemoryTotalBytes, _ = strconv.ParseInt(m[1], 10, 64)
			data.MemoryUsedBytes, _ = strconv.ParseInt(m[2], 10, 64)
			data.MemoryUsagePercent, _ = strconv.Atoi(m[3])
		}
		if m := reMacCount.FindStringSubmatch(line); m != nil {
			data.MacCount, _ = strconv.Atoi(m[1])
		}
		if m := reIfaceStatus.FindStringSubmatch(line); m != nil {
			port, _ := strconv.Atoi(m[1])
			iface := getIface(port)
			link := m[2]
			iface.LinkUp = link != "Down"
			iface.LinkSpeedMbps = parseLinkSpeed(link)
			iface.State = m[3]
			h, _ := strconv.ParseInt(m[4], 10, 64)
			mi, _ := strconv.ParseInt(m[5], 10, 64)
			s, _ := strconv.ParseInt(m[6], 10, 64)
			iface.UptimeSeconds = h*3600 + mi*60 + s
		}
		if m := reIfaceUtil.FindStringSubmatch(line); m != nil {
			port, _ := strconv.Atoi(m[1])
			iface := getIface(port)
			iface.TxKBps, _ = strconv.ParseFloat(m[2], 64)
			iface.TxUtilPercent, _ = strconv.ParseFloat(m[3], 64)
			iface.RxKBps, _ = strconv.ParseFloat(m[4], 64)
			iface.RxUtilPercent, _ = strconv.ParseFloat(m[5], 64)
		}

		if rePortHeader.MatchString(line) {
			inPortTable = true
			continue
		}

		if inPortTable {
			if m := rePortRow.FindStringSubmatch(line); m != nil {
				port, _ := strconv.Atoi(m[1])
				consumption, _ := strconv.ParseFloat(m[4], 64)
				data.Ports = append(data.Ports, PortData{
					Port:        port,
					Consumption: consumption,
				})
			}
		}
	}

	ports := make([]int, 0, len(ifaceMap))
	for p := range ifaceMap {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	for _, p := range ports {
		data.Interfaces = append(data.Interfaces, *ifaceMap[p])
	}

	// CPU usage is the median of the per-second history from `show
	// cpu-utilization`. The header line ("CPU usage status: X%") reports just
	// the current second, which on our SSH polls is dominated by key
	// exchange / auth / shell setup and spikes to 50-100% — so we ignore the
	// header and use the ~60-sample per-second table instead.
	var cpuSamples []float64
	for _, m := range reCPUUsage.FindAllStringSubmatch(output, -1) {
		util, err := strconv.ParseFloat(m[3], 64)
		if err == nil {
			cpuSamples = append(cpuSamples, util)
		}
	}
	if n := len(cpuSamples); n > 0 {
		sort.Float64s(cpuSamples)
		if n%2 == 0 {
			data.CPUUsagePercent = (cpuSamples[n/2-1] + cpuSamples[n/2]) / 2
		} else {
			data.CPUUsagePercent = cpuSamples[n/2]
		}
	}

	return data
}

func parseSystemInfo(output string) *SystemInfo {
	info := &SystemInfo{}

	for _, line := range strings.Split(output, "\n") {
		if m := reModel.FindStringSubmatch(line); m != nil && info.Model == "" {
			info.Model = strings.TrimSpace(m[1])
		}
		if m := reSysName.FindStringSubmatch(line); m != nil && info.SystemName == "" {
			info.SystemName = strings.TrimSpace(m[1])
		}
		if m := reEthAddr.FindStringSubmatch(line); m != nil && info.MAC == "" {
			info.MAC = strings.ToLower(strings.TrimSpace(m[1]))
		}
		if m := reSerial.FindStringSubmatch(line); m != nil && info.SerialNumber == "" {
			info.SerialNumber = strings.TrimSpace(m[1])
		}
		if m := reFwVersion.FindStringSubmatch(line); m != nil && info.FirmwareVersion == "" {
			fw := strings.TrimSpace(m[1])
			if idx := strings.Index(fw, "|"); idx >= 0 {
				fw = strings.TrimSpace(fw[:idx])
			}
			info.FirmwareVersion = fw
		}
		if m := reHwVersion.FindStringSubmatch(line); m != nil && info.HardwareVersion == "" {
			info.HardwareVersion = strings.TrimSpace(m[1])
		}
	}

	return info
}
