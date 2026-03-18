package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/joho/godotenv"
)

var apiBase = "http://127.0.0.1:3000"
var version = "v0.5.1"
var exitFunc = os.Exit
var loginFunc = login

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func getCredsPath() string {
	usr, _ := user.Current()
	dir := filepath.Join(usr.HomeDir, ".hermit")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "credentials")
}

func saveCredentials(username, password string) error {
	creds := Credentials{Username: username, Password: password}
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(getCredsPath(), data, 0600)
}

func loadCredentials() (Credentials, error) {
	data, err := os.ReadFile(getCredsPath())
	if err != nil {
		return Credentials{}, err
	}
	var creds Credentials
	err = json.Unmarshal(data, &creds)
	return creds, err
}

func clearCredentials() {
	os.Remove(getCredsPath())
}

func login(username, password string) bool {
	// Use JSON encoding for safer request bodies
	reqData := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(reqData)

	req, _ := http.NewRequest("POST", apiBase+"/api/auth/login", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nConnection Error: %v (Host: %s)\n", err, apiBase)
		return false
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if success, ok := result["success"].(bool); ok && success {
		return true
	}
	return false
}

// promptLogin collects credentials for CLI sign-in and explains the default first-run account.
// Docs: See docs/installation.md for CLI login defaults and required credential rotation guidance.
func promptLogin() {
	reader := bufio.NewReader(os.Stdin)

	// First-run guidance mirrors the installation documentation so new operators can sign in immediately.
	// Docs: See docs/installation.md for the default credentials and required settings update.
	fmt.Println("First-time login? Use the default credentials: admin / hermit123")
	fmt.Println("You are required to change these credentials in the settings dashboard after signing in.")
	fmt.Println()

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		fmt.Println("Username and password required")
		exitFunc(1)
		return
	}

	fmt.Print("Logging in...")
	if !loginFunc(username, password) {
		fmt.Println(" failed")
		fmt.Println("Invalid credentials. If this is your first run, sign in with admin / hermit123 and then change the credentials in the settings dashboard.")
		exitFunc(1)
		return
	}
	fmt.Println(" OK")
	saveCredentials(username, password)
}

func main() {
	godotenv.Load()
	if envBase := os.Getenv("HERMIT_API_BASE"); envBase != "" {
		apiBase = envBase
	}

	// Handle logout first - doesn't require being logged in
	if len(os.Args) >= 2 && os.Args[1] == "logout" {
		clearCredentials()
		fmt.Println("Logged out successfully")
		return
	}

	// 1. Try saved credentials
	creds, err := loadCredentials()
	if err == nil && creds.Username != "" && creds.Password != "" {
		fmt.Print("Auto-login...")
		if loginFunc(creds.Username, creds.Password) {
			fmt.Println(" OK")
			runCLI()
			return
		}
		fmt.Println(" failed")
		clearCredentials()
	}

	// 2. Try environment variables
	envUser := os.Getenv("HERMIT_CLI_USER")
	envPass := os.Getenv("HERMIT_CLI_PASS")
	if envUser != "" && envPass != "" {
		fmt.Printf("Logging in as %s from environment...", envUser)
		if loginFunc(envUser, envPass) {
			fmt.Println(" OK")
			saveCredentials(envUser, envPass)
			runCLI()
			return
		}
		fmt.Println(" failed")
	}

	// 3. Prompt user
	promptLogin()
	runCLI()
}

func runCLI() {
	agentsCmd := flag.NewFlagSet("agents", flag.ExitOnError)
	agentsListCmd := flag.NewFlagSet("list", flag.ExitOnError)
	agentsCreateCmd := flag.NewFlagSet("create", flag.ExitOnError)
	agentsDeleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	containersCmd := flag.NewFlagSet("containers", flag.ExitOnError)
	containersListCmd := flag.NewFlagSet("list", flag.ExitOnError)

	tunnelCmd := flag.NewFlagSet("tunnel", flag.ExitOnError)

	flag.Usage = func() {
		fmt.Printf("HermitShell CLI %s - AI Agent Orchestrator\n", version)
		fmt.Println("Reference: See docs/installation.md for CLI usage")
		fmt.Println("")
		fmt.Println("Usage: hermitshell <command> [subcommand] [options]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  agents      Manage agents")
		fmt.Println("  containers  Manage containers")
		fmt.Println("  tunnel      Get tunnel URL")
		fmt.Println("  status      Check server status")
		fmt.Println("  start       Start server service")
		fmt.Println("  stop        Stop server service")
		fmt.Println("  restart     Restart server service")
		fmt.Println("  logout      Logout and clear credentials")
		fmt.Println("  help        Show this help message")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  hermitshell agents list")
		fmt.Println("  hermitshell status")
		fmt.Println("  hermitshell tunnel")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		exitFunc(1)
		return
	}

	switch os.Args[1] {
	case "logout":
		clearCredentials()
		fmt.Println("Logged out successfully")
		return
	case "agents":
		handleAgents(os.Args[2:], agentsCmd, agentsListCmd, agentsCreateCmd, agentsDeleteCmd)
	case "containers":
		handleContainers(os.Args[2:], containersCmd, containersListCmd)
	case "tunnel":
		handleTunnel(tunnelCmd)
	case "status":
		handleStatus()
	case "start":
		handleService("start")
	case "stop":
		handleService("stop")
	case "restart":
		handleService("restart")
	case "help", "-h", "--help":
		flag.Usage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		flag.Usage()
		exitFunc(1)
		return
	}
}

