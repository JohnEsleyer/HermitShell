package cloudflare

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestNewTunnelManager(t *testing.T) {
	mgr := NewTunnelManager()
	if mgr == nil {
		t.Fatal("NewTunnelManager returned nil")
	}
	if mgr.processes == nil {
		t.Error("processes map not initialized")
	}
	if mgr.urls == nil {
		t.Error("urls map not initialized")
	}
	if mgr.cancels == nil {
		t.Error("cancels map not initialized")
	}
}

func TestCheckBinary(t *testing.T) {
	mgr := NewTunnelManager()
	err := mgr.CheckBinary()
	if err != nil {
		t.Logf("cloudflared not found (expected on systems without cloudflared): %v", err)
	}
}

func TestURLRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://abc123.trycloudflare.com", true},
		{"https://my-tunnel.trycloudflare.com", true},
		{"http://abc123.trycloudflare.com", true},
		{"https://abc-123.trycloudflare.com", true},
		{"https://example.com", false},
		{"not a url", false},
		{"", false},
	}

	for _, tt := range tests {
		match := urlRe.FindString(tt.input)
		hasMatch := match != ""
		if hasMatch != tt.expected {
			t.Errorf("URL regex test failed for input %q: expected %v, got %v", tt.input, tt.expected, hasMatch)
		}
	}
}

func TestGetURL(t *testing.T) {
	mgr := NewTunnelManager()

	url := mgr.GetURL("nonexistent")
	if url != "" {
		t.Errorf("expected empty string for nonexistent tunnel, got %q", url)
	}

	mgr.urls["test-tunnel"] = "https://test.trycloudflare.com"
	url = mgr.GetURL("test-tunnel")
	if url != "https://test.trycloudflare.com" {
		t.Errorf("expected URL, got %q", url)
	}
}

func TestStopTunnel(t *testing.T) {
	mgr := NewTunnelManager()

	_, cancel := context.WithCancel(context.Background())
	mgr.cancels["test"] = cancel
	mgr.urls["test"] = "https://test.trycloudflare.com"

	mgr.StopTunnel("test")

	if _, exists := mgr.cancels["test"]; exists {
		t.Error("cancel should be removed")
	}
	if _, exists := mgr.urls["test"]; exists {
		t.Error("url should be removed")
	}
	if _, exists := mgr.processes["test"]; exists {
		t.Error("process should be removed")
	}
}

func TestCheckTunnelHealth(t *testing.T) {
	mgr := NewTunnelManager()

	healthy := mgr.CheckTunnelHealth("nonexistent", time.Second)
	if healthy {
		t.Error("expected false when tunnel doesn't exist")
	}
}

func TestURLRegexComplex(t *testing.T) {
	complexURLs := []string{
		"https://abc123def456.trycloudflare.com",
		"https://a-b-c-d.trycloudflare.com",
		"https://1234567890.trycloudflare.com",
	}

	for _, url := range complexURLs {
		match := urlRe.FindString(url)
		if match == "" {
			t.Errorf("expected to match URL %q", url)
		}
		if match != url {
			t.Errorf("expected full URL %q, got %q", url, match)
		}
	}
}

func TestURLRegexCompilation(t *testing.T) {
	if urlRe == nil {
		t.Fatal("urlRe should not be nil")
	}

	_, err := regexp.Compile(urlRe.String())
	if err != nil {
		t.Errorf("urlRe should be valid regexp: %v", err)
	}
}

func TestGetURLConcurrent(t *testing.T) {
	mgr := NewTunnelManager()
	mgr.urls["test"] = "https://test.trycloudflare.com"

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			url := mgr.GetURL("test")
			if url != "https://test.trycloudflare.com" {
				t.Errorf("unexpected URL: %s", url)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStartQuickTunnelIdempotent(t *testing.T) {
	mgr := NewTunnelManager()

	mgr.mu.Lock()
	mgr.processes["test"] = nil
	mgr.urls["test"] = "https://existing.trycloudflare.com"
	mgr.mu.Unlock()

	url, err := mgr.StartQuickTunnel("test", 3000)
	if err != nil {
		t.Errorf("expected no error for existing tunnel, got %v", err)
	}
	if url != "https://existing.trycloudflare.com" {
		t.Errorf("expected existing URL, got %q", url)
	}
}

func TestTunnelManagerConcurrentAccess(t *testing.T) {
	mgr := NewTunnelManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mgr.mu.Lock()
			mgr.urls["test"] = "https://test.trycloudflare.com"
			mgr.mu.Unlock()

			url := mgr.GetURL("test")
			if url != "https://test.trycloudflare.com" && url != "" {
				t.Errorf("unexpected URL: %s", url)
			}
		}(i)
	}

	wg.Wait()
}

func TestStartQuickTunnelReturnsURL(t *testing.T) {
	mgr := NewTunnelManager()

	if testing.Short() {
		t.Skip("skipping tunnel test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	urlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		url, err := mgr.StartQuickTunnel("quick-test", 3000)
		if err != nil {
			errChan <- err
			return
		}
		urlChan <- url
	}()

	select {
	case url := <-urlChan:
		if url == "" {
			t.Error("expected non-empty URL")
		}
		if url != mgr.GetURL("quick-test") {
			t.Errorf("URL mismatch: got %q, expected %q", url, mgr.GetURL("quick-test"))
		}
	case err := <-errChan:
		t.Errorf("StartQuickTunnel failed: %v", err)
	case <-ctx.Done():
		t.Error("timeout waiting for tunnel URL")
	}

	mgr.StopTunnel("quick-test")
}

func TestTunnelManagerMultipleIDs(t *testing.T) {
	mgr := NewTunnelManager()

	mgr.mu.Lock()
	mgr.urls["tunnel-1"] = "https://tunnel-1.trycloudflare.com"
	mgr.urls["tunnel-2"] = "https://tunnel-2.trycloudflare.com"
	mgr.mu.Unlock()

	if url1 := mgr.GetURL("tunnel-1"); url1 != "https://tunnel-1.trycloudflare.com" {
		t.Errorf("unexpected URL for tunnel-1: %s", url1)
	}
	if url2 := mgr.GetURL("tunnel-2"); url2 != "https://tunnel-2.trycloudflare.com" {
		t.Errorf("unexpected URL for tunnel-2: %s", url2)
	}
	if url3 := mgr.GetURL("tunnel-3"); url3 != "" {
		t.Errorf("expected empty URL for non-existent tunnel-3, got: %s", url3)
	}
}
