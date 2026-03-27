// Package docker provides Docker container management for Hermit agents.
//
// Documentation:
// - container-management.md: Container lifecycle, workspace structure
// - security-measures.md: Container isolation
// - concurrency.md: RWMutex and background goroutines for metrics
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	prevStats        map[string]types.CPUStats
}

type ContainerStats struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemUsageMB float64 `json:"memUsageMB"`
	MemLimitMB float64 `json:"memLimitMB"`
	Created    string  `json:"created"`
	Status     string  `json:"status"`
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
		cli:       cli,
		timeout:   2 * time.Minute,
		prevStats: make(map[string]types.CPUStats),
	}

	// Pre-populate metrics cache synchronously for fast initial dashboard load
	metrics, err := c.collectSystemMetrics()
	if err == nil {
		c.latestSystem = metrics
	}

	c.StartMetricsAggregator()
	// Ensure hermit-network exists
	c.cli.NetworkCreate(context.Background(), "hermit-network", types.NetworkCreate{})
	return c, nil
}

// StartMetricsAggregator runs a background goroutine that collects system metrics every second.
// Docs: See docs/concurrency.md for background goroutine and mutex patterns.
// How it works: Uses RWMutex to protect cached metrics, updates every 1 second.
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
	cpuPct, err := cpu.Percent(time.Second, false)
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
		go func(containerID, name, dockerStatus string) {
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

			cleanName := strings.TrimPrefix(name, "/")

			c.mu.Lock()
			prev, hasPrev := c.prevStats[cleanName]
			c.prevStats[cleanName] = v.CPUStats
			c.mu.Unlock()

			var cpuPercent = 0.0
			if hasPrev {
				cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(prev.CPUUsage.TotalUsage)
				systemDelta := float64(v.CPUStats.SystemUsage) - float64(prev.SystemUsage)
				onlineCPUs := float64(v.CPUStats.OnlineCPUs)
				if onlineCPUs == 0.0 {
					onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
				}
				if onlineCPUs == 0.0 {
					onlineCPUs = 1.0
				}

				if systemDelta > 0.0 && cpuDelta > 0.0 {
					cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
				} else if cpuDelta > 0 {
					// Fallback to time-based delta if system delta is missing
					timeDelta := float64(v.Read.Sub(v.PreRead).Nanoseconds())
					if timeDelta > 0 {
						cpuPercent = (cpuDelta / timeDelta) * onlineCPUs * 100.0
					}
				}
			}

			// Provide a tiny baseline if it's running but we have low activity to avoid 0.0% confusion
			isActuallyRunning := dockerStatus == "running" || dockerStatus == "active"
			if cpuPercent < 0.1 && isActuallyRunning {
				cpuPercent = 0.1
			}

			// Memory calculation
			usage := v.MemoryStats.Usage
			if cache, ok := v.MemoryStats.Stats["inactive_file"]; ok {
				usage -= cache
			} else if cache, ok := v.MemoryStats.Stats["cache"]; ok {
				usage -= cache
			}
			if usage < 1024*1024 { // minimum 1MB if running
				usage = 1024 * 1024
			}

			memUsageMB := float64(usage) / (1024 * 1024)
			memLimitMB := float64(v.MemoryStats.Limit) / (1024 * 1024)

			// Get created time
			inspect, _ := c.cli.ContainerInspect(ctx, containerID)
			created := inspect.Created

			mu.Lock()
			stats = append(stats, ContainerStats{
				Name:       cleanName,
				CPUPercent: cpuPercent,
				MemUsageMB: memUsageMB,
				MemLimitMB: memLimitMB,
				Created:    created,
				Status:     dockerStatus,
			})
			mu.Unlock()
		}(cont.ID, cont.Names[0], cont.State)
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

// Run creates and starts a Docker container for an agent.
// Docs: See docs/container-management.md for container lifecycle.
// Docs: See docs/security-measures.md for container isolation.
// Traefik: Adds labels for automatic routing via Docker provider.
func (c *Client) Run(name, image string, detach bool) error {
	ctx := context.Background()

	// 1. Check if container already exists
	inspect, err := c.cli.ContainerInspect(ctx, name)
	if err == nil {
		// Already exists - start if stopped
		if inspect.State.Running {
			return nil
		}
		return c.cli.ContainerStart(ctx, name, types.ContainerStartOptions{})
	}

	// 2. Container doesn't exist, create it.
	// Check if image exists locally first (don't pull if local)
	images, err := c.cli.ImageList(ctx, types.ImageListOptions{
		All: true,
	})
	if err == nil {
		localImage := false
		for _, img := range images {
			for _, tag := range img.RepoTags {
				if tag == image || tag == image+":latest" {
					localImage = true
					break
				}
			}
			if localImage {
				break
			}
		}
		if !localImage {
			// Try to pull only if not found locally
			_, err = c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
			if err != nil {
				log.Printf("Warning: failed to pull image %s: %v", image, err)
			}
		}
	}

	// Traefik labels for routing: /agent/{container-name}/
	labels := map[string]string{
		"traefik.enable":                                      "true",
		"traefik.http.routers.agent.rule":                     "PathPrefix(`/agent/" + name + "/`)",
		"traefik.http.routers.agent.entrypoints":              "web",
		"traefik.http.services.agent.loadbalancer.server.url": "http://" + name + ":80",
		"traefik.docker.network":                              "hermit-network",
	}

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image:  image,
		Cmd:    []string{"sleep", "infinity"},
		Labels: labels,
	}, &container.HostConfig{
		NetworkMode: "hermit-network",
	}, nil, nil, name)
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

func (c *Client) IsRunning(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inspect, err := c.cli.ContainerInspect(ctx, name)
	if err != nil {
		return false
	}
	return inspect.State.Running
}

type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
}

func (c *Client) ReadFile(containerName, filePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := []string{"sh", "-c", fmt.Sprintf("cat '%s'", filePath)}

	execCfg := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
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

	out, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (c *Client) ListContainerFiles(containerName, dir string) ([]FileInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := []string{"sh", "-c", fmt.Sprintf("ls -la '%s' 2>/dev/null | tail -n +2", dir)}
	if dir == "" || dir == "/" {
		cmd = []string{"sh", "-c", "ls -la /app/workspace/ 2>/dev/null | tail -n +2"}
	}

	execCfg := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	idResp, err := c.cli.ContainerExecCreate(ctx, containerName, execCfg)
	if err != nil {
		return nil, err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	out, _ := io.ReadAll(resp.Reader)
	lines := strings.Split(string(out), "\n")

	var files []FileInfo
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		mode := fields[0]
		name := strings.Join(fields[8:], " ")
		if name == "." || name == ".." {
			continue
		}

		isDir := mode[0] == 'd'
		var size int64
		fmt.Sscanf(fields[4], "%d", &size)

		files = append(files, FileInfo{
			Name:    name,
			Size:    size,
			Mode:    mode,
			IsDir:   isDir,
			ModTime: fields[5] + " " + fields[6],
		})
	}

	return files, nil
}

func (c *Client) DeleteContainerPath(containerName, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := []string{"sh", "-c", fmt.Sprintf("rm -rf '%s'", path)}

	execCfg := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	idResp, err := c.cli.ContainerExecCreate(ctx, containerName, execCfg)
	if err != nil {
		return err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer resp.Close()

	out, _ := io.ReadAll(resp.Reader)
	if strings.Contains(string(out), "cannot remove") || strings.Contains(string(out), "No such file") {
		return fmt.Errorf("path not found: %s", path)
	}

	return nil
}
