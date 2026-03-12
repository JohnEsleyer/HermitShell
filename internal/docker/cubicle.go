package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	timeout time.Duration
}

type ContainerStats struct {
	Name       string
	CPUPercent float64
	MemUsageMB float64
	MemLimitMB float64
}

type HostMetrics struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemoryUsed  uint64  `json:"memoryUsed"`
	MemoryTotal uint64  `json:"memoryTotal"`
	Timestamp   int64   `json:"timestamp"`
}

func NewClient() *Client {
	return &Client{
		timeout: 2 * time.Minute,
	}
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

func (c *Client) HostStats() (HostMetrics, error) {
	cmd := exec.Command("sh", "-c", "cat /proc/stat | head -n1; cat /proc/meminfo | head -n 2")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return HostMetrics{}, err
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		return HostMetrics{}, fmt.Errorf("unexpected proc output")
	}
	cpuFields := strings.Fields(lines[0])
	if len(cpuFields) < 5 {
		return HostMetrics{}, fmt.Errorf("invalid cpu output")
	}
	var total, idle uint64
	for i, v := range cpuFields[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 {
			idle = n
		}
	}
	cpuPct := 0.0
	if total > 0 {
		cpuPct = float64(total-idle) * 100 / float64(total)
	}

	memTotal := parseMemInfoLine(lines[1])
	memFree := parseMemInfoLine(lines[2])
	used := uint64(0)
	if memTotal > memFree {
		used = memTotal - memFree
	}

	return HostMetrics{CPUPercent: cpuPct, MemoryUsed: used * 1024, MemoryTotal: memTotal * 1024, Timestamp: time.Now().Unix()}, nil
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
