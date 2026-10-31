// Package monitor collects server resource statistics via SSH exec.
package monitor

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

// DiskPartition holds usage info for a single mounted filesystem.
type DiskPartition struct {
	Mount string `json:"mount"` // mount point, e.g. "/", "/home"
	Used  uint64 `json:"used"`  // bytes used
	Total uint64 `json:"total"` // bytes total
}

// MonitorStats holds a single snapshot of server resource usage.
type MonitorStats struct {
	CPU      float64         `json:"cpu"`             // 0–100 percentage
	MemUsed  uint64          `json:"mem_used"`         // bytes
	MemTotal uint64          `json:"mem_total"`        // bytes
	NetRx    float64         `json:"net_rx"`           // bytes/s
	NetTx    float64         `json:"net_tx"`           // bytes/s
	CpuTemp  float64         `json:"cpu_temp"`         // °C; -1 = not available
	Disks    []DiskPartition `json:"disks"`            // real block-device partitions
	Error    string          `json:"error,omitempty"`
}

type cpuSample struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

func (s cpuSample) total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal
}

type netSample struct{ rx, tx uint64 }

// Collector maintains inter-sample state for delta calculations.
// It is NOT safe for concurrent use; callers must serialize Collect calls
// or use separate Collector instances per goroutine.
type Collector struct {
	client     *gossh.Client
	prevCPU    cpuSample
	prevNet    netSample
	prevTime   time.Time
	hasPrevCPU bool // true once a valid CPU baseline sample exists
	hasPrevNet bool // true once a valid net baseline sample exists
}

// NewCollector creates a Collector for the given SSH client.
func NewCollector(client *gossh.Client) *Collector {
	return &Collector{client: client}
}

// collectCmd auto-detects Linux (/proc) vs macOS (sysctl + vm_stat + netstat).
// First output line is always "OS:linux" or "OS:macos".
// Sections are separated by sentinel strings.
const collectCmd = `if [ -r /proc/stat ]; then
  printf 'OS:linux\n'
  cat /proc/stat 2>/dev/null
  printf '\nGSSH_MEM\n'
  cat /proc/meminfo 2>/dev/null
  printf '\nGSSH_NET\n'
  cat /proc/net/dev 2>/dev/null
  printf '\nGSSH_TEMP\n'
  { cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || cat /sys/class/hwmon/hwmon0/temp1_input 2>/dev/null || printf '%s' '-1'; }
  printf '\nGSSH_DISK\n'
  df -Pk 2>/dev/null
else
  printf 'OS:macos\n'
  sysctl -n hw.logicalcpu 2>/dev/null
  printf '\nGSSH_CPU\n'
  ps -A -o %cpu 2>/dev/null
  printf '\nGSSH_MEM\n'
  sysctl -n hw.memsize 2>/dev/null
  printf '\nGSSH_VM\n'
  vm_stat 2>/dev/null
  printf '\nGSSH_NET\n'
  netstat -ibn 2>/dev/null
  printf '\nGSSH_TEMP\n'
  printf '%s' '-1'
  printf '\nGSSH_DISK\n'
  df -Pk 2>/dev/null
fi; true`

// Collect opens one SSH channel, reads system data, and returns a stats snapshot.
// The first call always returns 0 for rate-based metrics (CPU/net); subsequent
// calls compute deltas against the previous sample.
func (c *Collector) Collect() MonitorStats {
	now := time.Now()

	raw, err := c.exec(collectCmd)
	if err != nil {
		return MonitorStats{Error: "SSH exec 失败: " + err.Error()}
	}

	nl := strings.IndexByte(raw, '\n')
	if nl < 0 {
		return MonitorStats{Error: "服务器返回数据异常"}
	}
	osTag := strings.TrimSpace(raw[:nl])
	body := raw[nl+1:]

	switch osTag {
	case "OS:linux":
		return c.processLinux(body, now)
	case "OS:macos":
		return c.processMacos(body, now)
	default:
		return MonitorStats{Error: "不支持的服务器系统（需要 Linux 或 macOS）"}
	}
}

