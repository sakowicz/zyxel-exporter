package main

import (
	"testing"
)

// Raw outputs captured from a real Zyxel XMG1915-10EP running ZyNOS V4.80(ACGP.3).
// Kept verbatim so regressions in parsing are caught against the actual on-wire format.

const samplePwr = `  Port     State    PD     Pri.    PD_Class       CDP    Cons.(W)
  ----    --------  ------ ------  -----------    -----  ----------
     1    Enable    Off    3       IEEE802.3at    n      0.0
     2    Enable    On     3       IEEE802.3at    n      8.4
     3    Enable    Off    3       IEEE802.3at    n      0.0
     4    Enable    Off    3       IEEE802.3at    n      0.0
     5    Enable    Off    3       IEEE802.3at    n      0.0
     6    Enable    Off    3       IEEE802.3at    n      0.0
     7    Enable    Off    3       IEEE802.3at    n      0.0
     8    Enable    On     3       IEEE802.3at    n      9.1
   PoE Mode :              Classification
   Total Power:            130.0(W)
   Consuming Power:        17.5(W)
   Remaining Power:        112.5(W)
   PoE Usage:              13(%)
   Averaged Junction Temperature:  55 (c)
`

const sampleMemory = `    Name          Total          Used     Util
  ------    -----------    ----------    -----
  common    38993920(B)    6249936(B)    16(%)
`

const sampleCPU = `CPU usage status:   5.78%
 baseline 7203841 ticks
 sec   ticks   util sec   ticks   util sec   ticks   util sec   ticks   util
 --- ------- ------ --- ------- ------ --- ------- ------ --- ------- ------
   0  417085   5.78   1  374798   5.20   2  380126   5.27   3  409407   5.68
   4  319776   4.43   5  314307   4.36   6  315219   4.37   7  305231   4.23
   8  297614   4.13   9  297304   4.12  10  300969   4.17  11  296471   4.11
  12  296172   4.11  13  303323   4.21  14  303107   4.20  15  386741   5.36
  16  301326   4.18  17  298495   4.14  18  311549   4.32  19  314741   4.36
  20  320682   4.45  21  305229   4.23  22  299488   4.15  23  395697   5.49
  24  307809   4.27  25  309806   4.30  26  314427   4.36  27  949217  13.17
  28  317153   4.40  29  332825   4.62  30  325876   4.52  31  337026   4.67
  32  328736   4.56  33  329653   4.57  34  329323   4.57  35  302617   4.20
  36  319562   4.43  37  317595   4.40  38 3258693  45.23  39 7203841 100.00
  40 3522084  48.89  41  309927   4.30  42  308060   4.27  43  312502   4.33
  44  292430   4.05  45  375711   5.21  46  297783   4.13  47  293080   4.06
  48  296332   4.11  49  297813   4.13  50  292687   4.06  51  304123   4.22
  52  298593   4.14  53  297048   4.12  54  310431   4.30  55  327276   4.54
  56  315965   4.38  57  312444   4.33  58  307341   4.26  59  318545   4.42
  60  328903   4.56  61  348499   4.83  62  336708   4.67
`

const sampleInterfacesStatus = `  Port      Name          Link        State             Type          Up Time
  ---- ------------- -------------- ---------- -------------------- ----------
     1                         Down       STOP         100M/1G/2.5G    0:00:00
     2                       2.5G/F FORWARDING         100M/1G/2.5G  135:29:08
     3                       100M/F FORWARDING         100M/1G/2.5G  135:30:10
     4                         1G/F FORWARDING         100M/1G/2.5G  135:28:45
     5                       100M/F FORWARDING         100M/1G/2.5G  135:30:14
     6                       100M/F FORWARDING         100M/1G/2.5G  133:31:04
     7                         Down       STOP         100M/1G/2.5G    0:00:00
     8                       2.5G/F FORWARDING         100M/1G/2.5G  135:29:48
     9                         Down       STOP               1G/10G    0:00:00
    10                         Down       STOP               1G/10G    0:00:00
`

