package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/JohnEsleyer/hermit/internal/api"
	"github.com/JohnEsleyer/hermit/internal/cloudflare"
	"github.com/JohnEsleyer/hermit/internal/db"
	"github.com/JohnEsleyer/hermit/internal/docker"
	"github.com/JohnEsleyer/hermit/internal/llm"
	"github.com/JohnEsleyer/hermit/internal/telegram"
)

var version = "v0.4.0"

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

	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Docker client: %v", err)
	}
	log.Printf("Docker client initialized")

	// Start background metrics collection
	dockerClient.StartMetricsAggregator()

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

				// Initial webhook update with retries
				go func() {
					log.Printf("Waiting for tunnel %s to propagate...", url)
					// Wait for the URL to be reachable from our side first
					for i := 0; i < 30; i++ {
						if tunnelManager.CheckTunnelHealth("dashboard", 5*time.Second) {
							log.Printf("Tunnel %s is reachable, setting up webhooks...", url)
							break
						}
						time.Sleep(5 * time.Second)
					}

					// Now try to set webhooks with retries
					for i := 0; i < 20; i++ {
						updateAgentWebhooks(database, tunnelManager, url)
						time.Sleep(20 * time.Second)
					}
				}()
			}()

			go tunnelHealthMonitor(tunnelManager, portInt)
			go webhookHealthMonitor(database, tunnelManager)
		}
	}

	// Start calendar scheduler
	go calendarScheduler(database)

	apiServer := api.NewServer(database, nil, bot, llmClient, dockerClient, tunnelManager)

	log.Printf("Hermit %s (Go Fiber) starting on :%s ...", version, port)
	log.Printf("Dashboard available at: http://localhost:%s/", port)

	if err := apiServer.Listen(port); err != nil {
		log.Fatal(err)
	}
}

func updateAgentWebhooks(database *db.DB, tm *cloudflare.TunnelManager, baseURL string) {
	agents, err := database.ListAgents()
	if err != nil {
		log.Printf("Failed to list agents: %v", err)
		return
	}
	for _, a := range agents {
		if a.TelegramToken != "" {
			tempBot := telegram.NewBot(a.TelegramToken)
			webhookURL := fmt.Sprintf("%s/api/webhook/%d", baseURL, a.ID)

			// Check if already set
			info, err := tempBot.GetWebhookInfo()
			if err == nil && info.URL == webhookURL {
				continue
			}

			// Try to set webhook
			if err := tempBot.SetWebhook(webhookURL); err != nil {
				// Only log if it's not a resolution error (to avoid noise during propagation)
				// unless it's been a while.
				if !strings.Contains(err.Error(), "Failed to resolve host") {
					log.Printf("Failed to set webhook for agent %d: %v", a.ID, err)
				}
			} else {
				log.Printf("SUCCESS: Webhook set for agent %d: %s", a.ID, webhookURL)
			}
		}
	}
}

func webhookHealthMonitor(database *db.DB, tm *cloudflare.TunnelManager) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		url := tm.GetURL("dashboard")
		if url != "" {
			updateAgentWebhooks(database, tm, url)
		}
	}
}

func tunnelHealthMonitor(tm *cloudflare.TunnelManager, port int) {
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

func calendarScheduler(database *db.DB) {
	log.Println("Calendar scheduler started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		events, err := database.GetPendingCalendarEvents()
		if err != nil {
			continue
		}

		if len(events) > 0 {
			log.Printf("Calendar scheduler: found %d pending events", len(events))
		}

		// Get timezone offset from settings
		timeOffset, _ := database.GetSetting("time_offset")
		offsetHours := 0
		if timeOffset != "" {
			fmt.Sscanf(timeOffset, "%d", &offsetHours)
		}

		// Apply timezone offset to get local time
		now := time.Now().Add(time.Duration(offsetHours) * time.Hour)
		log.Printf("Calendar scheduler: current local time %s (offset +%d)", now.Format("2006-01-02 15:04:05"), offsetHours)

		for _, event := range events {
			// Parse event datetime (stored in local time)
			eventTime, err := time.Parse("2006-01-02 15:04", event.Date+" "+event.Time)
			if err != nil {
				log.Printf("Calendar scheduler: failed to parse event time: %v", err)
				continue
			}

			log.Printf("Calendar scheduler: checking event %d at %s vs now %s", event.ID, eventTime.Format("15:04"), now.Format("15:04"))

			// Check if event time has passed (compare local times)
			if now.After(eventTime) || now.Equal(eventTime) {
				log.Printf("Calendar scheduler: triggering event %d", event.ID)

				// Get agent
				agent, err := database.GetAgent(event.AgentID)
				if err != nil {
					log.Printf("Calendar scheduler: failed to get agent: %v", err)
					continue
				}

				// Send reminder to user
				if agent.TelegramToken != "" {
					bot := telegram.NewBot(agent.TelegramToken)
					// Get allowed users
					allowedUsers := strings.Split(agent.AllowedUsers, ",")
					if len(allowedUsers) > 0 && allowedUsers[0] != "" {
						chatID := strings.TrimSpace(allowedUsers[0])
						log.Printf("Calendar scheduler: sending reminder to chat %s", chatID)
						bot.SendMessage(chatID, "🔔 *Reminder*\n\n"+event.Prompt)
						database.MarkCalendarEventExecuted(event.ID)
					}
				}
			}
		}
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
