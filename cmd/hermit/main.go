package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/JohnEsleyer/HermitShell/internal/api"
	"github.com/JohnEsleyer/HermitShell/internal/cloudflare"
	"github.com/JohnEsleyer/HermitShell/internal/db"
	"github.com/JohnEsleyer/HermitShell/internal/docker"
	"github.com/JohnEsleyer/HermitShell/internal/llm"
	"github.com/JohnEsleyer/HermitShell/internal/telegram"
	"github.com/joho/godotenv"
)

// version represents the current release of HermitShell server.
// Using Major.Minor.Patch versioning scheme.
var version = "v0.8.0"

// main is the entry point for the HermitShell server.
// It initializes dependencies and starts the Go Fiber API server.
func main() {
	ensureEnvFile()
	godotenv.Load()
	port := getEnv("PORT", "3000")
	dbPath := getEnv("DATABASE_PATH", "./data/hermit.db")

	// Convert to absolute path if relative
	if !filepath.IsAbs(dbPath) {
		absPath, err := filepath.Abs(dbPath)
		if err == nil {
			dbPath = absPath
		}
	}
	telegramToken := getEnv("TELEGRAM_BOT_TOKEN", "")
	llmProvider := getEnv("LLM_PROVIDER", "openrouter")
	llmModel := getEnv("LLM_MODEL", "openai/gpt-5.2")
	llmAPIKey := getEnv("LLM_API_KEY", "")
	openAIKey := getEnv("OPENAI_API_KEY", "")
	anthropicKey := getEnv("ANTHROPIC_API_KEY", "")
	geminiKey := getEnv("GEMINI_API_KEY", "")
	openRouterKey := getEnv("OPENROUTER_API_KEY", "")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}
	if err := ensureRuntimeData(filepath.Dir(dbPath)); err != nil {
		log.Fatalf("Failed to initialize runtime data directories: %v", err)
	}

	// 1. Initialize Database
	// Reference: See docs/architecture.md#data-layer
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()
	log.Printf("Database initialized: %s", dbPath)

	if err := database.InitDefaultUser(); err != nil {
		log.Fatalf("Failed to initialize default user: %v", err)
	}
	log.Printf("Default user initialized: admin (password must be changed on first login)")

	// 2. Initialize Telegram Bot
	// Reference: See docs/telegram-integration.md
	var bot *telegram.Bot
	if telegramToken != "" {
		bot = telegram.NewBot(telegramToken)
		log.Printf("Telegram bot initialized")
	}

	var llmClient *llm.Client
	provider := llm.Provider(llmProvider)
	baseURL := ""
	selectedKey := llmAPIKey

	switch provider {
	case llm.ProviderOpenRouter:
		if openRouterKey != "" {
			selectedKey = openRouterKey
		}
		if llmModel == "" {
			llmModel = "openai/gpt-5.2"
		}
		baseURL = "https://openrouter.ai/api/v1"
	case llm.ProviderAnthropic:
		if anthropicKey != "" {
			selectedKey = anthropicKey
		}
		if llmModel == "" || llmModel == "gpt-4o-mini" {
			llmModel = "claude-3-5-sonnet-latest"
		}
		baseURL = "https://api.anthropic.com/v1"
	case llm.ProviderGemini:
		if geminiKey != "" {
			selectedKey = geminiKey
		}
		if llmModel == "" || llmModel == "gpt-4o-mini" {
			llmModel = "gemini-1.5-pro"
		}
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		provider = llm.ProviderOpenRouter
		if openRouterKey != "" {
			selectedKey = openRouterKey
		} else if openAIKey != "" {
			selectedKey = openAIKey
		}
		if llmModel == "" {
			llmModel = "openai/gpt-5.2"
		}
		baseURL = "https://openrouter.ai/api/v1"
	}

	if selectedKey != "" {
		llmClient = llm.NewClient(
			llm.WithProvider(provider),
			llm.WithBaseURL(baseURL),
			llm.WithAPIKey(selectedKey),
			llm.WithModel(llmModel),
		)
		log.Printf("LLM client initialized: provider=%s model=%s", provider, llmModel)
	}

	// 3. Initialize Docker Engine
	// Reference: See docs/container-management.md
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Docker client: %v", err)
	}
	log.Printf("Docker client initialized")

	// Start background metrics collection
	dockerClient.StartMetricsAggregator()

	// 4. Setup Cloudflare Tunnels (if enabled)
	// Reference: See docs/cloudflared.md
	tunnelManager := cloudflare.NewTunnelManager()

	portInt := 3000
	fmt.Sscanf(port, "%d", &portInt)
	domainMode, _ := database.GetSetting("domain_mode")

	if domainMode != "true" {
		if err := tunnelManager.CheckBinary(); err != nil {
			log.Printf("WARNING: %v", err)
		} else {
			go func() {
				log.Printf("Starting cloudflared tunnel for dashboard...")
				url, err := tunnelManager.StartQuickTunnel("dashboard", portInt)
				if err != nil {
					log.Printf("Failed to start dashboard tunnel: %v", err)
					return
				}
				log.Printf("==> Dashboard Public URL: %s", url)
			}()

			go tunnelHealthMonitor(tunnelManager, portInt, dbPath)
		}
	}

	// 5. Start Background Tasks
	// Note: Calendar scheduler is now started inside NewServer()

	// 6. Start API Server
	// Reference: See docs/api-endpoints.md for how to create new endpoints.
	apiServer := api.NewServer(database, nil, bot, llmClient, dockerClient, tunnelManager)

	// Start Telegram polling for all existing agents with tokens
	// Reference: See docs/telegram-integration.md for long polling architecture.
	go func() {
		time.Sleep(2 * time.Second) // Give server time to initialize
		agents, _ := database.ListAgents()
		for _, a := range agents {
			if a.TelegramToken != "" {
				apiServer.StartPollingForAgent(a)
				log.Printf("Started Telegram poller for agent: %s", a.Name)
			}
		}
	}()

	log.Printf("Hermit %s (Go Fiber) starting on :%s ...", version, port)
	log.Printf("Dashboard available at: http://localhost:%s/", port)

	if err := apiServer.Listen(port); err != nil {
		log.Fatal(err)
	}
}

