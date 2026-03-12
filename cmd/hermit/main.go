package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/JohnEsleyer/hermit/internal/api"
	"github.com/JohnEsleyer/hermit/internal/cloudflare"
	"github.com/JohnEsleyer/hermit/internal/db"
	"github.com/JohnEsleyer/hermit/internal/docker"
	"github.com/JohnEsleyer/hermit/internal/llm"
	"github.com/JohnEsleyer/hermit/internal/telegram"
)

func main() {
	port := getEnv("PORT", "3000")
	dbPath := getEnv("DATABASE_PATH", "./data/hermit.db")
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

	dockerClient := docker.NewClient()
	log.Printf("Docker client initialized")

	tunnelManager := cloudflare.NewTunnelManager()

	portInt := 3000
	fmt.Sscanf(port, "%d", &portInt)
	domainMode, _ := database.GetSetting("domain_mode")

	if domainMode != "true" {
		go func() {
			log.Printf("Starting cloudflared tunnel for dashboard...")
			url, err := tunnelManager.StartQuickTunnel("dashboard", portInt)
			if err != nil {
				log.Printf("Failed to start dashboard tunnel: %v", err)
			} else {
				log.Printf("==> Dashboard Public URL: %s", url)
			}
		}()
	}

	apiServer := api.NewServer(database, nil, bot, llmClient, dockerClient, tunnelManager)

	fs := http.FileServer(http.Dir("./dashboard/public"))
	http.Handle("/dashboard/", http.StripPrefix("/dashboard/", fs))
	http.Handle("/", fs)

	http.HandleFunc("/api/auth/login", apiServer.HandleLogin)
	http.HandleFunc("/api/auth/logout", apiServer.HandleLogout)
	http.HandleFunc("/api/auth/change-password", apiServer.HandleChangePassword)
	http.HandleFunc("/api/auth/change-credentials", apiServer.HandleChangeCredentials)
	http.HandleFunc("/api/auth/check", apiServer.HandleCheckAuth)

	http.HandleFunc("/api/agent-tests/xml-contract", apiServer.HandleXMLContractTest)
	http.HandleFunc("/api/agents", apiServer.HandleAgents)
	http.HandleFunc("/api/agents/", apiServer.HandleAgentDetail)
	http.HandleFunc("/api/settings", apiServer.HandleSettings)
	http.HandleFunc("/api/context", apiServer.HandleContext)
	http.HandleFunc("/api/workspace/out", apiServer.HandleWorkspaceOut)
	http.HandleFunc("/api/docker/exec", apiServer.HandleDockerExec)
	http.HandleFunc("/api/docker/containers", apiServer.HandleDockerContainers)
	http.HandleFunc("/api/metrics", apiServer.HandleSystemMetrics)
	http.HandleFunc("/api/docker/files", apiServer.HandleDockerFiles)
	http.HandleFunc("/api/docker/download", apiServer.HandleDockerDownload)
	http.HandleFunc("/api/allowlist", apiServer.HandleAllowList)
	http.HandleFunc("/api/allowlist/", apiServer.HandleAllowListDetail)
	http.HandleFunc("/api/calendar", apiServer.HandleCalendar)
	http.HandleFunc("/api/calendar/", apiServer.HandleCalendarDetail)
	http.HandleFunc("/api/tunnels", apiServer.HandleTunnels)
	http.HandleFunc("/api/tunnels/", apiServer.HandleTunnelDetail)
	http.HandleFunc("/api/telegram/verify", apiServer.HandleTelegramVerify)
	http.HandleFunc("/webhook/", apiServer.HandleWebhook)

	log.Printf("Hermit (Go) starting on :%s ...", port)
	log.Printf("Dashboard available at: http://localhost:%s/dashboard/", port)
	log.Printf("Optimized for 1GB VPS. Memory footprint < 15MB.")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
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
	if _, err := os.Stat(runtimeContextPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sourceFile, err := os.Open("./context.md")
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(runtimeContextPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
