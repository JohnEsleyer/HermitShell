package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	timeout          time.Duration
	mu               sync.RWMutex
	latestSystem     SystemMetrics
	aggregatorActive bool
}

type ContainerStats struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemUsageMB float64 `json:"memUsageMB"`
	MemLimitMB float64 `json:"memLimitMB"`
}

type HostMetrics struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsed    uint64  `json:"memoryUsed"`
	MemoryTotal   uint64  `json:"memoryTotal"`
	MemoryFree    uint64  `json:"memoryFree"`
	DiskUsed      uint64  `json:"diskUsed"`
	DiskTotal     uint64  `json:"diskTotal"`
	DiskFree      uint64  `json:"diskFree"`
	MemoryPercent float64 `json:"memoryPercent"`
	DiskPercent   float64 `json:"diskPercent"`
	Timestamp     int64   `json:"timestamp"`
}

type SystemMetrics struct {
	Host       HostMetrics      `json:"host"`
	Containers []ContainerStats `json:"containers"`
}

func NewClient() *Client {
	c := &Client{timeout: 2 * time.Minute}
	c.StartMetricsAggregator()
	return c
}

func (c *Client) StartMetricsAggregator() {
	c.mu.Lock()
	if c.aggregatorActive {
		c.mu.Unlock()
		return
	}
	c.aggregatorActive = true
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			metrics, err := c.collectSystemMetrics()
			if err == nil {
				c.mu.Lock()
				c.latestSystem = metrics
				c.mu.Unlock()
			}
			<-ticker.C
		}
	}()
}

func (c *Client) LatestSystemMetrics() (SystemMetrics, error) {
	c.mu.RLock()
	cached := c.latestSystem
	c.mu.RUnlock()
	if cached.Host.Timestamp > 0 {
		return cached, nil
	}

	metrics, err := c.collectSystemMetrics()
	if err != nil {
		return SystemMetrics{}, err
	}
	c.mu.Lock()
	c.latestSystem = metrics
	c.mu.Unlock()
	return metrics, nil
}

func (c *Client) collectSystemMetrics() (SystemMetrics, error) {
	var wg sync.WaitGroup
	var host HostMetrics
	var containers []ContainerStats
	var hostErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		host, hostErr = c.collectHostMetrics()
	}()
	go func() {
		defer wg.Done()
		containers, _ = c.collectContainerMetrics()
	}()
	wg.Wait()

	if hostErr != nil {
		return SystemMetrics{}, hostErr
	}
	if containers == nil {
		containers = []ContainerStats{}
	}

	return SystemMetrics{Host: host, Containers: containers}, nil
}

func (c *Client) collectHostMetrics() (HostMetrics, error) {
	cmd := exec.Command("sh", "-c", "cat /proc/stat | head -n1; cat /proc/meminfo | head -n 3; df -B1 / | tail -n1")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return HostMetrics{}, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 5 {
		return HostMetrics{}, fmt.Errorf("unexpected host metrics output")
	}

	cpuPct, err := parseCPUPercent(lines[0])
	if err != nil {
		return HostMetrics{}, err
	}

	memTotal := parseMemInfoLine(lines[1]) * 1024
	memFree := parseMemInfoLine(lines[2]) * 1024
	memAvailable := parseMemInfoLine(lines[3]) * 1024
	if memAvailable == 0 {
		memAvailable = memFree
	}
	memUsed := uint64(0)
	if memTotal > memAvailable {
		memUsed = memTotal - memAvailable
	}
	memPercent := 0.0
	if memTotal > 0 {
		memPercent = float64(memUsed) * 100.0 / float64(memTotal)
	}

	diskTotal, diskUsed, diskFree := parseDFLine(lines[4])
	diskPercent := 0.0
	if diskTotal > 0 {
		diskPercent = float64(diskUsed) * 100.0 / float64(diskTotal)
	}

	return HostMetrics{
		CPUPercent:    cpuPct,
		MemoryUsed:    memUsed,
		MemoryTotal:   memTotal,
		MemoryFree:    memAvailable,
		DiskUsed:      diskUsed,
		DiskTotal:     diskTotal,
		DiskFree:      diskFree,
		MemoryPercent: memPercent,
		DiskPercent:   diskPercent,
		Timestamp:     time.Now().Unix(),
	}, nil
}

func (c *Client) collectContainerMetrics() ([]ContainerStats, error) {
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	stats := make([]ContainerStats, 0)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		cpu := strings.TrimSuffix(strings.TrimSpace(parts[1]), "%")
		cpuF, _ := strconv.ParseFloat(cpu, 64)
		used, limit := parseMemUsage(parts[2])
		stats = append(stats, ContainerStats{Name: strings.TrimSpace(parts[0]), CPUPercent: cpuF, MemUsageMB: used, MemLimitMB: limit})
	}
	return stats, nil
}

func (c *Client) Exec(containerName string, command string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", "-w", "/app/workspace/work", containerName, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	if err != nil {
		return output, fmt.Errorf("command failed: %v", err)
	}

	return output, nil
}

func (c *Client) Run(name, image string, detach bool) error {
	args := []string{"run"}
	if detach {
		args = append(args, "-d")
	}
	args = append(args, []string{"--name", name, image, "sleep", "infinity"}...)

	cmd := exec.Command("docker", args...)
	return cmd.Run()
}

func (c *Client) Stop(name string) error {
	cmd := exec.Command("docker", "stop", name)
	return cmd.Run()
}

func (c *Client) Remove(name string) error {
	cmd := exec.Command("docker", "rm", "-f", name)
	return cmd.Run()
}

func (c *Client) List() ([]string, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var containers []string
	for _, name := range strings.Split(out.String(), "\n") {
		name = strings.TrimSpace(name)
		if name != "" {
			containers = append(containers, name)
		}
	}
	return containers, nil
}

func (c *Client) Stats() ([]ContainerStats, error) {
	metrics, err := c.LatestSystemMetrics()
	if err != nil {
		return nil, err
	}
	return metrics.Containers, nil
}

func (c *Client) HostStats() (HostMetrics, error) {
	metrics, err := c.LatestSystemMetrics()
	if err != nil {
		return HostMetrics{}, err
	}
	return metrics.Host, nil
}

func parseCPUPercent(line string) (float64, error) {
	cpuFields := strings.Fields(line)
	if len(cpuFields) < 5 {
		return 0, fmt.Errorf("invalid cpu output")
	}
	var total, idle uint64
	for i, v := range cpuFields[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 {
			idle = n
		}
	}
	if total == 0 {
		return 0, nil
	}
	return float64(total-idle) * 100 / float64(total), nil
}

func parseDFLine(line string) (uint64, uint64, uint64) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return 0, 0, 0
	}
	total, _ := strconv.ParseUint(fields[1], 10, 64)
	used, _ := strconv.ParseUint(fields[2], 10, 64)
	free, _ := strconv.ParseUint(fields[3], 10, 64)
	return total, used, free
}

func parseMemUsage(v string) (float64, float64) {
	parts := strings.Split(v, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return toMB(parts[0]), toMB(parts[1])
}

func toMB(v string) float64 {
	t := strings.TrimSpace(v)
	t = strings.TrimSuffix(t, "iB")
	if t == "" {
		return 0
	}
	num := t[:len(t)-1]
	unit := strings.ToUpper(t[len(t)-1:])
	f, _ := strconv.ParseFloat(num, 64)
	switch unit {
	case "G":
		return f * 1024
	case "M":
		return f
	case "K":
		return f / 1024
	default:
		return f / (1024 * 1024)
	}
}

func parseMemInfoLine(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}
