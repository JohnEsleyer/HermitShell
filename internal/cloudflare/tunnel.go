package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	apiToken  string
	accountID string
	tunnelDir string
}

type Tunnel struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	DeletedAt string `json:"deleted_at"`
	ExpireAt  string `json:"expire_at"`
}

type TunnelCredentials struct {
	AccountTag   string `json:"AccountTag"`
	AccoundID    string `json:"AccoundID"`
	ZoneTag      string `json:"ZoneTag"`
	TunnelID     string `json:"TunnelID"`
	TunnelSecret string `json:"TunnelSecret"`
}

type CreateTunnelResponse struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type DNSCRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func NewClient(apiToken, accountID string) *Client {
	home, _ := os.UserHomeDir()
	tunnelDir := filepath.Join(home, ".hermit", "tunnels")
	os.MkdirAll(tunnelDir, 0755)

	return &Client{
		apiToken:  apiToken,
		accountID: accountID,
		tunnelDir: tunnelDir,
	}
}

func (c *Client) CreateTunnel(name string) (*CreateTunnelResponse, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/teamnet/tunnels", c.accountID)

	payload := map[string]interface{}{
		"name": name,
		"type": "cloudflared",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create tunnel: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create tunnel: %s", string(respBody))
	}

	var result struct {
		Result CreateTunnelResponse `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Result, nil
}

func (c *Client) GetTunnelCredentials(tunnelUUID string) (*TunnelCredentials, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/teamnet/tunnels/%s/credentials", c.accountID, tunnelUUID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tunnel credentials: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get tunnel credentials: %s", string(respBody))
	}

	var result struct {
		Result TunnelCredentials `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	credPath := filepath.Join(c.tunnelDir, tunnelUUID+".json")
	os.MkdirAll(filepath.Dir(credPath), 0755)
	os.WriteFile(credPath, respBody, 0600)

	return &result.Result, nil
}

func (c *Client) DeleteTunnel(tunnelUUID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/teamnet/tunnels/%s", c.accountID, tunnelUUID)

	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete tunnel: %w", err)
	}
	defer resp.Body.Close()

	credPath := filepath.Join(c.tunnelDir, tunnelUUID+".json")
	os.Remove(credPath)

	return nil
}

func (c *Client) CreateDNSRecord(zoneID, name, target string) (*DNSCRecord, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)

	payload := map[string]interface{}{
		"type":    "CNAME",
		"name":    name,
		"content": target,
		"proxied": true,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS record: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to create DNS record: %s", string(respBody))
	}

	var result struct {
		Result DNSCRecord `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result.Result, nil
}

func (c *Client) DeleteDNSRecord(zoneID, recordID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)

	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) GetZoneID(domain string) (string, error) {
	url := "https://api.cloudflare.com/client/v4/zones"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	q := req.URL.Query()
	q.Add("name", domain)
	q.Add("per_page", "1")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get zone: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get zone: %s", string(respBody))
	}

	var result struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Result) == 0 {
		return "", fmt.Errorf("zone not found")
	}

	return result.Result[0].ID, nil
}

func (c *Client) StartTunnel(tunnelUUID, port string) error {
	credPath := filepath.Join(c.tunnelDir, tunnelUUID+".json")
	if _, err := os.Stat(credPath); err != nil {
		return fmt.Errorf("credentials file not found: %w", err)
	}

	cmd := exec.Command("cloudflared", "tunnel", "--config", credPath, "run", tunnelUUID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	return nil
}

func GenerateTunnelName(agentName string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	rand.Seed(time.Now().UnixNano())
	randStr := make([]byte, 6)
	for i := range randStr {
		randStr[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("hermit-agent-%s-%s", strings.ToLower(agentName), string(randStr))
}

func (c *Client) ProvisionTunnel(agentName, domain, port string) (string, string, error) {
	tunnelName := GenerateTunnelName(agentName)

	tunnel, err := c.CreateTunnel(tunnelName)
	if err != nil {
		return "", "", fmt.Errorf("failed to create tunnel: %w", err)
	}

	_, err = c.GetTunnelCredentials(tunnel.UUID)
	if err != nil {
		c.DeleteTunnel(tunnel.UUID)
		return "", "", fmt.Errorf("failed to get credentials: %w", err)
	}

	zoneID, err := c.GetZoneID(domain)
	if err != nil {
		c.DeleteTunnel(tunnel.UUID)
		return "", "", fmt.Errorf("failed to get zone: %w", err)
	}

	hostname := fmt.Sprintf("%s.%s", tunnelName, domain)
	_, err = c.CreateDNSRecord(zoneID, hostname, tunnel.UUID+".cfargotunnel.com")
	if err != nil {
		c.DeleteTunnel(tunnel.UUID)
		return "", "", fmt.Errorf("failed to create DNS record: %w", err)
	}

	if err := c.StartTunnel(tunnel.UUID, port); err != nil {
		return "", "", fmt.Errorf("failed to start tunnel: %w", err)
	}

	return tunnel.UUID, "https://" + hostname, nil
}