const sampleInterfacesUtil = `  Port      Link          Tx kB/s      Tx util(%)     Rx kB/s     Rx util(%)
  ---- ------------- ----------------- ---------- --------------- ----------
     1          Down               0.0        0.0             0.0        0.0
     2        2.5G/F            12.787        0.0         202.278        0.1
     3        100M/F           188.520        1.5           1.412        0.0
     4          1G/F              11.7        0.0           8.720        0.0
     5        100M/F             0.320        0.0             0.0        0.0
     6        100M/F             0.320        0.0             0.0        0.0
     7          Down               0.0        0.0             0.0        0.0
     8        2.5G/F             3.151        0.0           2.859        0.0
     9          Down               0.0        0.0             0.0        0.0
    10          Down               0.0        0.0             0.0        0.0
`

const sampleMacCount = `No : 21
`

const sampleSystemInfo = `Product Model        : XMG1915-10EP
System Name        : branch
System Mode        : Standalone
System Contact        :
System Location        :
System up Time        :   134:41:32 (1ce6dfa9 ticks)
Ethernet Address    : 70:49:a2:56:bc:30
Bootbase Version    : V1.00 | 02/13/2023
ZyNOS F/W Version    : V4.80(ACGP.3) | 11/26/2024
Hardware Version    : V1.16
Config Boot Image     : 1
Current Boot Image     : 1
Current Configuration     : 1
RomRasSize        : 5867624
Serial Number        : S252L23001041
Register MAC Address    : 70:49:a2:56:bc:30
`

func TestParsePower(t *testing.T) {
	d := parse(samplePwr)

	if d.TotalPower != 130.0 {
		t.Errorf("TotalPower = %v, want 130.0", d.TotalPower)
	}
	if d.ConsumingPower != 17.5 {
		t.Errorf("ConsumingPower = %v, want 17.5", d.ConsumingPower)
	}
	if d.RemainingPower != 112.5 {
		t.Errorf("RemainingPower = %v, want 112.5", d.RemainingPower)
	}
	if d.PoEUsagePercent != 13 {
		t.Errorf("PoEUsagePercent = %v, want 13", d.PoEUsagePercent)
	}
	if d.JunctionTempC != 55 {
		t.Errorf("JunctionTempC = %v, want 55", d.JunctionTempC)
	}
	if len(d.Ports) != 8 {
		t.Fatalf("Ports = %d entries, want 8", len(d.Ports))
	}

	consumption := map[int]float64{}
	for _, p := range d.Ports {
		consumption[p.Port] = p.Consumption
	}
	if consumption[2] != 8.4 {
		t.Errorf("port 2 consumption = %v, want 8.4", consumption[2])
	}
	if consumption[8] != 9.1 {
		t.Errorf("port 8 consumption = %v, want 9.1", consumption[8])
	}
	if consumption[1] != 0.0 {
		t.Errorf("port 1 consumption = %v, want 0.0", consumption[1])
	}
}

func TestParseMemory(t *testing.T) {
	d := parse(sampleMemory)

	if d.MemoryTotalBytes != 38993920 {
		t.Errorf("MemoryTotalBytes = %v, want 38993920", d.MemoryTotalBytes)
	}
	if d.MemoryUsedBytes != 6249936 {
		t.Errorf("MemoryUsedBytes = %v, want 6249936", d.MemoryUsedBytes)
	}
	if d.MemoryUsagePercent != 16 {
		t.Errorf("MemoryUsagePercent = %v, want 16", d.MemoryUsagePercent)
	}
}

func TestParseCPU(t *testing.T) {
	d := parse(sampleCPU)

	// 63 per-second samples; the outliers (sec 27=13.17, sec 38=45.23, sec
	// 39=100.00, sec 40=48.89) are from prior polling spikes and must NOT
	// dominate the value. The median of the sorted samples for this input
	// is 4.36 — verified against the spec-correct sort of these 63 values.
	if d.CPUUsagePercent < 4.0 || d.CPUUsagePercent > 5.0 {
		t.Errorf("CPUUsagePercent = %v, want ~4.36 (median); a value above 5 means setup-spike outliers are leaking in", d.CPUUsagePercent)
	}
}

