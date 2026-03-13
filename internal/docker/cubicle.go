package docker

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type Client struct {
	cli              *client.Client
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

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	c := &Client{
		cli:     cli,
		timeout: 2 * time.Minute,
	}
	c.StartMetricsAggregator()
	return c, nil
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
	var hostErr, contErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		host, hostErr = c.collectHostMetrics()
	}()
	go func() {
		defer wg.Done()
		containers, contErr = c.collectContainerMetrics()
	}()
	wg.Wait()

	if hostErr != nil {
		return SystemMetrics{}, hostErr
	}
	if contErr != nil {
		return SystemMetrics{}, contErr
	}
	if containers == nil {
		containers = []ContainerStats{}
	}

	return SystemMetrics{Host: host, Containers: containers}, nil
}

func (c *Client) collectHostMetrics() (HostMetrics, error) {
	cpuPct, err := cpu.Percent(0, false)
	if err != nil {
		return HostMetrics{}, err
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return HostMetrics{}, err
	}

	d, err := disk.Usage("/")
	if err != nil {
		return HostMetrics{}, err
	}

	cpuVal := 0.0
	if len(cpuPct) > 0 {
		cpuVal = cpuPct[0]
	}

	return HostMetrics{
		CPUPercent:    cpuVal,
		MemoryUsed:    vm.Used,
		MemoryTotal:   vm.Total,
		MemoryFree:    vm.Available,
		DiskUsed:      d.Used,
		DiskTotal:     d.Total,
		DiskFree:      d.Free,
		MemoryPercent: vm.UsedPercent,
		DiskPercent:   d.UsedPercent,
		Timestamp:     time.Now().Unix(),
	}, nil
}

func (c *Client) collectContainerMetrics() ([]ContainerStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	var stats []ContainerStats
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, cont := range containers {
		wg.Add(1)
		go func(containerID, name string) {
			defer wg.Done()

			statsResp, err := c.cli.ContainerStats(ctx, containerID, false)
			if err != nil {
				return
			}
			defer statsResp.Body.Close()

			var v *types.StatsJSON
			if err := json.NewDecoder(statsResp.Body).Decode(&v); err != nil {
				return
			}

			var cpuPercent = 0.0
			cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
			systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
			if systemDelta > 0.0 && cpuDelta > 0.0 {
				cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
			}

			memUsageMB := float64(v.MemoryStats.Usage) / (1024 * 1024)
			memLimitMB := float64(v.MemoryStats.Limit) / (1024 * 1024)

			cleanName := strings.TrimPrefix(name, "/")
			mu.Lock()
			stats = append(stats, ContainerStats{
				Name:       cleanName,
				CPUPercent: cpuPercent,
				MemUsageMB: memUsageMB,
				MemLimitMB: memLimitMB,
			})
			mu.Unlock()
		}(cont.ID, cont.Names[0])
	}

	wg.Wait()
	return stats, nil
}

func (c *Client) Exec(containerName string, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	execCfg := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", command},
		WorkingDir:   "/app/workspace/work",
	}

	idResp, err := c.cli.ContainerExecCreate(ctx, containerName, execCfg)
	if err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	out, _ := io.ReadAll(resp.Reader)
	return string(out), nil
}

func (c *Client) Run(name, image string, detach bool) error {
	ctx := context.Background()
	_, err := c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"sleep", "infinity"},
	}, nil, nil, nil, name)
	if err != nil {
		return err
	}

	return c.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}

func (c *Client) Stop(name string) error {
	ctx := context.Background()
	return c.cli.ContainerStop(ctx, name, container.StopOptions{})
}

func (c *Client) Remove(name string) error {
	ctx := context.Background()
	return c.cli.ContainerRemove(ctx, name, types.ContainerRemoveOptions{})
}

func (c *Client) List() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, cont := range containers {
		names = append(names, cont.Names...)
	}
	return names, nil
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