// ── Linux ─────────────────────────────────────────────────────────────────────

func (c *Collector) processLinux(body string, now time.Time) MonitorStats {
	statRaw, rest := cutSection(body, "\nGSSH_MEM\n")
	memRaw, rest2 := cutSection(rest, "\nGSSH_NET\n")
	netRaw, rest3 := cutSection(rest2, "\nGSSH_TEMP\n")
	tempRaw, diskRaw := cutSection(rest3, "\nGSSH_DISK\n")

	memUsed, memTotal, _ := parseMem(memRaw)
	curNet, _ := parseLinuxNet(netRaw)
	curCPU, cpuErr := parseCPU(statRaw)

	stats := MonitorStats{
		MemUsed:  memUsed,
		MemTotal: memTotal,
		CpuTemp:  parseTemp(tempRaw),
		Disks:    parseDisk(diskRaw),
	}

	if c.hasPrevCPU || c.hasPrevNet {
		dt := now.Sub(c.prevTime).Seconds()
		if dt > 0 {
			if c.hasPrevCPU && cpuErr == nil {
				// Use int64 to detect counter resets (avoid uint64 underflow).
				dTot := int64(curCPU.total()) - int64(c.prevCPU.total())
				dIdle := int64(curCPU.idle) - int64(c.prevCPU.idle)
				if dTot > 0 {
					stats.CPU = 100.0 * (1.0 - float64(dIdle)/float64(dTot))
					if stats.CPU < 0 {
						stats.CPU = 0
					}
				}
			}
			if c.hasPrevNet {
				if d := int64(curNet.rx) - int64(c.prevNet.rx); d >= 0 {
					stats.NetRx = float64(d) / dt
				}
				if d := int64(curNet.tx) - int64(c.prevNet.tx); d >= 0 {
					stats.NetTx = float64(d) / dt
				}
			}
		}
	}

	if memTotal == 0 {
		stats.Error = "服务器不支持 /proc（容器限制）"
	}
	if cpuErr == nil {
		c.prevCPU = curCPU
		c.hasPrevCPU = true
	} else {
		// CPU parse failed: invalidate CPU baseline so the next successful parse
		// starts fresh and avoids a stale-delta CPU spike.
		c.hasPrevCPU = false
	}
	// Net baseline is always updated independently of CPU parse success,
	// so net throughput remains visible even when CPU stats are unavailable.
	c.prevNet = curNet
	c.prevTime = now
	c.hasPrevNet = true
	return stats
}

// ── macOS ─────────────────────────────────────────────────────────────────────

func (c *Collector) processMacos(body string, now time.Time) MonitorStats {
	// body layout:
	//   hw.logicalcpu \nGSSH_CPU\n ps-%cpu \nGSSH_MEM\n hw.memsize \nGSSH_VM\n vm_stat \nGSSH_NET\n netstat-ibn \nGSSH_TEMP\n -1 \nGSSH_DISK\n df
	ncpuRaw, rest1 := cutSection(body, "\nGSSH_CPU\n")
	psRaw, rest2 := cutSection(rest1, "\nGSSH_MEM\n")
	memTotalRaw, rest3 := cutSection(rest2, "\nGSSH_VM\n")
	vmStatRaw, rest4 := cutSection(rest3, "\nGSSH_NET\n")
	netRaw, rest5 := cutSection(rest4, "\nGSSH_TEMP\n")
	_, diskRaw := cutSection(rest5, "\nGSSH_DISK\n")

	ncpu := 1
	if n, err := strconv.Atoi(strings.TrimSpace(ncpuRaw)); err == nil && n > 0 {
		ncpu = n
	}

	cpu := parseMacosPS(psRaw, ncpu)
	memTotal, _ := strconv.ParseUint(strings.TrimSpace(memTotalRaw), 10, 64)
	memUsed := parseMacosVmStat(vmStatRaw)
	curNet := parseMacosNet(netRaw)

	stats := MonitorStats{
		CPU:      cpu,
		MemUsed:  memUsed,
		MemTotal: memTotal,
		CpuTemp:  -1, // macOS temperature not accessible without root
		Disks:    parseDisk(diskRaw),
	}

	if c.hasPrevNet {
		dt := now.Sub(c.prevTime).Seconds()
		if dt > 0 {
			if d := int64(curNet.rx) - int64(c.prevNet.rx); d >= 0 {
				stats.NetRx = float64(d) / dt
			}
			if d := int64(curNet.tx) - int64(c.prevNet.tx); d >= 0 {
				stats.NetTx = float64(d) / dt
			}
		}
	}

	c.prevNet = curNet
	c.prevTime = now
	c.hasPrevNet = true
	return stats
}