func TestParseMacCount(t *testing.T) {
	d := parse(sampleMacCount)
	if d.MacCount != 21 {
		t.Errorf("MacCount = %v, want 21", d.MacCount)
	}
}

func TestParseInterfaces(t *testing.T) {
	// Status + utilization combined, as they arrive together in Fetch.
	d := parse(sampleInterfacesStatus + sampleInterfacesUtil)

	if len(d.Interfaces) != 10 {
		t.Fatalf("Interfaces = %d, want 10", len(d.Interfaces))
	}

	byPort := map[int]InterfaceData{}
	for _, iface := range d.Interfaces {
		byPort[iface.Port] = iface
	}

	// Port 2: 2.5G/F, FORWARDING, 135:29:08 uptime, ~12.8/202.3 kB/s
	p2 := byPort[2]
	if !p2.LinkUp {
		t.Errorf("port 2 LinkUp = false, want true")
	}
	if p2.LinkSpeedMbps != 2500 {
		t.Errorf("port 2 LinkSpeedMbps = %d, want 2500", p2.LinkSpeedMbps)
	}
	if p2.State != "FORWARDING" {
		t.Errorf("port 2 State = %q, want FORWARDING", p2.State)
	}
	// 135*3600 + 29*60 + 8 = 487748
	if p2.UptimeSeconds != 487748 {
		t.Errorf("port 2 UptimeSeconds = %d, want 487748", p2.UptimeSeconds)
	}
	if p2.TxKBps != 12.787 {
		t.Errorf("port 2 TxKBps = %v, want 12.787", p2.TxKBps)
	}
	if p2.RxKBps != 202.278 {
		t.Errorf("port 2 RxKBps = %v, want 202.278", p2.RxKBps)
	}
	if p2.RxUtilPercent != 0.1 {
		t.Errorf("port 2 RxUtilPercent = %v, want 0.1", p2.RxUtilPercent)
	}

	// Port 3: 100M/F, 1.5% tx util — only port with non-zero util
	p3 := byPort[3]
	if p3.LinkSpeedMbps != 100 {
		t.Errorf("port 3 LinkSpeedMbps = %d, want 100", p3.LinkSpeedMbps)
	}
	if p3.TxUtilPercent != 1.5 {
		t.Errorf("port 3 TxUtilPercent = %v, want 1.5", p3.TxUtilPercent)
	}

	// Port 4: 1G/F
	p4 := byPort[4]
	if p4.LinkSpeedMbps != 1000 {
		t.Errorf("port 4 LinkSpeedMbps = %d, want 1000", p4.LinkSpeedMbps)
	}

	// Port 1, 7, 9, 10: Down → speed 0, !LinkUp, STOP, uptime 0
	for _, port := range []int{1, 7, 9, 10} {
		p := byPort[port]
		if p.LinkUp {
			t.Errorf("port %d LinkUp = true, want false", port)
		}
		if p.LinkSpeedMbps != 0 {
			t.Errorf("port %d LinkSpeedMbps = %d, want 0", port, p.LinkSpeedMbps)
		}
		if p.State != "STOP" {
			t.Errorf("port %d State = %q, want STOP", port, p.State)
		}
		if p.UptimeSeconds != 0 {
			t.Errorf("port %d UptimeSeconds = %d, want 0", port, p.UptimeSeconds)
		}
	}
}

