package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/joho/godotenv"
)

var apiBase = "http://localhost:3000"
var version = "v0.4.3"

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
	reqBody := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	req, _ := http.NewRequest("POST", apiBase+"/api/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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

func promptLogin() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		fmt.Println("Username and password required")
		os.Exit(1)
	}

	fmt.Print("Logging in...")
	if !login(username, password) {
		fmt.Println(" failed")
		fmt.Println("Invalid credentials")
		os.Exit(1)
	}
	fmt.Println(" OK")
	saveCredentials(username, password)
}

func main() {
	godotenv.Load()

	// Handle logout first - doesn't require being logged in
	if len(os.Args) >= 2 && os.Args[1] == "logout" {
		clearCredentials()
		fmt.Println("Logged out successfully")
		return
	}

	creds, err := loadCredentials()
	if err == nil && creds.Username != "" && creds.Password != "" {
		fmt.Print("Auto-login...")
		if login(creds.Username, creds.Password) {
			fmt.Println(" OK")
		} else {
			fmt.Println(" failed")
			clearCredentials()
			promptLogin()
		}
	} else {
		promptLogin()
	}

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
		fmt.Println("  logout      Logout and clear credentials")
		fmt.Println("  help        Show this help message")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  hermitshell agents list")
		fmt.Println("  hermitshell agents create --name rain --model gpt-4")
		fmt.Println("  hermitshell tunnel")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
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
	case "help", "-h", "--help":
		flag.Usage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
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
			os.Exit(1)
		}
		createAgent(*name, *role, *model, *provider)
	case "delete":
		id := delete.Int("id", 0, "Agent ID to delete")
		delete.Parse(args[1:])
		if *id == 0 {
			fmt.Println("Error: --id is required")
			os.Exit(1)
		}
		deleteAgent(*id)
	default:
		printAgents()
	}
}

func handleContainers(args []string, parent, list *flag.FlagSet) {
	printContainers()
}

func handleTunnel(tunnel *flag.FlagSet) {
	req, _ := http.NewRequest("GET", apiBase+"/api/tunnel-url", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if url, ok := result["url"].(string); ok && url != "" {
		fmt.Println(url)
	} else {
		fmt.Println("Tunnel not available")
		os.Exit(1)
	}
}

func printAgents() {
	req, _ := http.NewRequest("GET", apiBase+"/api/agents", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Agent '%s' created successfully (ID: %d)\n", name, int(result["id"].(float64)))
	} else {
		fmt.Printf("Failed to create agent: %v\n", result)
		os.Exit(1)
	}
}

func deleteAgent(id int) {
	req, _ := http.NewRequest("DELETE", apiBase+fmt.Sprintf("/api/agents/%d", id), nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Agent %d deleted successfully\n", id)
	} else {
		fmt.Printf("Failed to delete agent: %v\n", result)
		os.Exit(1)
	}
}

func printContainers() {
	req, _ := http.NewRequest("GET", apiBase+"/api/containers", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