// parseMacosPS sums %cpu from `ps -A -o %cpu` and divides by ncpu.
// ps reports per-process rolling-average CPU; summing approximates total usage.
func parseMacosPS(raw string, ncpu int) float64 {
	var sum float64
	first := true
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first { // skip header line "%CPU"
			first = false
			continue
		}
		if v, err := strconv.ParseFloat(line, 64); err == nil {
			sum += v
		}
	}
	cpu := sum / float64(ncpu)
	if cpu > 100 {
		cpu = 100
	}
	if cpu < 0 {
		cpu = 0
	}
	return cpu
}

// parseMacosVmStat parses `vm_stat` output and returns used memory in bytes.
// Used ≈ (active + wired + compressed) pages × page_size, matching Activity Monitor.
func parseMacosVmStat(raw string) uint64 {
	var pageSize uint64 = 4096
	var active, wired, compressed uint64

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		// Header: "Mach Virtual Memory Statistics: (page size of 4096 bytes)"
		if strings.Contains(line, "page size of") {
			for _, f := range strings.Fields(line) {
				if n, err := strconv.ParseUint(f, 10, 64); err == nil && n > 0 {
					pageSize = n
					break
				}
			}
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		valStr := strings.TrimRight(strings.TrimSpace(line[colon+1:]), ".")
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "Pages active":
			active = val
		case "Pages wired down":
			wired = val
		case "Pages occupied by compressor":
			compressed = val
		}
	}
	return (active + wired + compressed) * pageSize
}

// parseMacosNet parses `netstat -ibn` and sums rx/tx bytes for non-loopback
// interfaces. It handles both MAC-addressed and address-less Link# rows.
func parseMacosNet(raw string) netSample {
	var rx, tx uint64
	lineNo := 0
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		lineNo++
		if lineNo == 1 { // skip column header
			continue
		}
		line := scanner.Text()
		if !strings.Contains(line, "<Link#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		// Skip loopback
		if fields[0] == "lo0" || fields[0] == "lo" {
			continue
		}
		// Column layout (fixed after Network/Address):
		//   with MAC:    Name Mtu Network MAC    Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll [Drop]
		//   without MAC: Name Mtu Network        Ipkts Ierrs Ibytes Opkts Oerrs Obytes Coll [Drop]
		// Detect MAC presence: field[3] contains ":" (colon from MAC notation)
		var iIdx, oIdx int
		if len(fields) > 3 && strings.Contains(fields[3], ":") {
			iIdx, oIdx = 6, 9 // MAC present
		} else {
			iIdx, oIdx = 5, 8 // no MAC
		}
		if oIdx < len(fields) {
			r, _ := strconv.ParseUint(fields[iIdx], 10, 64)
			t, _ := strconv.ParseUint(fields[oIdx], 10, 64)
			rx += r
			tx += t
		}
	}
	return netSample{rx: rx, tx: tx}
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// exec opens one SSH session, runs cmd, and returns combined stdout+stderr.
// On command failure the combined output is included in the error so callers
// and log consumers can see the shell's error message without extra SSH sessions.
func (c *Collector) exec(cmd string) (string, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	if err != nil {
		if len(out) > 0 {
			return "", fmt.Errorf("%w; output: %s", err, strings.TrimSpace(string(out)))
		}
		return "", err
	}
	return string(out), nil
}

// cutSection splits s at the first occurrence of sep and returns (before, after).
// If sep is absent, returns (s, "").
func cutSection(s, sep string) (string, string) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):]
	}
	return s, ""
}

