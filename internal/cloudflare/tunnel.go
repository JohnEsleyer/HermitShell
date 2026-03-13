package cloudflare

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

func (m *TunnelManager) CheckBinary() error {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return fmt.Errorf("cloudflared CLI not found. Please install it: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started-guide/run-as-24-7-service/")
	}
	return nil
}

type TunnelManager struct {
	mu        sync.Mutex
	processes map[string]*exec.Cmd
	cancels   map[string]context.CancelFunc
	urls      map[string]string
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		processes: make(map[string]*exec.Cmd),
		cancels:   make(map[string]context.CancelFunc),
		urls:      make(map[string]string),
	}
}

var urlRe = regexp.MustCompile(`https?://[a-zA-Z0-9-]+\.trycloudflare\.com`)

// StartQuickTunnel starts a trycloudflare tunnel for a specific local port and returns the URL.
// It will periodically restart the tunnel if it exits, maintaining the process.
func (m *TunnelManager) StartQuickTunnel(id string, port int) (string, error) {
	m.mu.Lock()
	if _, exists := m.processes[id]; exists {
		url := m.urls[id]
		m.mu.Unlock()
		return url, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[id] = cancel
	m.mu.Unlock()

	urlChan := make(chan string, 1)
	go m.runTunnelLoop(ctx, id, port, urlChan)

	select {
	case url := <-urlChan:
		m.mu.Lock()
		m.urls[id] = url
		m.mu.Unlock()
		return url, nil
	case <-time.After(60 * time.Second):
		cancel()
		m.mu.Lock()
		delete(m.cancels, id)
		m.mu.Unlock()
		return "", fmt.Errorf("timeout waiting for tunnel URL")
	}
}

func (m *TunnelManager) runTunnelLoop(ctx context.Context, id string, port int, urlChan chan string) {
	firstRun := true

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Printf("Tunnel %s error creating stderr pipe: %v", id, err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			log.Printf("Tunnel %s failed to start: %v", id, err)
			time.Sleep(5 * time.Second)
			continue
		}

		m.mu.Lock()
		m.processes[id] = cmd
		m.mu.Unlock()

		urlFound := false
		urlMu := sync.Mutex{}

		waitCh := make(chan error, 1)
		go func() {
			reader := bufio.NewReader(stderr)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				line = strings.TrimSpace(line)
				if match := urlRe.FindString(line); match != "" {
					urlMu.Lock()
					if firstRun {
						select {
						case urlChan <- match:
							firstRun = false
							urlFound = true
						default:
						}
					} else {
						m.mu.Lock()
						m.urls[id] = match
						m.mu.Unlock()
					}
					urlMu.Unlock()
				}
			}
			waitCh <- cmd.Wait()
		}()

		select {
		case <-waitCh:
		case <-time.After(60 * time.Second):
			log.Printf("Tunnel %s: timeout waiting for process", id)
			cmd.Wait()
		}

		urlMu.Lock()
		m.mu.Lock()
		delete(m.processes, id)
		m.mu.Unlock()

		if !urlFound && firstRun {
			log.Printf("Tunnel %s exited without URL, restarting in 5s...", id)
		} else {
			log.Printf("Tunnel %s exited, restarting in 5s...", id)
		}
		urlMu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *TunnelManager) StopTunnel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, exists := m.cancels[id]; exists {
		cancel()
		delete(m.cancels, id)
	}
	if cmd, exists := m.processes[id]; exists {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(m.processes, id)
	}
	delete(m.urls, id)
}

func (m *TunnelManager) GetURL(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.urls[id]
}

func (m *TunnelManager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmd, ok := m.processes[id]
	return ok && cmd.Process != nil && cmd.ProcessState == nil
}

func (m *TunnelManager) CheckTunnelHealth(id string, timeout time.Duration) bool {
	url := m.GetURL(id)
	if url == "" {
		return false
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 500
}
