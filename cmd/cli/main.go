package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/joho/godotenv"
)

var (
	apiBase = getEnv("HERMIT_API_BASE", "http://localhost:3000")
	cliUser = getEnv("HERMIT_CLI_USER", "")
	cliPass = getEnv("HERMIT_CLI_PASS", "")
)

func main() {
	godotenv.Load()

	agentsCmd := flag.NewFlagSet("agents", flag.ExitOnError)
	agentsListCmd := flag.NewFlagSet("list", flag.ExitOnError)
	agentsCreateCmd := flag.NewFlagSet("create", flag.ExitOnError)
	agentsDeleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	containersCmd := flag.NewFlagSet("containers", flag.ExitOnError)
	containersListCmd := flag.NewFlagSet("list", flag.ExitOnError)

	tunnelCmd := flag.NewFlagSet("tunnel", flag.ExitOnError)

	loginCmd := flag.NewFlagSet("login", flag.ExitOnError)

	flag.Usage = func() {
		fmt.Println("Hermit CLI - AI Agent Orchestrator")
		fmt.Println("")
		fmt.Println("Usage: hermit-cli <command> [subcommand] [options]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  agents      Manage agents")
		fmt.Println("  containers  Manage containers")
		fmt.Println("  tunnel      Get tunnel URL")
		fmt.Println("  login       Login to dashboard")
		fmt.Println("  help        Show this help message")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  hermit-cli agents list")
		fmt.Println("  hermit-cli agents create --name rain --model gpt-4")
		fmt.Println("  hermit-cli tunnel")
		fmt.Println("  hermit-cli login --user admin --pass hermit123")
		fmt.Println("")
		fmt.Println("Environment variables:")
		fmt.Println("  HERMIT_API_BASE   API base URL (default: http://localhost:3000)")
		fmt.Println("  HERMIT_CLI_USER   CLI username")
		fmt.Println("  HERMIT_CLI_PASS   CLI password")
	}

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "agents":
		handleAgents(os.Args[2:], agentsCmd, agentsListCmd, agentsCreateCmd, agentsDeleteCmd)
	case "containers":
		handleContainers(os.Args[2:], containersCmd, containersListCmd)
	case "tunnel":
		handleTunnel(tunnelCmd)
	case "login":
		handleLogin(loginCmd)
	case "help", "-h", "--help":
		flag.Usage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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

func handleLogin(login *flag.FlagSet) {
	username := login.String("user", cliUser, "Username")
	password := login.String("pass", cliPass, "Password")
	login.Parse(os.Args[2:])

	if *username == "" || *password == "" {
		fmt.Println("Error: --user and --pass are required")
		os.Exit(1)
	}

	reqBody := fmt.Sprintf(`{"username":"%s","password":"%s"}`, *username, *password)
	req, _ := http.NewRequest("POST", apiBase+"/api/auth/login", strings.NewReader(reqBody))
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
		fmt.Println("Login successful!")
	} else {
		fmt.Println("Login failed")
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

	fmt.Println("\nUse: hermit-cli agents create --name <name> --model <model>")
	fmt.Println("     hermit-cli agents delete --id <id>")
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