func TestParseSystemInfo(t *testing.T) {
	info := parseSystemInfo(sampleSystemInfo)

	if info.Model != "XMG1915-10EP" {
		t.Errorf("Model = %q, want XMG1915-10EP", info.Model)
	}
	if info.SystemName != "branch" {
		t.Errorf("SystemName = %q, want branch", info.SystemName)
	}
	if info.MAC != "70:49:a2:56:bc:30" {
		t.Errorf("MAC = %q, want 70:49:a2:56:bc:30", info.MAC)
	}
	if info.SerialNumber != "S252L23001041" {
		t.Errorf("SerialNumber = %q, want S252L23001041", info.SerialNumber)
	}
	// FW version line is "ZyNOS F/W Version : V4.80(ACGP.3) | 11/26/2024".
	// Parser must strip everything after the pipe.
	if info.FirmwareVersion != "V4.80(ACGP.3)" {
		t.Errorf("FirmwareVersion = %q, want V4.80(ACGP.3)", info.FirmwareVersion)
	}
	if info.HardwareVersion != "V1.16" {
		t.Errorf("HardwareVersion = %q, want V1.16", info.HardwareVersion)
	}
}

func TestParseLinkSpeed(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"Down", 0},
		{"100M/F", 100},
		{"1G/F", 1000},
		{"2.5G/F", 2500},
		{"10G/F", 10000},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseLinkSpeed(c.in); got != c.want {
			t.Errorf("parseLinkSpeed(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// --- Variations: different port counts, models, edge cases. ---

// 24-port GS1900-style switch — same CLI shape, just more rows.
const sampleInterfacesStatus24 = `  Port      Name          Link        State             Type          Up Time
  ---- ------------- -------------- ---------- -------------------- ----------
     1                         1G/F FORWARDING               100M/1G   12:00:00
     2                         1G/F FORWARDING               100M/1G   12:00:00
     3                         Down       STOP               100M/1G    0:00:00
     4                         1G/F FORWARDING               100M/1G   11:59:55
     5                         Down       STOP               100M/1G    0:00:00
     6                         1G/F FORWARDING               100M/1G    8:30:12
     7                         Down       STOP               100M/1G    0:00:00
     8                         Down       STOP               100M/1G    0:00:00
     9                         Down       STOP               100M/1G    0:00:00
    10                         1G/F FORWARDING               100M/1G   10:15:33
    11                         Down       STOP               100M/1G    0:00:00
    12                         Down       STOP               100M/1G    0:00:00
    13                         Down       STOP               100M/1G    0:00:00
    14                         Down       STOP               100M/1G    0:00:00
    15                         Down       STOP               100M/1G    0:00:00
    16                         Down       STOP               100M/1G    0:00:00
    17                         Down       STOP               100M/1G    0:00:00
    18                         Down       STOP               100M/1G    0:00:00
    19                         Down       STOP               100M/1G    0:00:00
    20                         Down       STOP               100M/1G    0:00:00
    21                         Down       STOP               100M/1G    0:00:00
    22                         Down       STOP               100M/1G    0:00:00
    23                         Down       STOP               100M/1G    0:00:00
    24                         1G/F FORWARDING               100M/1G   12:00:00
    25                        10G/F FORWARDING               1G/10G     6:00:00
    26                         Down       STOP               1G/10G    0:00:00
`

// Switch where ports have user-assigned names (e.g. "uplink", "ap-01").
const sampleInterfacesStatusNamed = `  Port      Name          Link        State             Type          Up Time
  ---- ------------- -------------- ---------- -------------------- ----------
     1 uplink-core           2.5G/F FORWARDING         100M/1G/2.5G  135:29:08
     2 ap-livingroom         100M/F FORWARDING         100M/1G/2.5G  135:30:10
     3                         Down       STOP         100M/1G/2.5G    0:00:00
`

// Non-PoE switch / unsupported command — Zyxel returns a brief error.
const samplePwrUnsupported = `% Invalid input detected at '^' marker.
`

// PoE present but nothing connected — all 8 ports report 0.0W and Off.
const samplePwrAllOff = `  Port     State    PD     Pri.    PD_Class       CDP    Cons.(W)
  ----    --------  ------ ------  -----------    -----  ----------
     1    Enable    Off    3       IEEE802.3at    n      0.0
     2    Enable    Off    3       IEEE802.3at    n      0.0
     3    Enable    Off    3       IEEE802.3at    n      0.0
     4    Enable    Off    3       IEEE802.3at    n      0.0
     5    Enable    Off    3       IEEE802.3at    n      0.0
     6    Enable    Off    3       IEEE802.3at    n      0.0
     7    Enable    Off    3       IEEE802.3at    n      0.0
     8    Enable    Off    3       IEEE802.3at    n      0.0
   Total Power:            130.0(W)
   Consuming Power:        0.0(W)
   Remaining Power:        130.0(W)
   PoE Usage:              0(%)
   Averaged Junction Temperature:  42 (c)
`

// PoE with a port administratively disabled (Disable state).
const samplePwrPartialDisabled = `  Port     State    PD     Pri.    PD_Class       CDP    Cons.(W)
  ----    --------  ------ ------  -----------    -----  ----------
     1    Disable   Off    3       IEEE802.3at    n      0.0
     2    Enable    On     3       IEEE802.3at    n      8.4
   Total Power:            130.0(W)
   Consuming Power:        8.4(W)
   Remaining Power:        121.6(W)
   PoE Usage:              6(%)
   Averaged Junction Temperature:  50 (c)
`

func TestParseInterfaces_24Port(t *testing.T) {
	d := parse(sampleInterfacesStatus24)

	if len(d.Interfaces) != 26 {
		t.Fatalf("Interfaces = %d, want 26 (24 + 2 uplinks)", len(d.Interfaces))
	}

	byPort := map[int]InterfaceData{}
	for _, iface := range d.Interfaces {
		byPort[iface.Port] = iface
	}

	if !byPort[1].LinkUp || byPort[1].LinkSpeedMbps != 1000 {
		t.Errorf("port 1: up=%v speed=%d, want up=true speed=1000", byPort[1].LinkUp, byPort[1].LinkSpeedMbps)
	}
	if byPort[25].LinkSpeedMbps != 10000 {
		t.Errorf("port 25 (10G uplink): speed=%d, want 10000", byPort[25].LinkSpeedMbps)
	}
	// 12:00:00 = 12 * 3600 = 43200
	if byPort[1].UptimeSeconds != 43200 {
		t.Errorf("port 1 UptimeSeconds = %d, want 43200", byPort[1].UptimeSeconds)
	}
}

func TestParseInterfaces_NamedPorts(t *testing.T) {
	d := parse(sampleInterfacesStatusNamed)

	if len(d.Interfaces) != 3 {
		t.Fatalf("Interfaces = %d, want 3", len(d.Interfaces))
	}

	byPort := map[int]InterfaceData{}
	for _, iface := range d.Interfaces {
		byPort[iface.Port] = iface
	}

	// The optional name token between port number and link state must not
	// throw off the parser.
	if !byPort[1].LinkUp || byPort[1].LinkSpeedMbps != 2500 {
		t.Errorf("port 1 (named uplink-core): up=%v speed=%d, want up=true speed=2500", byPort[1].LinkUp, byPort[1].LinkSpeedMbps)
	}
	if !byPort[2].LinkUp || byPort[2].LinkSpeedMbps != 100 {
		t.Errorf("port 2 (named ap-livingroom): up=%v speed=%d, want up=true speed=100", byPort[2].LinkUp, byPort[2].LinkSpeedMbps)
	}
	if byPort[3].LinkUp {
		t.Errorf("port 3 (unnamed, down): up=%v, want false", byPort[3].LinkUp)
	}
}

func TestParsePower_Unsupported(t *testing.T) {
	// Non-PoE switch: `show pwr` returns an error. Parser must not crash
	// and must produce zero PoE values + no ports.
	d := parse(samplePwrUnsupported)

	if d.TotalPower != 0 || d.ConsumingPower != 0 || d.RemainingPower != 0 {
		t.Errorf("PoE values should be zero on non-PoE switch, got total=%v consuming=%v remaining=%v",
			d.TotalPower, d.ConsumingPower, d.RemainingPower)
	}
	if len(d.Ports) != 0 {
		t.Errorf("Ports = %d, want 0 on non-PoE switch", len(d.Ports))
	}
}

func TestParsePower_AllOff(t *testing.T) {
	d := parse(samplePwrAllOff)

	if d.ConsumingPower != 0.0 {
		t.Errorf("ConsumingPower = %v, want 0.0", d.ConsumingPower)
	}
	if d.PoEUsagePercent != 0 {
		t.Errorf("PoEUsagePercent = %v, want 0", d.PoEUsagePercent)
	}
	if len(d.Ports) != 8 {
		t.Fatalf("Ports = %d, want 8 (all enabled, all off)", len(d.Ports))
	}
	for _, p := range d.Ports {
		if p.Consumption != 0.0 {
			t.Errorf("port %d consumption = %v, want 0.0", p.Port, p.Consumption)
		}
	}
}

func TestParsePower_PartialDisabled(t *testing.T) {
	d := parse(samplePwrPartialDisabled)

	if len(d.Ports) != 2 {
		t.Fatalf("Ports = %d, want 2", len(d.Ports))
	}

	consumption := map[int]float64{}
	for _, p := range d.Ports {
		consumption[p.Port] = p.Consumption
	}
	if consumption[1] != 0.0 {
		t.Errorf("port 1 (disabled): consumption=%v, want 0.0", consumption[1])
	}
	if consumption[2] != 8.4 {
		t.Errorf("port 2 (enabled, on): consumption=%v, want 8.4", consumption[2])
	}
}

func TestParse_EmptyInput(t *testing.T) {
	// Worst-case: every SSH command failed and we got nothing. Parser
	// should return a zero-valued struct, not panic.
	d := parse("")
	if d == nil {
		t.Fatal("parse(\"\") returned nil")
	}
	if d.TotalPower != 0 || len(d.Ports) != 0 || len(d.Interfaces) != 0 || d.MacCount != 0 {
		t.Errorf("expected zero-valued SwitchData, got %+v", d)
	}
}

func TestParse_GarbageInput(t *testing.T) {
	// Random text shouldn't extract anything.
	d := parse("hello world\nthis is not a switch output\nfoo bar 123\n")
	if d.TotalPower != 0 || len(d.Ports) != 0 || len(d.Interfaces) != 0 {
		t.Errorf("expected nothing extracted from garbage, got %+v", d)
	}
}

func TestParseLinkSpeed_ExoticSpeeds(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"5G/F", 5000},   // 5GBASE-T
		{"25G/F", 25000}, // 25G SFP28
		{"40G/F", 40000}, // 40G QSFP+
		{"10M/F", 10},    // legacy 10Base-T
		{"100G/F", 100000},
		{"bogus", 0}, // garbage falls back to 0
	}
	for _, c := range cases {
		if got := parseLinkSpeed(c.in); got != c.want {
			t.Errorf("parseLinkSpeed(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// Combined output — verifies the parser handles all six commands concatenated
// (the actual Fetch order) without one section's data bleeding into another's.
func TestParseCombined(t *testing.T) {
	combined := sampleCPU + "\n" + samplePwr + "\n" + sampleMemory + "\n" +
		sampleInterfacesStatus + "\n" + sampleInterfacesUtil + "\n" + sampleMacCount

	d := parse(combined)

	if d.TotalPower != 130.0 {
		t.Errorf("TotalPower = %v, want 130.0", d.TotalPower)
	}
	if d.MemoryTotalBytes != 38993920 {
		t.Errorf("MemoryTotalBytes = %v, want 38993920", d.MemoryTotalBytes)
	}
	if d.MacCount != 21 {
		t.Errorf("MacCount = %v, want 21", d.MacCount)
	}
	if len(d.Ports) != 8 {
		t.Errorf("len(Ports) = %d, want 8", len(d.Ports))
	}
	if len(d.Interfaces) != 10 {
		t.Errorf("len(Interfaces) = %d, want 10", len(d.Interfaces))
	}
	if d.CPUUsagePercent < 4.0 || d.CPUUsagePercent > 5.0 {
		t.Errorf("CPUUsagePercent = %v, want ~4.36 (median)", d.CPUUsagePercent)
	}
}
