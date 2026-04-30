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

var (
	reTotalPower     = regexp.MustCompile(`Total Power:\s*([\d.]+)\(W\)`)
	reConsumingPower = regexp.MustCompile(`Consuming Power:\s*([\d.]+)\(W\)`)
	reRemainingPower = regexp.MustCompile(`Remaining Power:\s*([\d.]+)\(W\)`)
	rePoEUsage       = regexp.MustCompile(`PoE Usage:\s*(\d+)\(%\)`)
	reJunctionTemp   = regexp.MustCompile(`Averaged Junction Temperature:\s*(\d+)\s*\(c\)`)
	rePortRow        = regexp.MustCompile(`^\s+(\d+)\s+(Enable|Disable)\s+(On|Off)\s+\d+\s+\S+\s+\S+\s+([\d.]+)\s`)
	rePortHeader     = regexp.MustCompile(`Port\s+State\s+PD`)
)

func Fetch(cfg ZyxelConfig) (*SwitchData, error) {
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
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	if err := session.RequestPty("xterm", 80, 40, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		return nil, fmt.Errorf("pty request: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("shell start: %w", err)
	}

	// Wait for banner/prompt before sending commands
	time.Sleep(500 * time.Millisecond)

	if _, err := fmt.Fprintf(stdin, "show pwr\nexit\n"); err != nil {
		return nil, fmt.Errorf("write command: %w", err)
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

	var raw []byte
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("read output: %w", r.err)
		}
		raw = r.data
	case <-ctx.Done():
		return nil, fmt.Errorf("read timeout")
	}

	return parse(strings.ReplaceAll(string(raw), "\r", "")), nil
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
