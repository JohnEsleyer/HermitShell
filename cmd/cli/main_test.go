// Package main provides CLI tests for HermitShell.
//
// Reference: See docs/cloudflared.md for tunnel management details.
package main

import (
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandleTunnelWithHealthyTunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tunnel-url" {
			w.Write([]byte(`{"url": "https://abc123.trycloudflare.com", "healthy": true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	oldAPIBase := apiBase
	apiBase = server.URL
	defer func() { apiBase = oldAPIBase }()

	tunnelCmd := newTunnelCmd()
	handleTunnel(tunnelCmd)
}

func TestHandleTunnelWithUnhealthyTunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tunnel-url" {
			w.Write([]byte(`{"url": "https://abc123.trycloudflare.com", "healthy": false}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	oldAPIBase := apiBase
	apiBase = server.URL
	defer func() { apiBase = oldAPIBase }()

	tunnelCmd := newTunnelCmd()
	handleTunnel(tunnelCmd)
}

func TestHandleTunnelNoTunnel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tunnel-url" {
			w.Write([]byte(`{"url": "", "healthy": false}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	oldAPIBase := apiBase
	apiBase = server.URL
	defer func() { apiBase = oldAPIBase }()

	tunnelCmd := newTunnelCmd()

	exited := false
	exitFunc = func(code int) {
		exited = true
	}
	defer func() { exitFunc = os.Exit }()

	handleTunnel(tunnelCmd)

	if !exited {
		t.Error("Expected exit for no tunnel")
	}
}

func TestHandleTunnelDomainMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tunnel-url" {
			w.Write([]byte(`{"domainMode": true, "url": "", "healthy": false}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	oldAPIBase := apiBase
	apiBase = server.URL
	defer func() { apiBase = oldAPIBase }()

	tunnelCmd := newTunnelCmd()

	exited := false
	exitFunc = func(code int) {
		exited = true
	}
	defer func() { exitFunc = os.Exit }()

	handleTunnel(tunnelCmd)
	if !exited {
		t.Error("Expected exit for domain mode")
	}
}

func TestCheckCloudflaredBinary(t *testing.T) {
	err := checkCloudflaredBinary()
	if err != nil {
		t.Logf("cloudflared not found (expected on systems without cloudflared): %v", err)
	}
}

func newTunnelCmd() *flag.FlagSet {
	return flag.NewFlagSet("tunnel", flag.ExitOnError)
}

func TestPromptLoginShowsDefaultCredentialsGuidance(t *testing.T) {
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldExit := exitFunc
	oldLogin := loginFunc
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		exitFunc = oldExit
		loginFunc = oldLogin
	}()

	inputFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create temp stdin: %v", err)
	}
	if _, err := inputFile.WriteString("admin\nhermit123\n"); err != nil {
		t.Fatalf("write temp stdin: %v", err)
	}
	if _, err := inputFile.Seek(0, 0); err != nil {
		t.Fatalf("rewind temp stdin: %v", err)
	}
	os.Stdin = inputFile

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe

	loginCalled := false
	loginFunc = func(username, password string) bool {
		loginCalled = true
		return username == "admin" && password == "hermit123"
	}
	exitFunc = func(code int) {
		t.Fatalf("promptLogin unexpectedly exited with code %d", code)
	}

	promptLogin()
	writePipe.Close()

	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(output)

	if !loginCalled {
		t.Fatal("expected promptLogin to attempt login")
	}
	if !strings.Contains(out, "admin / hermit123") {
		t.Fatalf("expected default credentials in output, got %q", out)
	}
	if !strings.Contains(out, "settings dashboard") {
		t.Fatalf("expected settings dashboard guidance in output, got %q", out)
	}
}

func TestPromptLoginInvalidCredentialsMentionsFirstRunGuidance(t *testing.T) {
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldExit := exitFunc
	oldLogin := loginFunc
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		exitFunc = oldExit
		loginFunc = oldLogin
	}()

	inputFile, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatalf("create temp stdin: %v", err)
	}
	if _, err := inputFile.WriteString("admin\nwrongpass\n"); err != nil {
		t.Fatalf("write temp stdin: %v", err)
	}
	if _, err := inputFile.Seek(0, 0); err != nil {
		t.Fatalf("rewind temp stdin: %v", err)
	}
	os.Stdin = inputFile

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe

	loginFunc = func(username, password string) bool {
		return false
	}
	exited := false
	exitFunc = func(code int) {
		exited = true
	}

	promptLogin()
	writePipe.Close()

	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(output)

	if !exited {
		t.Fatal("expected promptLogin to exit on invalid credentials")
	}
	if !strings.Contains(out, "admin / hermit123") {
		t.Fatalf("expected first-run credentials reminder, got %q", out)
	}
	if !strings.Contains(out, "settings dashboard") {
		t.Fatalf("expected settings dashboard reminder, got %q", out)
	}
}