func tunnelHealthMonitor(tm *cloudflare.TunnelManager, port int, dbPath string) {
	time.Sleep(10 * time.Second)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !tm.CheckTunnelHealth("dashboard", 5*time.Second) {
			log.Printf("Tunnel health check failed, restarting tunnel...")

			tm.StopTunnel("dashboard")
			time.Sleep(2 * time.Second)

			cleanupStaleCloudflaredProcesses()

			url, err := tm.StartQuickTunnel("dashboard", port)
			if err != nil {
				log.Printf("Failed to restart tunnel: %v", err)
			} else {
				log.Printf("Tunnel restarted: %s", url)
			}
		}
	}
}

func cleanupStaleCloudflaredProcesses() {
	cmd := exec.Command("pkill", "-f", "cloudflared.*localhost:3000")
	if err := cmd.Run(); err != nil {
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func ensureRuntimeData(dataDir string) error {
	skillsDir := filepath.Join(dataDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}

	runtimeContextPath := filepath.Join(skillsDir, "context.md")

	// Always copy from source to ensure we have the latest version
	sourceFile, err := os.Open("./context.md")
	if err != nil {
		// If source doesn't exist in current dir, check if runtime version already exists
		if _, runtimeErr := os.Stat(runtimeContextPath); runtimeErr == nil {
			log.Printf("Note: ./context.md not found, using existing %s", runtimeContextPath)
			return nil
		}
		return err
	}
	defer sourceFile.Close()

	// Get source file info for comparison
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Check if runtime version exists and is up-to-date
	if runtimeInfo, err := os.Stat(runtimeContextPath); err == nil {
		// If source is older or same as runtime, skip update
		if !sourceInfo.ModTime().After(runtimeInfo.ModTime()) {
			return nil
		}
	}

	destFile, err := os.Create(runtimeContextPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err == nil {
		log.Printf("Updated %s from ./context.md", runtimeContextPath)
	}
	return err
}

func ensureEnvFile() {
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		return // Already exists
	}

	content := `# HermitShell Configuration
PORT=3000
DATABASE_PATH=./data/hermit.db
HERMIT_API_BASE=http://127.0.0.1:3000
HERMIT_CLI_USER=admin
HERMIT_CLI_PASS=hermit123

# Telegram (optional)
# TELEGRAM_BOT_TOKEN=

# LLM Providers
LLM_PROVIDER=openrouter
# OPENROUTER_API_KEY=
# OPENAI_API_KEY=
# ANTHROPIC_API_KEY=
# GEMINI_API_KEY=
`
	err := os.WriteFile(envPath, []byte(content), 0600)
	if err != nil {
		log.Printf("Warning: Failed to create default .env file: %v", err)
	} else {
		log.Printf("Generated default .env file with admin credentials.")
	}
}