// ── Shared parsers ────────────────────────────────────────────────────────────

// parseTemp converts a millidegree string (Linux thermal sysfs) to Celsius.
// Returns -1 if the value is absent or invalid.
func parseTemp(raw string) float64 {
	s := strings.TrimSpace(raw)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return -1
	}
	return float64(n) / 1000.0
}

// parseDisk parses `df -Pk` output and returns one DiskPartition per real
// block device (filesystem path starts with "/dev/"). Virtual filesystems
// (tmpfs, devtmpfs, sysfs, cgroup, …) are silently skipped.
// df -Pk columns: Filesystem  1K-blocks  Used  Available  Capacity%  Mounted-on
func parseDisk(raw string) []DiskPartition {
	var result []DiskPartition
	scanner := bufio.NewScanner(strings.NewReader(raw))
	lineNo := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNo++
		if lineNo == 1 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// Only real block devices
		if !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		t, e1 := strconv.ParseUint(fields[1], 10, 64)
		u, e2 := strconv.ParseUint(fields[2], 10, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		// Mount point may be the last field (handles wrapped long lines)
		mount := strings.Join(fields[5:], " ")
		result = append(result, DiskPartition{
			Mount: mount,
			Used:  u * 1024,
			Total: t * 1024,
		})
	}
	return result
}

// ── Linux parsers ─────────────────────────────────────────────────────────────

func parseCPU(raw string) (cpuSample, error) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return cpuSample{}, fmt.Errorf("unexpected cpu line")
		}
		var s cpuSample
		ptrs := []*uint64{&s.user, &s.nice, &s.system, &s.idle, &s.iowait, &s.irq, &s.softirq, &s.steal}
		for i, p := range ptrs {
			n, e := strconv.ParseUint(fields[i+1], 10, 64)
			if e != nil {
				return cpuSample{}, e
			}
			*p = n
		}
		return s, nil
	}
	return cpuSample{}, fmt.Errorf("cpu line not found")
}

func parseMem(raw string) (used, total uint64, err error) {
	var memTotal, memAvail uint64
	var memFree, buffers, cached, sReclaim uint64
	hasAvail := false
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, e := strconv.ParseUint(fields[1], 10, 64)
		if e != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal = val * 1024
		case "MemAvailable:":
			memAvail = val * 1024
			hasAvail = true
		case "MemFree:":
			memFree = val * 1024
		case "Buffers:":
			buffers = val * 1024
		case "Cached:":
			cached = val * 1024
		case "SReclaimable:":
			sReclaim = val * 1024
		}
	}
	if memTotal == 0 {
		return 0, 0, fmt.Errorf("MemTotal not found")
	}
	// 老内核（<3.14，如 CentOS 6）没有 MemAvailable 字段，退化为
	// free 的经典口径：可用 = free + buffers + cached + 可回收 slab。
	if !hasAvail {
		memAvail = memFree + buffers + cached + sReclaim
	}
	if memAvail > memTotal {
		memAvail = memTotal
	}
	return memTotal - memAvail, memTotal, nil
}

func parseLinuxNet(raw string) (netSample, error) {
	var rx, tx uint64
	lineNo := 0
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 { // skip header lines
			continue
		}
		line := strings.TrimSpace(scanner.Text())
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		if strings.TrimSpace(line[:idx]) == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(fields[0], 10, 64)
		t, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += r
		tx += t
	}
	return netSample{rx: rx, tx: tx}, nil
}
