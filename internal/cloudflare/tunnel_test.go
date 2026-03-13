package cloudflare

import (
	"context"
	"regexp"
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

	// Test empty case
	url := mgr.GetURL("nonexistent")
	if url != "" {
		t.Errorf("expected empty string for nonexistent tunnel, got %q", url)
	}

	// Test with URL set
	mgr.urls["test-tunnel"] = "https://test.trycloudflare.com"
	url = mgr.GetURL("test-tunnel")
	if url != "https://test.trycloudflare.com" {
		t.Errorf("expected URL, got %q", url)
	}
}

func TestStopTunnel(t *testing.T) {
	mgr := NewTunnelManager()

	// Setup a mock tunnel
	_, cancel := context.WithCancel(context.Background())
	mgr.cancels["test"] = cancel
	mgr.urls["test"] = "https://test.trycloudflare.com"

	// Stop the tunnel
	mgr.StopTunnel("test")

	// Verify cleanup
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

	// Test with no URL
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
