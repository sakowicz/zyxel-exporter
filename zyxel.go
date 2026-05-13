package main

import (
	"context"
	"fmt"
	"io"
	"regexp"
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

type SwitchData struct {
	TotalPower      float64
	ConsumingPower  float64
	RemainingPower  float64
	PoEUsagePercent int
	JunctionTempC   int
	Ports           []PortData
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

	reModel     = regexp.MustCompile(`Product Model\s*:\s*(.+)`)
	reSysName   = regexp.MustCompile(`System Name\s*:\s*(.+)`)
	reEthAddr   = regexp.MustCompile(`Ethernet Address\s*:\s*(.+)`)
	reSerial    = regexp.MustCompile(`Serial Number\s*:\s*(.+)`)
	reFwVersion = regexp.MustCompile(`F/W Version\s*:\s*(.+)`)
	reHwVersion = regexp.MustCompile(`Hardware Version\s*:\s*(.+)`)
)

func Fetch(cfg ZyxelConfig) (*SwitchData, error) {
	out, err := runCommand(cfg, "show pwr")
	if err != nil {
		return nil, err
	}
	return parse(out), nil
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

	if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
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
		return strings.ReplaceAll(string(r.data), "\r", ""), nil
	case <-ctx.Done():
		return "", fmt.Errorf("read timeout")
	}
}

func parse(output string) *SwitchData {
	data := &SwitchData{}
	inPortTable := false

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