func handleAgents(args []string, parent, list, create, delete *flag.FlagSet) {
	if len(args) < 1 {
		printAgents()
		return
	}

	switch args[0] {
	case "list":
		list.Parse(args[1:])
		printAgents()
	case "create":
		name := create.String("name", "", "Agent name (required)")
		role := create.String("role", "assistant", "Agent role")
		model := create.String("model", "openai/gpt-4", "LLM model")
		provider := create.String("provider", "openrouter", "LLM provider")
		create.Parse(args[1:])
		if *name == "" {
			fmt.Println("Error: --name is required")
			exitFunc(1)
			return
		}
		createAgent(*name, *role, *model, *provider)
	case "delete":
		id := delete.Int("id", 0, "Agent ID to delete")
		delete.Parse(args[1:])
		if *id == 0 {
			fmt.Println("Error: --id is required")
			exitFunc(1)
			return
		}
		deleteAgent(*id)
	default:
		printAgents()
	}
}

func handleContainers(args []string, parent, list *flag.FlagSet) {
	printContainers()
}

// handleTunnel tests the Cloudflare tunnel and displays the public URL.
// Reference: See docs/cloudflared.md for tunnel management and URL extraction.
func handleTunnel(tunnel *flag.FlagSet) {
	// First check if cloudflared binary is available
	if err := checkCloudflaredBinary(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		fmt.Println("Cloudflare tunnel may not be available")
	}

	// Fetch tunnel URL from API
	req, _ := http.NewRequest("GET", apiBase+"/api/tunnel-url", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	url, urlOk := result["url"].(string)
	healthy, healthyOk := result["healthy"].(bool)

	if urlOk && url != "" {
		status := "unknown"
		if healthyOk {
			if healthy {
				status = "healthy"
			} else {
				status = "unhealthy"
			}
		}
		fmt.Printf("Tunnel URL: %s\n", url)
		fmt.Printf("Status: %s\n", status)
		if healthyOk && !healthy {
			fmt.Println("Note: Tunnel may be still provisioning. Try again in a few seconds.")
		}
	} else {
		fmt.Println("Tunnel not available")
		if domainMode, _ := result["domainMode"].(bool); domainMode {
			fmt.Println("Domain mode is enabled. Use your configured domain instead.")
		}
		exitFunc(1)
		return
	}
}

// checkCloudflaredBinary verifies that the cloudflared CLI is installed.
// Reference: See docs/cloudflared.md for installation instructions.
func checkCloudflaredBinary() error {
	cmd := exec.Command("cloudflared", "--version")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("cloudflared CLI not found. Install from: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/get-started-guide/run-as-24-7-service/")
	}
	return nil
}

func printAgents() {
	req, _ := http.NewRequest("GET", apiBase+"/api/agents", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var agents []map[string]interface{}
	json.Unmarshal(body, &agents)

	if len(agents) == 0 {
		fmt.Println("No agents found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tMODEL\tPROVIDER")
	fmt.Fprintln(w, "--\t----\t------\t-----\t--------")

	for _, a := range agents {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			int(a["id"].(float64)),
			a["name"],
			a["status"],
			a["model"],
			a["provider"],
		)
	}
	w.Flush()

	fmt.Println("\nUse: hermitshell agents create --name <name> --model <model>")
	fmt.Println("     hermitshell agents delete --id <id>")
}

func createAgent(name, role, model, provider string) {
	reqBody := fmt.Sprintf(`{
		"name": "%s",
		"role": "%s",
		"model": "%s",
		"provider": "%s"
	}`, name, role, model, provider)

	req, _ := http.NewRequest("POST", apiBase+"/api/agents", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Agent '%s' created successfully (ID: %d)\n", name, int(result["id"].(float64)))
	} else {
		fmt.Printf("Failed to create agent: %v\n", result)
		exitFunc(1)
		return
	}
}

func deleteAgent(id int) {
	req, _ := http.NewRequest("DELETE", apiBase+fmt.Sprintf("/api/agents/%d", id), nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Agent %d deleted successfully\n", id)
	} else {
		fmt.Printf("Failed to delete agent: %v\n", result)
		exitFunc(1)
		return
	}
}

func printContainers() {
	req, _ := http.NewRequest("GET", apiBase+"/api/containers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var containers []map[string]interface{}
	json.Unmarshal(body, &containers)

	if len(containers) == 0 {
		fmt.Println("No containers found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCPU\tMEMORY")
	fmt.Fprintln(w, "--\t----\t------\t---\t------")

	for _, c := range containers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\t%.2f\n",
			c["id"],
			c["agentName"],
			c["status"],
			c["cpu"],
			c["memory"],
		)
	}
	w.Flush()
}
func handleStatus() {
	fmt.Println("Checking HermitShell Server status...")

	// Check systemd
	cmd := exec.Command("systemctl", "is-active", "hermit")
	output, _ := cmd.Output()
	systemdStatus := strings.TrimSpace(string(output))

	fmt.Printf("Systemd Service: %s\n", systemdStatus)

	// Check API
	fmt.Printf("API Endpoint (%s): ", apiBase)
	resp, err := http.Get(apiBase + "/api/auth/check")
	if err != nil {
		fmt.Println("OFFLINE ❌")
	} else {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("ONLINE ✅")
		} else {
			fmt.Printf("ERROR (HTTP %d) ⚠️\n", resp.StatusCode)
		}
	}
}

func handleService(action string) {
	fmt.Printf("Executing '%s' on hermit service...\n", action)
	// Use sudo for systemctl actions
	cmd := exec.Command("sudo", "systemctl", action, "hermit")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to %s service: %v\n", action, err)
		exitFunc(1)
		return
	}
	fmt.Println("Done.")
}
