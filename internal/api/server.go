// Package api provides the HTTP API server for Hermit.
// Documentation: See /docs folder for detailed guides:
// - authentication.md: Login, session management, password handling
// - security-measures.md: Security layers and protections
// - api-endpoints.md: How to create new endpoints
// - frontend-backend-communication.md: How React talks to Go backend
// - time-management.md: Time offset settings and display
// - concurrency.md: Goroutines, mutexes, and parallel operations
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohnEsleyer/HermitShell/internal/cloudflare"
	"github.com/JohnEsleyer/HermitShell/internal/crypto"
	"github.com/JohnEsleyer/HermitShell/internal/db"
	"github.com/JohnEsleyer/HermitShell/internal/docker"
	"github.com/JohnEsleyer/HermitShell/internal/llm"
	"github.com/JohnEsleyer/HermitShell/internal/parser"
	"github.com/JohnEsleyer/HermitShell/internal/telegram"
	"github.com/JohnEsleyer/HermitShell/internal/workspace"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var distFS embed.FS

type ModelPricing struct {
	InputPricePerMillion  float64
	OutputPricePerMillion float64
}

var modelContextWindowSize = map[string]int{
	"gpt-4o":                               128000,
	"gpt-4o-2024-05-13":                    128000,
	"gpt-4o-2024-08-27":                    128000,
	"gpt-4o-mini":                          128000,
	"gpt-4o-mini-2024-07-18":               128000,
	"gpt-4-turbo":                          128000,
	"gpt-4-turbo-2024-04-09":               128000,
	"gpt-4":                                8192,
	"gpt-4-32k":                            32768,
	"gpt-3.5-turbo":                        16385,
	"gpt-3.5-turbo-0125":                   16385,
	"claude-3-5-sonnet":                    200000,
	"claude-3-5-sonnet-20241022":           200000,
	"claude-3-5-haiku":                     200000,
	"claude-3-opus":                        200000,
	"claude-3-sonnet":                      200000,
	"claude-3-haiku":                       200000,
	"claude-2.1":                           200000,
	"claude-2.0":                           100000,
	"claude-instant":                       100000,
	"gemini-3.1-pro":                       1048576,
	"gemini-3.1-flash":                     1048576,
	"gemini-2.5-pro":                       1048576,
	"gemini-2.5-flash":                     1048576,
	"gemini-2.5-flash-lite":                1048576,
	"gemini-1.5-pro":                       200000,
	"gemini-1.5-pro-002":                   200000,
	"gemini-1.5-flash":                     1000000,
	"gemini-1.5-flash-002":                 1000000,
	"gemini-1.0-pro":                       32768,
	"gemini-pro":                           32768,
	"openai/gpt-4":                         8192,
	"openai/gpt-4-turbo":                   128000,
	"openai/gpt-4o":                        128000,
	"openai/gpt-4o-mini":                   128000,
	"openai/gpt-3.5-turbo":                 16385,
	"anthropic/claude-3.5-sonnet":          200000,
	"anthropic/claude-3.5-sonnet-20241022": 200000,
	"anthropic/claude-3-opus":              200000,
	"anthropic/claude-3-sonnet":            200000,
	"anthropic/claude-3-haiku":             200000,
	"google/gemini-1.5-pro":                200000,
	"google/gemini-1.5-flash":              1000000,
	"meta-llama/llama-3.1-405b-instruct":   128000,
	"meta-llama/llama-3.1-70b-instruct":    128000,
	"meta-llama/llama-3.1-8b-instruct":     128000,
	"meta-llama/llama-3-70b-instruct":      8192,
	"meta-llama/llama-3-8b-ininst":         8192,
}

var geminiPricing = map[string]ModelPricing{
	"gemini-3.1-pro":        {InputPricePerMillion: 2.00, OutputPricePerMillion: 12.00},
	"gemini-3.1-flash":      {InputPricePerMillion: 0.25, OutputPricePerMillion: 1.50},
	"gemini-2.5-pro":        {InputPricePerMillion: 1.25, OutputPricePerMillion: 10.00},
	"gemini-2.5-flash":      {InputPricePerMillion: 0.30, OutputPricePerMillion: 2.50},
	"gemini-2.5-flash-lite": {InputPricePerMillion: 0.10, OutputPricePerMillion: 0.40},
	"gemini-1.5-pro":        {InputPricePerMillion: 1.25, OutputPricePerMillion: 5.00},
	"gemini-1.5-flash":      {InputPricePerMillion: 0.075, OutputPricePerMillion: 0.30},
	"gemini-1.0-pro":        {InputPricePerMillion: 0.50, OutputPricePerMillion: 1.50},
	"gemini-pro":            {InputPricePerMillion: 0.50, OutputPricePerMillion: 1.50},
}

var geminiContextWindowSize = map[string]int{
	"gemini-3.1-pro":        1048576,
	"gemini-3.1-flash":      1048576,
	"gemini-2.5-pro":        1048576,
	"gemini-2.5-flash":      1048576,
	"gemini-2.5-flash-lite": 1048576,
	"gemini-1.5-pro":        200000,
	"gemini-1.5-flash":      1000000,
	"gemini-1.0-pro":        32768,
	"gemini-pro":            32768,
}

func getModelContextWindow(model string) int {
	modelLower := strings.ToLower(model)

	if strings.Contains(modelLower, "gemini") {
		for key, size := range geminiContextWindowSize {
			if strings.Contains(modelLower, strings.ToLower(key)) {
				return size
			}
		}
		return 1048576
	}

	if size, ok := modelContextWindowSize[model]; ok {
		return size
	}
	if size, ok := modelContextWindowSize[modelLower]; ok {
		return size
	}
	for key, size := range modelContextWindowSize {
		if strings.Contains(modelLower, strings.ToLower(key)) {
			return size
		}
	}
	return 128000
}

func getGeminiPricing(model string) ModelPricing {
	modelLower := strings.ToLower(model)
	for key, pricing := range geminiPricing {
		if strings.Contains(modelLower, strings.ToLower(key)) {
			return pricing
		}
	}
	return ModelPricing{InputPricePerMillion: 0.25, OutputPricePerMillion: 1.50}
}

func calculateTokenCost(tokenCount int, provider, model string) float64 {
	if strings.Contains(strings.ToLower(provider), "gemini") {
		pricing := getGeminiPricing(model)
		return (float64(tokenCount) / 1000000.0) * pricing.InputPricePerMillion
	}
	return 0
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ternary[T any](condition bool, whenTrue, whenFalse T) T {
	if condition {
		return whenTrue
	}
	return whenFalse
}

type AgentStats struct {
	WordCount     int     `json:"wordCount"`
	TokenEstimate int     `json:"tokenEstimate"`
	ContextWindow int     `json:"contextWindow"`
	HistoryCount  int     `json:"historyCount"`
	EstimatedCost float64 `json:"estimatedCost"`
	LLMAPICalls   int64   `json:"llmApiCalls"`
}

// Server handles HTTP requests and manages shared state with concurrency protection.
// Docs: See docs/concurrency.md for mutex and goroutine patterns used.
type Server struct {
	db      *db.DB
	ws      *workspace.Workspace
	bot     *telegram.Bot
	llm     *llm.Client
	docker  *docker.Client
	tunnels *cloudflare.TunnelManager
	app     *fiber.App

	verifyCodes   map[string]string
	verifyTokens  map[string]string
	takeoverMode  map[string]bool
	mu            sync.RWMutex
	contextStore  map[string][]string
	tokenCounters map[string]int

	containerStats map[string]docker.ContainerStats

	pollers   map[int64]context.CancelFunc
	pollersMu sync.Mutex

	wsClients map[*websocket.Conn]bool
	wsMutex   sync.Mutex

	schedulerRunning bool
	schedulerMu      sync.Mutex
}

func NewServer(database *db.DB, ws *workspace.Workspace, bot *telegram.Bot, llmClient *llm.Client, dockerClient *docker.Client, tunnels *cloudflare.TunnelManager) *Server {
	s := &Server{
		db:             database,
		ws:             ws,
		bot:            bot,
		llm:            llmClient,
		docker:         dockerClient,
		tunnels:        tunnels,
		verifyCodes:    make(map[string]string),
		verifyTokens:   make(map[string]string),
		takeoverMode:   make(map[string]bool),
		contextStore:   make(map[string][]string),
		tokenCounters:  make(map[string]int),
		containerStats: make(map[string]docker.ContainerStats),
		pollers:        make(map[int64]context.CancelFunc),
		wsClients:      make(map[*websocket.Conn]bool),
	}

	// Set default agent image if not already set or if it's the old remote image
	image, err := database.GetSetting("default_agent_image")
	if err != nil || image == "" || strings.HasPrefix(image, "hermit/") {
		database.SetSetting("default_agent_image", "hermit-agent:latest")
	}

	app := fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024,
	})

	app.Use(cors.New())
	app.Use(logger.New())

	s.setupRoutes(app)
	s.app = app

	// Initialize crypto key from default admin password (or a setting if we had one)
	// In a real scenario, this would be more dynamic.
	s.db.SetCryptoKey(crypto.DeriveKey("hermit123"))

	// Start background metrics broadcast
	go s.StartMetricsBroadcast()

	// Start calendar scheduler (sends reminders to agents for processing)
	// Run in goroutine so it doesn't block server startup
	go s.StartCalendarScheduler()

	return s
}

func (s *Server) Listen(port string) error {
	return s.app.Listen(":" + port)
}

// authMiddleware verifies that the request is authenticated.
func (s *Server) authMiddleware(c *fiber.Ctx) error {
	session := c.Cookies("session")
	if session == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			session = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if session == "" {
		return c.Status(401).JSON(fiber.Map{"success": false, "error": "Unauthorized"})
	}

	id, err := strconv.ParseInt(session, 10, 64)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "error": "Invalid session"})
	}

	username, _, err := s.db.GetUserByID(id)
	if err != nil || username == "" {
		return c.Status(401).JSON(fiber.Map{"success": false, "error": "Invalid session"})
	}

	c.Locals("userID", id)
	return c.Next()
}

// setupRoutes registers all API endpoints with Fiber router.
// Docs: See docs/api-endpoints.md for how to add new endpoints.
// Docs: See docs/frontend-backend-communication.md for frontend integration.
func (s *Server) setupRoutes(app *fiber.App) {
	app.Use("/api/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	api := app.Group("/api")

	api.Get("/ws", websocket.New(func(c *websocket.Conn) {
		s.wsMutex.Lock()
		s.wsClients[c] = true
		s.wsMutex.Unlock()

		defer func() {
			s.wsMutex.Lock()
			delete(s.wsClients, c)
			s.wsMutex.Unlock()
			c.Close()
		}()

		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	api.Post("/auth/login", s.HandleLogin)
	api.Post("/auth/logout", s.HandleLogout)
	api.Get("/auth/check", s.HandleCheckAuth)

	// Protected routes
	api.Use(s.authMiddleware)

	api.Post("/auth/change-credentials", s.HandleChangeCredentials)

	api.Get("/agents", s.HandleListAgents)
	api.Post("/agents", s.HandleCreateAgent)
	api.Get("/agents/:id", s.HandleGetAgent)
	api.Put("/agents/:id", s.HandleUpdateAgent)
	api.Delete("/agents/:id", s.HandleDeleteAgent)
	api.Post("/agents/:id/action", s.HandleAgentAction)
	api.Post("/agents/:id/chat", s.HandleAgentChat)
	api.Get("/agents/:id/logs", s.HandleGetAgentLogs)
	api.Get("/agents/:id/stats", s.HandleGetAgentStats)
	api.Get("/agents/:id/context", s.HandleGetAgentContextWindow)
	api.Get("/agents/:id/last-message", s.HandleGetLastMessage)
	api.Get("/agents/:id/unread", s.HandleGetUnreadCount)
	api.Post("/agents/:id/mark-seen", s.HandleMarkMessagesSeen)

	api.Get("/skills", s.HandleListSkills)
	api.Post("/skills", s.HandleCreateSkill)
	api.Put("/skills/:id", s.HandleUpdateSkill)
	api.Delete("/skills/:id", s.HandleDeleteSkill)
	api.Get("/skills/context", s.HandleGetContextSkill)
	api.Post("/skills/context/reset", s.HandleResetContextSkill)

	api.Get("/calendar", s.HandleListCalendar)
	api.Post("/calendar", s.HandleCreateCalendarEvent)
	api.Put("/calendar/:id", s.HandleUpdateCalendarEvent)
	api.Delete("/calendar/:id", s.HandleDeleteCalendarEvent)

	api.Get("/allowlist", s.HandleListAllowlist)
	api.Post("/allowlist", s.HandleCreateAllowlist)
	api.Delete("/allowlist/:id", s.HandleDeleteAllowlist)

	api.Get("/metrics", s.HandleMetrics)
	api.Get("/logs", s.HandleGetLogs)
	api.Get("/containers", s.HandleContainers)
	api.Delete("/containers/:id", s.HandleTerminateContainer)
	api.Post("/containers/:id/action", s.HandleContainerAction)
	api.Get("/containers/:id/files", s.HandleContainerFiles)
	api.Post("/containers/:id/upload", s.HandleContainerUpload)
	api.Get("/containers/:id/download", s.HandleContainerDownload)

	api.Get("/settings", s.HandleGetSettings)
	api.Post("/settings", s.HandleSetSettings)
	api.Get("/tunnel-url", s.HandleGetTunnelURL)
	api.Get("/time", s.HandleGetTime)
	api.Get("/apps", s.HandleGetAllApps)

	// Backup and Restore - Export/Import all app data
	// Docs: See docs/backup-restore.md for detailed documentation
	api.Get("/backup/export", s.HandleExportBackup)
	api.Post("/backup/import", s.HandleImportBackup)

	api.Post("/test-contract", s.HandleTestContract)

	api.Post("/images/upload", s.HandleImageUpload)

	api.Post("/telegram/send-code", s.HandleTelegramSendCode)
	api.Post("/telegram/verify", s.HandleTelegramVerify)

	// Agent Specific Skills
	api.Get("/agents/:id/skills", s.HandleListAgentSkills)
	api.Post("/agents/:id/skills", s.HandleSaveSkill)
	api.Delete("/agents/:id/skills/:skillId", s.HandleDeleteSkill)

	// App serving
	app.Get("/apps/:agentId/:appName/*", s.HandleServeApp)
	app.Get("/apps/:agentId/:appName", s.HandleServeApp)
	api.Get("/agents/:id/apps", s.HandleListApps)

	s.setupStaticRoutes(app)
}

// HandleAgentChat handles incoming chat messages from mobile client / web HTTP
func (s *Server) HandleAgentChat(c *fiber.Ctx) error {
	agentID, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	agent, err := s.db.GetAgent(agentID)
	if err != nil || agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	userText := strings.TrimSpace(req.Message)

	// Decrypt message if encrypted (handle both enc: and cbc: prefixes)
	if userText != "" {
		if strings.HasPrefix(userText, "cbc:") {
			log.Printf("[DEBUG] Message is encrypted (CBC), attempting decryption...")
			decrypted, err := crypto.Decrypt(userText[4:], crypto.DeriveKey("hermit123"))
			if err != nil {
				log.Printf("[ERROR] Failed to decrypt CBC message: %v", err)
				return c.Status(400).JSON(fiber.Map{"error": "Failed to decrypt message"})
			}
			userText = decrypted
			log.Printf("[DEBUG] Successfully decrypted CBC message: '%s'", userText)
		} else if strings.HasPrefix(userText, "enc:") {
			log.Printf("[DEBUG] Message is encrypted (GCM), attempting decryption...")
			decrypted, err := crypto.Decrypt(userText[4:], crypto.DeriveKey("hermit123"))
			if err != nil {
				log.Printf("[ERROR] Failed to decrypt GCM message: %v", err)
				return c.Status(400).JSON(fiber.Map{"error": "Failed to decrypt message"})
			}
			userText = decrypted
			log.Printf("[DEBUG] Successfully decrypted GCM message: '%s'", userText)
		} else {
			log.Printf("[DEBUG] Message is NOT encrypted: '%s'", userText)
		}
	}

	if userText == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Message cannot be empty"})
	}

	userID := "mobile"
	chatID := "mobile-chat"

	authHeader := c.Get("Authorization")
	session := strings.TrimPrefix(authHeader, "Bearer ")
	if session == "" {
		session = c.Cookies("session")
	}
	if session != "" {
		userID = session
	}

	currentTime := s.db.GetSystemTime()
	userTextWithTime := fmt.Sprintf("[Current System Time: %s] %s", currentTime.Format("2006-01-02 15:04:05"), userText)

	// Handle slash commands locally without LLM
	// Slash commands are deterministic and processed by the system directly.
	log.Printf("[DEBUG] Checking for slash command: userText='%s', starts with /: %v", userText, strings.HasPrefix(userText, "/"))
	if strings.HasPrefix(userText, "/") {
		log.Printf("[DEBUG] Detected slash command, calling handleLocalCommand")
		return s.handleLocalCommand(c, agent, userID, userText)
	}

	// Check if takeover mode is enabled for this chat
	// In takeover mode, user is in the driver's seat and system processes XML tags from user input
	takeoverMode := s.takeoverMode[chatID]

	// Parse user input for XML tags
	parsedInput := parser.ParseLLMOutput(userText)
	hasXMLTags := hasSystemExecutionInput(parsedInput)

	// If user sends XML commands and takeover is off, reject them
	if hasXMLTags && !takeoverMode {
		s.addHistoryAndBroadcastWithRejection(agent.ID, userID, "user", userText, true)
		s.addHistoryAndBroadcast(agent.ID, "system", "system", "System: XML commands are not allowed when takeover mode is off. Enable takeover mode to issue commands directly.")
		return c.JSON(fiber.Map{
			"message":  "System: XML commands are not allowed when takeover mode is off. Enable takeover mode to issue commands directly.",
			"role":     "system",
			"rejected": true,
		})
	}

	// In takeover mode: User is in control, process XML tags from user input
	if takeoverMode {
		if hasXMLTags {
			s.addHistoryAndBroadcast(agent.ID, userID, "user", userText)
			processedLog := describeParsedTags(parsedInput)
			s.addHistoryAndBroadcast(agent.ID, "system", "system", processedLog)
			s.db.LogAction(agent.ID, "system", "takeover_input", processedLog)

			feedback := s.ExecuteXMLPayload(agent.ID, chatID, userText, nil)
			files := extractExecutionFiles(parsedInput)

			// In takeover mode, system responds directly with files
			if parsedInput.Message != "" || len(files) > 0 {
				s.addHistoryAndBroadcastWithFiles(agent.ID, chatID, "assistant", parsedInput.Message, files)
			}

			response := formatSystemExecutionResponse(parsedInput, feedback)
			s.addHistoryAndBroadcast(agent.ID, "system", "system", response)
			return c.JSON(fiber.Map{"message": response, "files": files, "role": "system"})
		} else {
			// No XML tags detected in takeover mode
			s.addHistoryAndBroadcast(agent.ID, userID, "user", userText)
			s.addHistoryAndBroadcast(agent.ID, "system", "system", "System: No XML commands detected. In takeover mode, use XML tags like <terminal>, <give>, <calendar>, etc.")
			return c.JSON(fiber.Map{"message": "System: No XML commands detected", "role": "system"})
		}
	}

	// Non-takeover mode: LLM is in the driver's seat
	// System processes XML tags from LLM response only
	s.db.LogAction(agent.ID, "agent", "ai_processing", fmt.Sprintf("Processing HTTP chat message from user %s", userID))

	history, _ := s.db.GetHistory(agent.ID, 10)
	var messages []llm.Message

	// Build system prompt: ALWAYS use global context.md as base instructions
	// This ensures all agents have the proper instructions regardless of agent-specific config
	systemPrompt := agent.Personality

	// Load global context.md from HermitShell root directory
	if content, err := os.ReadFile("./context.md"); err == nil {
		contextStr := strings.TrimSpace(string(content))
		if contextStr != "" {
			// Replace placeholders with agent-specific values
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_NAME}}", agent.Name)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_ROLE}}", agent.Role)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_PERSONALITY}}", agent.Personality)
			systemPrompt = contextStr
		}
	}

	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})
	messages = append(messages, llm.Message{Role: "user", Content: userTextWithTime})

	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		role := h.Role
		if role == "system" {
			role = "user"
		}
		messages = append(messages, llm.Message{Role: role, Content: h.Content})
	}

	client := s.getLLMClientForAgent(agent)
	if client == nil {
		config := s.getLLMConfigStatus(agent)
		message := "LLM client not configured"
		if !config.Configured {
			message = "LLM client not configured: missing " + config.missingSummary()
		}
		s.addHistoryAndBroadcast(agent.ID, "system", "system", "Error: "+message)
		return c.Status(500).JSON(fiber.Map{"error": message})
	}

	s.db.LogAction(agent.ID, "network", "llm_request", fmt.Sprintf("Provider: %s, Model: %s, Messages: %d", agent.Provider, agent.Model, len(messages)))

	// Retry helper for missing <message> tag
	maxRetries := 1
	var response string
	var parsed parser.ParsedResponse

	for attempt := 0; attempt <= maxRetries; attempt++ {
		response, err = client.Chat(agent.Model, messages)

		s.db.IncrementLLMAPICalls(agent.ID)
		contextWindow := getModelContextWindow(agent.Model)
		s.db.UpdateAgentContextWindow(agent.ID, contextWindow)

		if err != nil {
			s.addHistoryAndBroadcast(agent.ID, "system", "system", "LLM Error: "+err.Error())
			return c.Status(500).JSON(fiber.Map{"error": "AI Error: " + err.Error()})
		}

		// Parse the LLM response to extract message content and files
		parsed = parser.ParseLLMOutput(response)

		s.db.LogAction(agent.ID, "agent", "llm_response", fmt.Sprintf("Response (attempt %d): %.200s...", attempt+1, response))
		s.addHistoryAndBroadcast(agent.ID, userID, "user", userText)

		// Success if <message> tag found
		if parsed.Message != "" {
			break
		}

		// If no <message> tag and we have retries left, add explicit reminder
		if attempt < maxRetries {
			log.Printf("[RETRY] No <message> tag found, retrying with reminder (attempt %d/%d)", attempt+1, maxRetries+1)
			s.addHistoryAndBroadcast(agent.ID, "system", "system", fmt.Sprintf("Retry %d/%d: Your last response had no <message> tags. Remember: ALL visible text must be wrapped in <message> tags, or it will be ignored.", attempt+1, maxRetries+1))
			messages = append(messages, llm.Message{Role: "system", Content: "Reminder: Your response must contain <message>...</message> tags around ALL visible text. Without these tags, your response will be completely ignored by the user."})
		}
	}

	// Final check: if still no <message> tag after retries, reject
	if parsed.Message == "" {
		s.addHistoryAndBroadcast(agent.ID, "system", "system", "LLM Error: Response missing required <message> tag after retries")
		return c.Status(400).JSON(fiber.Map{"error": "Response missing required <message> tag. All visible text must be wrapped in <message> tags."})
	}

	// Extract files from parsed response
	var files []string
	for _, action := range parsed.Actions {
		if action.Type == "GIVE" {
			files = append(files, action.Value)
		}
	}

	// Only broadcast the parsed message content, not raw response
	// This ensures <thought> tags are kept internal and only <message> content goes to users
	s.addHistoryAndBroadcast(agent.ID, "assistant", "assistant", parsed.Message)

	// Process XML tags from LLM response (terminal, give, calendar, etc.)
	feedback := s.ExecuteXMLPayload(agent.ID, chatID, response, nil)
	if len(feedback) > 0 {
		feedbackJSON, _ := json.Marshal(feedback)
		s.addHistoryAndBroadcast(agent.ID, "system", "system", string(feedbackJSON))
	}

	// Return the message content (without XML tags) and files
	// System messages are NOT encrypted
	// Role indicates the source: "assistant" for LLM, "system" for system messages
	return c.JSON(fiber.Map{
		"message": parsed.Message,
		"files":   files,
		"role":    "assistant",
	})
}

func (s *Server) HandleServeApp(c *fiber.Ctx) error {
	agentID, _ := strconv.ParseInt(c.Params("agentId"), 10, 64)
	appName := c.Params("appName")
	file := c.Params("*")
	if file == "" {
		file = "index.html"
	}

	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
	}

	containerPath := "/app/workspace/apps/" + appName + "/" + file
	content, err := s.docker.ReadFile(containerName, containerPath)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "File not found in container"})
	}

	contentType := "text/plain"
	if strings.HasSuffix(file, ".html") {
		contentType = "text/html"
	} else if strings.HasSuffix(file, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(file, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(file, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(file, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(file, ".jpg") || strings.HasSuffix(file, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(file, ".svg") {
		contentType = "image/svg+xml"
	}

	c.Set("Content-Type", contentType)
	return c.SendString(content)
}

func (s *Server) HandleListAgentSkills(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	skills, err := s.db.ListSkills()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var result []db.Skill

	agentSkillsPath := fmt.Sprintf("data/agents/%d/skills", id)
	contextData, err := os.ReadFile(filepath.Join(agentSkillsPath, "context.md"))
	if err == nil {
		result = append(result, db.Skill{
			ID:          -1,
			AgentID:     id,
			Title:       "context.md",
			Description: "Agent personality and context (built-in)",
			Content:     string(contextData),
		})
	}

	for _, sk := range skills {
		if sk.AgentID == 0 || sk.AgentID == id {
			result = append(result, *sk)
		}
	}

	return c.JSON(result)
}

func (s *Server) HandleSaveSkill(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var skill db.Skill
	if err := c.BodyParser(&skill); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}
	skill.AgentID = id

	if skill.ID == -1 && skill.Title == "context.md" {
		agentSkillsPath := fmt.Sprintf("data/agents/%d/skills", id)
		os.MkdirAll(agentSkillsPath, 0755)
		err := os.WriteFile(filepath.Join(agentSkillsPath, "context.md"), []byte(skill.Content), 0644)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true})
	}

	var err error
	if skill.ID > 0 {
		err = s.db.UpdateSkill(&skill)
	} else {
		_, err = s.db.CreateSkill(&skill)
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) setupStaticRoutes(app *fiber.App) {
	distPath := "./dashboard/dist"

	app.Static("/data/image", "./data/image")
	app.Static("/", distPath)

	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/apps") {
			return c.Next()
		}
		return c.SendFile(distPath + "/index.html")
	})
}

func (s *Server) HandleContainerUpload(c *fiber.Ctx) error {
	containerID := c.Params("id")
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "No file uploaded"})
	}

	agents, _ := s.db.ListAgents()
	var agent *db.Agent
	for _, a := range agents {
		contName := a.ContainerID
		if contName == "" {
			contName = "agent-" + strings.ToLower(a.Name)
		}
		if contName == containerID {
			agent = a
			break
		}
	}

	if agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	}

	// Save to workspace/in
	targetDir := filepath.Join("data", "agents", agent.Name, "workspace", "in")
	os.MkdirAll(targetDir, 0755)
	dst := filepath.Join(targetDir, file.Filename)

	if err := c.SaveFile(file, dst); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save file: " + err.Error()})
	}

	s.db.LogAction(agent.ID, "system", "file_uploaded", fmt.Sprintf("File '%s' uploaded to workspace/in", file.Filename))

	return c.JSON(fiber.Map{"success": true, "filename": file.Filename})
}

// HandleLogin processes user authentication.
// Docs: See docs/authentication.md for login flow and security details.
// Docs: See docs/security-measures.md for HTTP-only cookie implementation.
func (s *Server) HandleLogin(c *fiber.Ctx) error {
	var req struct{ Username, Password string }
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	id, mustChange, err := s.db.VerifyUser(req.Username, req.Password)
	if err != nil || id == 0 {
		return c.JSON(fiber.Map{"success": false, "error": "Invalid credentials"})
	}

	// Set HTTP-only cookie for session - prevents JavaScript access (XSS protection)
	// See docs/security-measures.md for security details
	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    fmt.Sprintf("%d", id),
		Path:     "/",
		HTTPOnly: true,
	})

	return c.JSON(fiber.Map{
		"success":            true,
		"token":              fmt.Sprintf("%d", id),
		"mustChangePassword": mustChange,
	})
}

func (s *Server) HandleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleCheckAuth(c *fiber.Ctx) error {
	session := c.Cookies("session")
	if session == "" {
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			session = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if session == "" {
		return c.JSON(fiber.Map{"authenticated": false})
	}

	id, _ := strconv.ParseInt(session, 10, 64)
	username, mustChange, err := s.db.GetUserByID(id)
	if err != nil || username == "" {
		return c.JSON(fiber.Map{"authenticated": false})
	}
	return c.JSON(fiber.Map{"authenticated": true, "username": username, "mustChangePassword": mustChange})
}

func (s *Server) HandleChangeCredentials(c *fiber.Ctx) error {
	var req struct{ NewUsername, NewPassword string }
	c.BodyParser(&req)

	session := c.Cookies("session")
	id, _ := strconv.ParseInt(session, 10, 64)
	username, _, _ := s.db.GetUserByID(id)

	if err := s.db.UpdateCredentials(username, req.NewUsername, req.NewPassword); err != nil {
		return c.JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) getSystemHealth() (fiber.Map, error) {
	metrics, err := s.docker.LatestSystemMetrics()
	if err != nil {
		return nil, err
	}

	agents, _ := s.db.ListAgents()
	dockerMap := make(map[string]docker.ContainerStats)
	for _, cont := range metrics.Containers {
		dockerMap[strings.ToLower(cont.Name)] = cont
	}

	var allContainers []fiber.Map
	for _, a := range agents {
		contName := strings.ToLower(a.ContainerID)
		if contName == "" {
			contName = "agent-" + strings.ToLower(a.Name)
		}

		status := "stopped"
		var cpu, mem float64
		if stats, ok := dockerMap[contName]; ok {
			status = "running"
			cpu = stats.CPUPercent
			mem = stats.MemUsageMB
		}

		allContainers = append(allContainers, fiber.Map{
			"name":       contName,
			"agentName":  a.Name,
			"status":     status,
			"cpu":        cpu,
			"cpuPercent": cpu,
			"memory":     mem,
			"memUsageMB": mem,
		})
	}

	for _, cont := range metrics.Containers {
		isAgent := false
		lowerName := strings.ToLower(cont.Name)
		for _, a := range agents {
			if lowerName == strings.ToLower(a.ContainerID) || lowerName == "agent-"+strings.ToLower(a.Name) {
				isAgent = true
				break
			}
		}
		if !isAgent {
			allContainers = append(allContainers, fiber.Map{
				"name":       cont.Name,
				"agentName":  "System",
				"status":     "active",
				"cpu":        cont.CPUPercent,
				"cpuPercent": cont.CPUPercent,
				"memory":     cont.MemUsageMB,
				"memUsageMB": cont.MemUsageMB,
			})
		}
	}

	tunnelEnabled, _ := s.db.GetSetting("tunnel_enabled")
	tunnelURL := ""
	if tunnelEnabled != "false" {
		tunnelURL = s.tunnels.GetURL("dashboard")
	}

	return fiber.Map{
		"host":       metrics.Host,
		"containers": allContainers,
		"tunnelURL":  tunnelURL,
	}, nil
}

func (s *Server) HandleMetrics(c *fiber.Ctx) error {
	health, err := s.getSystemHealth()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(health)
}

func (s *Server) StartMetricsBroadcast() {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		health, err := s.getSystemHealth()
		if err != nil {
			continue
		}

		payload := fiber.Map{
			"type":   "system_health",
			"health": health,
		}

		msg, _ := json.Marshal(payload)
		s.wsMutex.Lock()
		for client := range s.wsClients {
			client.WriteMessage(websocket.TextMessage, msg)
		}
		s.wsMutex.Unlock()
	}
}

// StartCalendarScheduler monitors calendar events and triggers agents when they fire.
// Instead of sending reminders directly, it injects the prompt as a user message
// to the agent's conversation, letting the agent process and respond naturally.
func (s *Server) StartCalendarScheduler() {
	s.schedulerMu.Lock()
	if s.schedulerRunning {
		s.schedulerMu.Unlock()
		return
	}
	s.schedulerRunning = true
	s.schedulerMu.Unlock()

	log.Println("Calendar scheduler started (agent-aware mode)")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.processScheduledEvents()
	}
}

func (s *Server) processScheduledEvents() {
	// Use mutex to prevent duplicate processing of the same event
	s.schedulerMu.Lock()
	defer s.schedulerMu.Unlock()

	events, err := s.db.GetPendingCalendarEvents()
	if err != nil {
		return
	}

	if len(events) > 0 {
		log.Printf("[SCHEDULER] Found %d pending events", len(events))
	}

	currentTime := s.db.GetSystemTime()

	for _, event := range events {
		// Parse event datetime (stored in local time)
		eventTime, err := time.Parse("2006-01-02 15:04", event.Date+" "+event.Time)
		if err != nil {
			log.Printf("[SCHEDULER] Failed to parse event time: %v", err)
			continue
		}

		// Check if event time has passed
		if currentTime.After(eventTime) || currentTime.Equal(eventTime) {
			log.Printf("[SCHEDULER] Triggering event %d: %s", event.ID, event.Prompt)

			// Get agent
			agent, err := s.db.GetAgent(event.AgentID)
			if err != nil {
				log.Printf("[SCHEDULER] Failed to get agent: %v", err)
				continue
			}

			// Send reminder to agent as a new user message
			// Use special source tag so agent knows not to re-schedule
			// Format: [SCHEDULED_REMINDER] to distinguish from regular user messages
			reminderMessage := fmt.Sprintf("[SCHEDULED_REMINDER] %s", event.Prompt)

			// Add as user message to history
			chatID := "scheduler"
			if agent.TelegramToken != "" {
				allowedUsers := strings.Split(agent.AllowedUsers, ",")
				if len(allowedUsers) > 0 && allowedUsers[0] != "" {
					chatID = strings.TrimSpace(allowedUsers[0])
				}
			}

			s.db.AddHistory(agent.ID, chatID, "user", reminderMessage)
			s.addHistoryAndBroadcast(agent.ID, chatID, "user", reminderMessage)

			// Mark as executed BEFORE triggering agent to prevent race conditions
			s.db.MarkCalendarEventExecuted(event.ID)

			// Trigger agent to process this message
			go s.triggerAgentForReminder(agent.ID, chatID, reminderMessage)
		}
	}
}

// triggerAgentForReminder sends the reminder to the agent for processing
func (s *Server) triggerAgentForReminder(agentID int64, chatID, message string) {
	log.Printf("[SCHEDULER] Triggering agent %d for reminder processing", agentID)

	// Get LLM client for agent
	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		log.Printf("[SCHEDULER] Failed to get agent: %v", err)
		return
	}

	client := s.getLLMClientForAgent(agent)
	if client == nil {
		log.Printf("[SCHEDULER] No LLM client configured for agent %d", agentID)
		return
	}

	// Build messages with system prompt and reminder
	messages := s.buildAgentMessages(agent, message)

	// Get LLM response
	response, err := client.Chat(agent.Model, messages)
	if err != nil {
		log.Printf("[SCHEDULER] LLM error: %v", err)
		return
	}

	// Parse and process response
	parsed := parser.ParseLLMOutput(response)
	if parsed.Message != "" {
		// Broadcast agent's response
		s.addHistoryAndBroadcast(agent.ID, chatID, "assistant", parsed.Message)

		// Send to Telegram if configured
		if agent.TelegramToken != "" {
			bot := telegram.NewBot(agent.TelegramToken)
			bot.SendMessage(chatID, parsed.Message)
		}

		// Execute any XML actions (terminal, give, etc.)
		feedback := s.ExecuteXMLPayload(agent.ID, chatID, response, nil)
		if len(feedback) > 0 {
			feedbackJSON, _ := json.Marshal(feedback)
			s.addHistoryAndBroadcast(agent.ID, "system", "system", string(feedbackJSON))
		}
	} else {
		log.Printf("[SCHEDULER] Agent response missing <message> tag, sending fallback")
		fallback := "🔔 Reminder: " + message
		s.addHistoryAndBroadcast(agent.ID, chatID, "assistant", fallback)
		if agent.TelegramToken != "" {
			bot := telegram.NewBot(agent.TelegramToken)
			bot.SendMessage(chatID, fallback)
		}
	}
}

// buildAgentMessages constructs the message list for agent processing
func (s *Server) buildAgentMessages(agent *db.Agent, userMessage string) []llm.Message {
	var messages []llm.Message

	// Load system prompt from context.md
	systemPrompt := agent.Personality
	if content, err := os.ReadFile("./context.md"); err == nil {
		contextStr := strings.TrimSpace(string(content))
		if contextStr != "" {
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_NAME}}", agent.Name)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_ROLE}}", agent.Role)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_PERSONALITY}}", agent.Personality)
			systemPrompt = contextStr
		}
	}

	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})

	// Add current time info
	currentTime := s.db.GetSystemTime()
	userMessageWithTime := fmt.Sprintf("[System time: %s]\n\n%s", currentTime.Format("2006-01-02 15:04:05"), userMessage)
	messages = append(messages, llm.Message{Role: "user", Content: userMessageWithTime})

	// Add recent history for context
	history, _ := s.db.GetHistory(agent.ID, 10)
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		role := h.Role
		if role == "system" {
			role = "user"
		}
		messages = append(messages, llm.Message{Role: role, Content: h.Content})
	}

	return messages
}

type LogWithAgent struct {
	ID        int64  `json:"id"`
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name"`
	AgentPic  string `json:"agent_pic"`
	UserID    string `json:"user_id"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) HandleGetLogs(c *fiber.Ctx) error {
	category := c.Query("category", "all")
	limit, _ := strconv.Atoi(c.Query("limit", "100"))

	logs, err := s.db.GetAllAuditLogs(category, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	agents, err := s.db.ListAgents()
	if err != nil {
		agents = []*db.Agent{}
	}

	agentMap := make(map[int64]*db.Agent)
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	typeLogMap := make(map[string]string)
	typeLogMap["all"] = ""
	typeLogMap["system"] = "system"
	typeLogMap["agent"] = "agent"
	typeLogMap["docker"] = "docker"
	typeLogMap["network"] = "network"

	filteredLogs := logs
	if cat, ok := typeLogMap[category]; ok && cat != "" {
		var filtered []*db.AuditLog
		isNetwork := category == "network"
		for _, log := range logs {
			if isNetwork {
				if strings.HasPrefix(log.Action, "network") || strings.HasPrefix(log.Action, "tunnel") {
					filtered = append(filtered, log)
				}
			} else if strings.HasPrefix(log.Action, cat) {
				filtered = append(filtered, log)
			}
		}
		filteredLogs = filtered
	}

	var response []LogWithAgent
	for _, log := range filteredLogs {
		agentName := ""
		agentPic := ""
		if agent, ok := agentMap[log.AgentID]; ok {
			agentName = agent.Name
			agentPic = agent.ProfilePic
		}
		response = append(response, LogWithAgent{
			ID:        log.ID,
			AgentID:   log.AgentID,
			AgentName: agentName,
			AgentPic:  agentPic,
			UserID:    log.UserID,
			Action:    log.Action,
			Details:   log.Details,
			CreatedAt: log.CreatedAt,
		})
	}

	return c.JSON(response)
}

func (s *Server) HandleContainers(c *fiber.Ctx) error {
	metrics, err := s.docker.LatestSystemMetrics()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	agents, _ := s.db.ListAgents()

	type ContainerInfo struct {
		ID          string  `json:"id"`
		AgentID     string  `json:"agentId"`
		AgentName   string  `json:"agentName"`
		ProfilePic  string  `json:"profilePic"`
		Status      string  `json:"status"`
		CPU         float64 `json:"cpu"`
		Memory      float64 `json:"memory"`
		ContainerID string  `json:"containerId"`
		CreatedAt   string  `json:"createdAt"`
		UpdatedAt   string  `json:"updatedAt"`
	}

	var containers []ContainerInfo
	dockerMap := make(map[string]docker.ContainerStats)
	for _, cont := range metrics.Containers {
		dockerMap[strings.ToLower(cont.Name)] = cont
	}

	for _, a := range agents {
		contName := strings.ToLower(a.ContainerID)
		if contName == "" {
			contName = "agent-" + strings.ToLower(a.Name)
		}

		// Check if container exists in Docker (running or stopped)
		stats, exists := dockerMap[contName]

		status := "stopped"
		var cpu, mem float64
		created := a.CreatedAt

		if exists && stats.Status == "running" {
			status = "running"
			cpu = stats.CPUPercent
			mem = stats.MemUsageMB
			if stats.Created != "" {
				created = stats.Created
			}
		}

		// Show container if it exists (running or stopped) OR if we have a ContainerID set
		if exists || a.ContainerID != "" {
			containers = append(containers, ContainerInfo{
				ID:          contName,
				AgentID:     fmt.Sprintf("%d", a.ID),
				AgentName:   a.Name,
				ProfilePic:  a.ProfilePic,
				Status:      status,
				CPU:         cpu,
				Memory:      mem,
				ContainerID: contName,
				CreatedAt:   created,
				UpdatedAt:   a.UpdatedAt,
			})
		}
	}

	for _, cont := range metrics.Containers {
		lowerName := strings.ToLower(cont.Name)
		isAgent := false
		for _, a := range agents {
			if lowerName == strings.ToLower(a.ContainerID) || lowerName == "agent-"+strings.ToLower(a.Name) {
				isAgent = true
				break
			}
		}
		if !isAgent {
			containers = append(containers, ContainerInfo{
				ID:          lowerName,
				AgentID:     "",
				AgentName:   "System: " + cont.Name,
				ProfilePic:  "",
				Status:      "active",
				CPU:         cont.CPUPercent,
				Memory:      cont.MemUsageMB,
				ContainerID: lowerName,
				CreatedAt:   cont.Created,
				UpdatedAt:   "",
			})
		}
	}

	return c.JSON(containers)
}

func (s *Server) HandleContainerFiles(c *fiber.Ctx) error {
	containerID := c.Params("id")
	path := c.Query("path", "/app/workspace")

	agents, _ := s.db.ListAgents()
	var agent *db.Agent
	for _, a := range agents {
		contName := a.ContainerID
		if contName == "" {
			contName = "agent-" + strings.ToLower(a.Name)
		}
		if contName == containerID {
			agent = a
			break
		}
	}

	if agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	}

	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
	}

	files, err := s.docker.ListContainerFiles(containerName, path)
	if err != nil {
		return c.JSON(fiber.Map{"path": path, "files": []interface{}{}})
	}

	var result []map[string]interface{}
	for _, f := range files {
		result = append(result, map[string]interface{}{
			"name":    f.Name,
			"size":    f.Size,
			"isDir":   f.IsDir,
			"modTime": f.ModTime,
		})
	}

	return c.JSON(fiber.Map{
		"path":  path,
		"files": result,
	})
}

func (s *Server) HandleContainerDownload(c *fiber.Ctx) error {
	containerID := c.Params("id")
	filename := c.Query("file")
	folder := c.Query("folder", "out")

	if filename == "" {
		return c.Status(400).JSON(fiber.Map{"error": "file query parameter required"})
	}

	agents, _ := s.db.ListAgents()
	var agent *db.Agent
	for _, a := range agents {
		contName := a.ContainerID
		if contName == "" {
			contName = "agent-" + strings.ToLower(a.Name)
		}
		if contName == containerID {
			agent = a
			break
		}
	}

	if agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	}

	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
	}

	containerPath := "/app/workspace/" + folder + "/" + filename
	content, err := s.docker.ReadFile(containerName, containerPath)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "file not found in container"})
	}

	tmpPath := filepath.Join(os.TempDir(), filename)
	os.WriteFile(tmpPath, []byte(content), 0644)
	defer os.Remove(tmpPath)

	return c.Download(tmpPath, filename)
}

func (s *Server) HandleListAgents(c *fiber.Ctx) error {
	agents, err := s.db.ListAgents()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	type AgentResponse struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Role          string `json:"role"`
		Personality   string `json:"personality"`
		Provider      string `json:"provider"`
		Status        string `json:"status"`
		TunnelURL     string `json:"tunnelUrl"`
		ProfilePic    string `json:"profilePic"`
		BannerURL     string `json:"bannerUrl"`
		Background    string `json:"background"`
		ContainerID   string `json:"containerId"`
		AllowedUsers  string `json:"allowedUsers"`
		Model         string `json:"model"`
		TelegramID    string `json:"telegramId"`
		TelegramToken string `json:"telegramToken"`
	}

	var result []AgentResponse
	for _, a := range agents {
		tunnelURL := a.TunnelURL
		if tunnelURL == "" {
			tunnelURL = s.tunnels.GetURL(fmt.Sprintf("agent-%d", a.ID))
		}

		result = append(result, AgentResponse{
			ID:            a.ID,
			Name:          a.Name,
			Role:          a.Role,
			Personality:   a.Personality,
			Provider:      a.Provider,
			Status:        a.Status,
			TunnelURL:     tunnelURL,
			ProfilePic:    a.ProfilePic,
			BannerURL:     a.BannerURL,
			Background:    a.Background,
			ContainerID:   a.ContainerID,
			AllowedUsers:  a.AllowedUsers,
			Model:         a.Model,
			TelegramID:    a.TelegramID,
			TelegramToken: a.TelegramToken,
		})
	}

	return c.JSON(result)
}

func (s *Server) HandleCreateAgent(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		Role          string `json:"role"`
		Personality   string `json:"personality"`
		Provider      string `json:"provider"`
		ProfilePic    string `json:"profilePic"`
		BannerURL     string `json:"bannerUrl"`
		Background    string `json:"background"`
		TelegramToken string `json:"telegramToken"`
		TelegramID    string `json:"telegramId"`
		Status        string `json:"status"`
		Model         string `json:"model"`
		AllowedUsers  string `json:"allowedUsers"`
		Platform      string `json:"platform"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}

	a := db.Agent{
		Name:          req.Name,
		Role:          req.Role,
		Personality:   req.Personality,
		Provider:      req.Provider,
		ProfilePic:    req.ProfilePic,
		BannerURL:     req.BannerURL,
		Background:    req.Background,
		TelegramToken: req.TelegramToken,
		TelegramID:    req.TelegramID,
		Status:        "standby",
		Platform:      req.Platform,
		Active:        true,
	}
	if a.Platform == "" {
		a.Platform = "telegram"
	}
	if a.Role == "" {
		a.Role = "assistant"
	}
	if a.Provider == "" {
		a.Provider = "openrouter"
	}
	if a.Background == "" {
		a.Background = "doodle"
	}
	a.Model = req.Model
	a.AllowedUsers = req.AllowedUsers

	id, err := s.db.CreateAgent(&a)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Log agent creation
	s.db.LogAction(id, "system", "agent_created", fmt.Sprintf("Agent '%s' created with provider=%s, model=%s", a.Name, a.Provider, a.Model))

	// Start Telegram polling for this agent if token is provided and platform is telegram
	if a.Platform == "telegram" && a.TelegramToken != "" {
		go func() {
			time.Sleep(1 * time.Second) // Give DB time to settle
			agent, _ := s.db.GetAgent(id)
			if agent != nil {
				s.StartPollingForAgent(agent)
			}
		}()
	}

	// Create agent-specific skills folder with context.md
	// Initialize with global context.md as base, personality will be prepended in system prompt
	agentSkillsPath := fmt.Sprintf("data/agents/%d/skills", id)
	os.MkdirAll(agentSkillsPath, 0755)

	// Copy global context.md as base for agent-specific context
	if globalContext, err := os.ReadFile("./context.md"); err == nil {
		os.WriteFile(filepath.Join(agentSkillsPath, "context.md"), globalContext, 0644)
	} else {
		// Fallback: empty context (will use global)
		os.WriteFile(filepath.Join(agentSkillsPath, "context.md"), []byte(""), 0644)
	}

	// Create and start Docker container for the agent
	go func() {
		time.Sleep(500 * time.Millisecond)
		existing, err := s.db.GetAgent(id)
		if err == nil && existing != nil {
			// Set container ID
			containerName := "agent-" + strings.ToLower(existing.Name)
			existing.ContainerID = containerName
			s.db.UpdateAgent(existing)

			// Create and start the container
			if s.docker != nil {
				image, _ := s.db.GetSetting("default_agent_image")
				if image == "" {
					image = "hermit-agent:latest"
				}

				log.Printf("Creating container %s with image %s for agent %s", containerName, image, existing.Name)
				err := s.docker.Run(containerName, image, true)
				if err != nil {
					log.Printf("Failed to create container for agent %s: %v", existing.Name, err)
					s.db.LogAction(existing.ID, "docker", "container_creation_failed", err.Error())
				} else {
					existing.Status = "running"
					s.db.UpdateAgent(existing)
					s.db.LogAction(existing.ID, "docker", "container_created", "Container created and started for new agent")
					log.Printf("Container %s created and started successfully for agent %s", containerName, existing.Name)
				}
			}
		}
	}()

	return c.JSON(fiber.Map{"id": id, "success": true})
}

func (s *Server) HandleGetAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	agent, _ := s.db.GetAgent(id)
	return c.JSON(agent)
}

func (s *Server) HandleUpdateAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	existing, err := s.db.GetAgent(id)
	if err != nil || existing == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	var req struct {
		Name          string `json:"name"`
		Role          string `json:"role"`
		Personality   string `json:"personality"`
		Provider      string `json:"provider"`
		ProfilePic    string `json:"profilePic"`
		BannerURL     string `json:"bannerUrl"`
		Background    string `json:"background"`
		Model         string `json:"model"`
		AllowedUsers  string `json:"allowedUsers"`
		TelegramID    string `json:"telegramId"`
		TelegramToken string `json:"telegramToken"`
		Platform      string `json:"platform"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Role != "" {
		existing.Role = req.Role
	}
	existing.Personality = req.Personality
	if req.Provider != "" {
		existing.Provider = req.Provider
	}
	existing.ProfilePic = req.ProfilePic
	existing.BannerURL = req.BannerURL
	if req.Background != "" {
		existing.Background = req.Background
	}
	existing.Model = req.Model
	existing.AllowedUsers = req.AllowedUsers
	if req.TelegramID != "" {
		existing.TelegramID = req.TelegramID
	}
	if req.TelegramToken != "" {
		existing.TelegramToken = req.TelegramToken
	}
	if req.Platform != "" {
		existing.Platform = req.Platform
	}

	if err := s.db.UpdateAgent(existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Restart polling if Telegram token changed
	if req.TelegramToken != "" {
		s.StartPollingForAgent(existing)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleDeleteAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)

	// Stop polling for this agent before deletion
	s.StopPollingForAgent(id)

	// Get agent to delete its container
	agent, _ := s.db.GetAgent(id)
	if agent != nil && s.docker != nil {
		containerName := agent.ContainerID
		if containerName == "" {
			containerName = "agent-" + strings.ToLower(agent.Name)
		}
		// Stop and remove the container
		s.docker.Stop(containerName)
		s.docker.Remove(containerName)
		s.db.LogAction(id, "docker", "container_deleted", "Container deleted with agent")
	}

	s.db.DeleteAgent(id)
	s.db.DeleteTunnelByAgentID(id)

	// Log agent deletion
	s.db.LogAction(0, "system", "agent_deleted", fmt.Sprintf("Agent ID %d deleted from dashboard", id))

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleImageUpload(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "missing image file"})
	}

	kind := c.FormValue("type", "asset")
	ext := strings.ToLower(path.Ext(file.Filename))
	if ext == "" {
		ext = ".png"
	}
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" && ext != ".gif" {
		return c.Status(400).JSON(fiber.Map{"error": "unsupported image type"})
	}

	if err := os.MkdirAll("data/image", 0o755); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	filename := fmt.Sprintf("%s-%d%s", kind, time.Now().UnixNano(), ext)
	dst := filepath.Join("data/image", filename)
	if err := c.SaveFile(file, dst); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"url": "/data/image/" + filename})
}

func (s *Server) HandleTerminateContainer(c *fiber.Ctx) error {
	containerID := c.Params("id")
	if s.docker != nil {
		s.docker.Stop(containerID)
		s.docker.Remove(containerID)
	}

	// Also mark associated agent as standby if found
	agents, _ := s.db.ListAgents()
	for _, a := range agents {
		if a.ContainerID == containerID {
			a.Status = "standby"
			s.db.UpdateAgent(a)
			break
		}
	}

	return c.JSON(fiber.Map{"success": true})
}

// HandleContainerAction manages container lifecycle (start/stop/reset).
// Docs: See docs/container-management.md for container lifecycle details.
func (s *Server) HandleContainerAction(c *fiber.Ctx) error {
	containerID := c.Params("id")
	var req struct {
		Action string `json:"action"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	if s.docker == nil {
		return c.Status(500).JSON(fiber.Map{"error": "Docker not available"})
	}

	switch req.Action {
	case "start":
		agents, _ := s.db.ListAgents()
		agentID := int64(0)
		for _, a := range agents {
			lowerCID := strings.ToLower(a.ContainerID)
			if lowerCID == containerID || containerID == "agent-"+strings.ToLower(a.Name) {
				_, err := s.ensureAgentContainer(a)
				if err != nil {
					s.db.LogAction(a.ID, "docker", "container_start_failed", err.Error())
					return c.Status(500).JSON(fiber.Map{"error": err.Error()})
				}
				agentID = a.ID
				s.db.LogAction(a.ID, "docker", "container_started", "Container started successfully")
				break
			}
		}
		if agentID == 0 {
			s.db.LogAction(0, "docker", "container_start_attempt", "Attempted to start non-linked container: "+containerID)
		}
	case "stop":
		s.docker.Stop(containerID)
		agents, _ := s.db.ListAgents()
		for _, a := range agents {
			lowerCID := strings.ToLower(a.ContainerID)
			if lowerCID == containerID || containerID == "agent-"+strings.ToLower(a.Name) {
				a.Status = "stopped"
				s.db.UpdateAgent(a)
				s.db.LogAction(a.ID, "docker", "container_stopped", "Container stopped")
				break
			}
		}
	case "reset":
		s.docker.Stop(containerID)
		s.docker.Remove(containerID)
		image := "hermit-agent:latest"
		err := s.docker.Run(containerID, image, true)
		agents, _ := s.db.ListAgents()
		for _, a := range agents {
			lowerCID := strings.ToLower(a.ContainerID)
			if lowerCID == containerID || containerID == "agent-"+strings.ToLower(a.Name) {
				if err != nil {
					s.db.LogAction(a.ID, "docker", "container_reset_failed", err.Error())
				} else {
					s.db.LogAction(a.ID, "docker", "container_reset", "Container reset successfully")
				}
				break
			}
		}
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleAgentAction(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var req struct {
		Action string `json:"action"`
	}
	c.BodyParser(&req)

	agent, _ := s.db.GetAgent(id)
	if agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
	}

	switch req.Action {
	case "start":
		if _, err := s.ensureAgentContainer(agent); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		agent.Status = "running"
	case "stop":
		if s.docker != nil {
			s.docker.Stop(containerName)
		}
		agent.Status = "stopped"
	case "reset":
		if s.docker != nil {
			s.docker.Stop(containerName)
			s.docker.Remove(containerName)
			image, _ := s.db.GetSetting("default_agent_image")
			if image == "" {
				image = "hermit-agent:latest"
			}
			err := s.docker.Run(containerName, image, true)
			if err != nil {
				s.db.LogAction(agent.ID, "docker", "container_reset_failed", err.Error())
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			s.db.LogAction(agent.ID, "docker", "container_reset", "Container reset successfully")
		}
		agent.Status = "running"
	}

	s.db.UpdateAgent(agent)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleGetAgentLogs(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	history, _ := s.db.GetHistory(id, 100)
	return c.JSON(history)
}

// handleLocalCommand processes commands without requiring LLM
func (s *Server) handleLocalCommand(c *fiber.Ctx, agent *db.Agent, userID, command string) error {
	parts := strings.Fields(command)
	cmd := parts[0]

	response := ""
	processedLog := fmt.Sprintf("Processed slash command: %s", cmd)

	switch cmd {
	case "/status":
		config := s.getLLMConfigStatus(agent)
		response = fmt.Sprintf("🤖 *Agent Status: %s*\n\n", agent.Name)
		response += fmt.Sprintf("• Provider: `%s`\n", config.ProviderUI)
		response += fmt.Sprintf("• Model: `%s`\n", firstNonEmpty(config.Model, "Not set"))
		response += fmt.Sprintf("• Model Type: `%s`\n", config.ModelType)
		response += fmt.Sprintf("• API Key: `%s`\n", ternary(config.APIKeySet, "Configured", "Missing"))
		response += fmt.Sprintf("• LLM Ready: `%s`\n", ternary(config.Configured, "Yes", "No"))
		if !config.Configured {
			response += fmt.Sprintf("• Missing: `%s`\n", config.missingSummary())
		}
		response += fmt.Sprintf("• Context Window: `%d` tokens\n", agent.ContextWindow)
		response += fmt.Sprintf("• LLM API Calls: `%d`\n", agent.LLMAPICalls)

		containerStatus := "Stopped"
		if agent.ContainerID != "" && s.docker != nil {
			if s.docker.IsRunning(agent.ContainerID) {
				containerStatus = "Running ✅"
			} else {
				containerStatus = "Stopped ❌"
			}
		}
		response += fmt.Sprintf("• Container: `%s` (%s)\n", agent.ContainerID, containerStatus)

		response += "• Connection: HTTP Active ✅\n"

		response += fmt.Sprintf("\n🔐 *Authorization*\n")
		response += fmt.Sprintf("• Allowed Users: `%s`\n", agent.AllowedUsers)
		if agent.AllowedUsers == "" {
			response += "• Status: ✅ No restrictions\n"
		} else {
			response += "• Status: ⚠️ Restricted\n"
		}

		tunnelURL := ""
		if s.tunnels != nil {
			tunnelURL = s.tunnels.GetURL("dashboard")
		}
		if tunnelURL != "" {
			response += fmt.Sprintf("\n🌐 *Dashboard*: `%s`\n", tunnelURL)
		}

	case "/reset":
		log.Printf("[SLASH] Processing /reset command for agent %s", agent.Name)
		if agent.ContainerID != "" && s.docker != nil {
			s.docker.Stop(agent.ContainerID)
			s.docker.Remove(agent.ContainerID)
			image, _ := s.db.GetSetting("default_agent_image")
			if image == "" {
				image = "hermit-agent:latest"
			}
			err := s.docker.Run(agent.ContainerID, image, true)
			if err != nil {
				response = fmt.Sprintf("❌ Failed to reset container: %v", err)
				log.Printf("[SLASH] /reset failed: %v", err)
			} else {
				response = "✅ Container reset successfully"
				s.db.LogAction(agent.ID, "docker", "container_reset", "Container reset from mobile client")
				log.Printf("[SLASH] /reset completed successfully")
			}
		} else {
			response = "❌ No container configured for this agent"
			log.Printf("[SLASH] /reset: no container configured")
		}

	case "/clear":
		log.Printf("[SLASH] Processing /clear command for agent %s", agent.Name)
		// Clear chat history (NOT context.md - that contains important instructions)
		s.db.ClearHistory(agent.ID)
		s.broadcastConversationCleared(agent.ID)
		log.Printf("[SLASH] /clear: history cleared, broadcast sent")

		response = "✅ Chat history cleared. Context window preserved."
		s.addHistoryAndBroadcast(agent.ID, userID, "user", command)
		s.addHistoryAndBroadcast(agent.ID, "system", "system", processedLog)
		s.addHistoryAndBroadcast(agent.ID, "system", "system", response)
		s.db.LogAction(agent.ID, "system", "slash_command", processedLog)
		log.Printf("[SLASH] /clear: completed, returning: %s", response)
		return c.JSON(fiber.Map{"message": response, "role": "system"})

	default:
		response = fmt.Sprintf("Unknown command: %s", cmd)
	}

	s.addHistoryAndBroadcast(agent.ID, userID, "user", command)
	s.addHistoryAndBroadcast(agent.ID, "system", "system", processedLog)
	s.addHistoryAndBroadcast(agent.ID, "system", "system", response)
	s.db.LogAction(agent.ID, "system", "slash_command", processedLog)

	log.Printf("[SLASH] Returning response: message='%s', role='system'", response)
	return c.JSON(fiber.Map{"message": response, "role": "system"})
}

func (s *Server) HandleGetAgentStats(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent id"})
	}

	agent, err := s.db.GetAgent(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	}

	history, err := s.db.GetHistory(id, 1000)
	if err != nil {
		history = []*db.HistoryEntry{}
	}

	contextPath := fmt.Sprintf("data/agents/%d/skills/context.md", id)
	contextData, _ := os.ReadFile(contextPath)

	var totalWords int
	var tokenEstimate int
	var contextWindow int

	isGemini := strings.Contains(strings.ToLower(agent.Provider), "gemini")

	if isGemini {
		geminiKey, _ := s.db.GetSetting("gemini_api_key")
		if geminiKey != "" {
			ctx := context.Background()
			client, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
			if err == nil {
				defer client.Close()

				modelName := agent.Model
				if !strings.HasPrefix(modelName, "models/") {
					modelName = "models/" + modelName
				}

				fullContent := ""
				if contextData != nil {
					fullContent = string(contextData)
				}
				for _, h := range history {
					fullContent += "\n" + h.Content
				}

				if fullContent != "" {
					model := client.GenerativeModel(modelName)
					resp, err := model.CountTokens(ctx, genai.Text(fullContent))
					if err == nil {
						tokenEstimate = int(resp.TotalTokens)
						totalWords = int(float64(tokenEstimate) * 0.75)
						contextWindow = getModelContextWindow(agent.Model)
					}
				}
			}
		}

		if contextWindow == 0 {
			for _, h := range history {
				totalWords += len(strings.Fields(h.Content))
			}
			if contextData != nil {
				totalWords += len(strings.Fields(string(contextData)))
			}
			tokenEstimate = int(float64(totalWords) / 0.75)
			contextWindow = getModelContextWindow(agent.Model)
		}
	} else {
		for _, h := range history {
			words := len(strings.Fields(h.Content))
			totalWords += words
		}

		if contextData != nil {
			contextWords := len(strings.Fields(string(contextData)))
			totalWords += contextWords
		}

		tokenEstimate = int(float64(totalWords) / 0.75)
		contextWindow = getModelContextWindow(agent.Model)
	}

	estimatedCost := calculateTokenCost(tokenEstimate, agent.Provider, agent.Model)

	stats := AgentStats{
		WordCount:     totalWords,
		TokenEstimate: tokenEstimate,
		ContextWindow: contextWindow,
		HistoryCount:  len(history),
		EstimatedCost: estimatedCost,
		LLMAPICalls:   agent.LLMAPICalls,
	}

	return c.JSON(stats)
}

// HandleGetAgentContextWindow returns the full context window for an agent.
func (s *Server) HandleGetAgentContextWindow(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent id"})
	}

	agent, err := s.db.GetAgent(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "agent not found"})
	}

	history, _ := s.db.GetHistory(id, 500)

	// Build system prompt like the chat handler does - ALWAYS use global context.md
	systemPrompt := agent.Personality

	// Load global context.md from HermitShell root directory
	if content, err := os.ReadFile("./context.md"); err == nil {
		contextStr := strings.TrimSpace(string(content))
		if contextStr != "" {
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_NAME}}", agent.Name)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_ROLE}}", agent.Role)
			contextStr = strings.ReplaceAll(contextStr, "{{AGENT_PERSONALITY}}", agent.Personality)
			systemPrompt = contextStr
		}
	}

	var historyList []map[string]interface{}
	for _, h := range history {
		historyList = append(historyList, map[string]interface{}{
			"role":      h.Role,
			"content":   h.Content,
			"timestamp": h.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"systemPrompt":     systemPrompt, // Full system prompt with variables substituted
		"agentPersonality": agent.Personality,
		"history":          historyList,
	})
}

// HandleGetLastMessage returns the last message from an agent.
func (s *Server) HandleGetLastMessage(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent id"})
	}

	history, err := s.db.GetHistory(id, 1)
	if err != nil || len(history) == 0 {
		return c.JSON(fiber.Map{"content": ""})
	}

	lastMessage := history[len(history)-1]
	return c.JSON(fiber.Map{"content": lastMessage.Content})
}

// HandleGetUnreadCount returns the unread message count for an agent.
func (s *Server) HandleGetUnreadCount(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent id"})
	}

	count, err := s.db.GetUnseenCount(id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"unread": count})
}

// HandleMarkMessagesSeen marks all messages for an agent as seen.
func (s *Server) HandleMarkMessagesSeen(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent id"})
	}

	if err := s.db.MarkHistorySeen(id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleListSkills(c *fiber.Ctx) error {
	skills, err := s.db.ListSkills()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	type SkillResponse struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		IsCore      bool   `json:"isCore"`
	}

	var result []SkillResponse
	for _, s := range skills {
		result = append(result, SkillResponse{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			Content:     s.Content,
			IsCore:      s.ID == 1,
		})
	}

	return c.JSON(result)
}

func (s *Server) HandleCreateSkill(c *fiber.Ctx) error {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	c.BodyParser(&req)

	skill := &db.Skill{
		Title:       req.Title,
		Description: req.Description,
		Content:     req.Content,
	}
	id, err := s.db.CreateSkill(skill)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id, "success": true})
}

func (s *Server) HandleUpdateSkill(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	c.BodyParser(&req)

	skills, _ := s.db.ListSkills()
	for _, s := range skills {
		if s.ID == id {
			s.Title = req.Title
			s.Description = req.Description
			s.Content = req.Content
			break
		}
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleDeleteSkill(c *fiber.Ctx) error {
	skillIDStr := c.Params("skillId")
	if skillIDStr == "" {
		skillIDStr = c.Params("id")
	}
	id, _ := strconv.ParseInt(skillIDStr, 10, 64)
	if id == 1 {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot delete core context skill"})
	}
	s.db.DeleteSkill(id)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleGetContextSkill(c *fiber.Ctx) error {
	dataDir, _ := s.db.GetSetting("data_dir")
	if dataDir == "" {
		dataDir = "./data"
	}
	contextPath := filepath.Join(dataDir, "skills", "context.md")

	content, err := os.ReadFile(contextPath)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Context file not found"})
	}

	return c.JSON(fiber.Map{"content": string(content)})
}

func (s *Server) HandleResetContextSkill(c *fiber.Ctx) error {
	dataDir, _ := s.db.GetSetting("data_dir")
	if dataDir == "" {
		dataDir = "./data"
	}
	contextPath := filepath.Join(dataDir, "skills", "context.md")

	defaultContent, err := os.ReadFile("./context.md")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	os.WriteFile(contextPath, defaultContent, 0644)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleListCalendar(c *fiber.Ctx) error {
	events, err := s.db.ListCalendarEvents()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	agents, err := s.db.ListAgents()
	if err != nil {
		agents = []*db.Agent{}
	}

	agentMap := make(map[int64]*db.Agent)
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	type CalendarResponse struct {
		ID        int64  `json:"id"`
		AgentID   int64  `json:"agentId"`
		AgentName string `json:"agentName"`
		AgentPic  string `json:"agentPic"`
		Date      string `json:"date"`
		Time      string `json:"time"`
		Prompt    string `json:"prompt"`
		Executed  bool   `json:"executed"`
		CreatedAt string `json:"createdAt"`
	}

	var result []CalendarResponse
	for _, e := range events {
		agentName := ""
		agentPic := ""
		if agent, ok := agentMap[e.AgentID]; ok {
			agentName = agent.Name
			agentPic = agent.ProfilePic
		}
		result = append(result, CalendarResponse{
			ID:        e.ID,
			AgentID:   e.AgentID,
			AgentName: agentName,
			AgentPic:  agentPic,
			Date:      e.Date,
			Time:      e.Time,
			Prompt:    e.Prompt,
			Executed:  e.Executed,
			CreatedAt: e.CreatedAt,
		})
	}

	return c.JSON(result)
}

func (s *Server) HandleCreateCalendarEvent(c *fiber.Ctx) error {
	var req struct {
		AgentID int64  `json:"agentId"`
		Date    string `json:"date"`
		Time    string `json:"time"`
		Prompt  string `json:"prompt"`
	}
	c.BodyParser(&req)

	event := &db.CalendarEvent{
		AgentID: req.AgentID,
		Date:    req.Date,
		Time:    req.Time,
		Prompt:  req.Prompt,
	}
	id, err := s.db.CreateCalendarEvent(event)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id, "success": true})
}

func (s *Server) HandleDeleteCalendarEvent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	s.db.DeleteCalendarEvent(id)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleUpdateCalendarEvent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	var req struct {
		AgentID int64  `json:"agentId"`
		Date    string `json:"date"`
		Time    string `json:"time"`
		Prompt  string `json:"prompt"`
	}
	c.BodyParser(&req)

	err := s.db.UpdateCalendarEvent(id, req.AgentID, req.Date, req.Time, req.Prompt)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleListAllowlist(c *fiber.Ctx) error {
	entries, err := s.db.ListAllowList()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	type AllowlistResponse struct {
		ID             int64  `json:"id"`
		TelegramUserID string `json:"telegramUserId"`
		FriendlyName   string `json:"friendlyName"`
		Notes          string `json:"notes"`
		CreatedAt      string `json:"createdAt"`
	}

	var result []AllowlistResponse
	for _, e := range entries {
		result = append(result, AllowlistResponse{
			ID:             e.ID,
			TelegramUserID: e.TelegramUserID,
			FriendlyName:   e.FriendlyName,
			Notes:          e.Notes,
			CreatedAt:      e.CreatedAt,
		})
	}

	return c.JSON(result)
}

func (s *Server) HandleCreateAllowlist(c *fiber.Ctx) error {
	var req struct {
		TelegramUserID string `json:"telegramUserId"`
		FriendlyName   string `json:"friendlyName"`
		Notes          string `json:"notes"`
	}
	c.BodyParser(&req)

	entry := &db.AllowListEntry{
		TelegramUserID: req.TelegramUserID,
		FriendlyName:   req.FriendlyName,
		Notes:          req.Notes,
	}
	id, err := s.db.CreateAllowListEntry(entry)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id, "success": true})
}

func (s *Server) HandleDeleteAllowlist(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	s.db.DeleteAllowListEntry(id)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleGetSettings(c *fiber.Ctx) error {
	tunnelEnabled, _ := s.db.GetSetting("tunnel_enabled")
	if tunnelEnabled == "" {
		tunnelEnabled = "true"
	}
	openrouterKey, _ := s.db.GetSetting("openrouter_api_key")
	openaiKey, _ := s.db.GetSetting("openai_api_key")
	anthropicKey, _ := s.db.GetSetting("anthropic_api_key")
	geminiKey, _ := s.db.GetSetting("gemini_api_key")
	timezone, _ := s.db.GetSetting("timezone")
	timeOffset, _ := s.db.GetSetting("time_offset")

	tunnelURL := s.tunnels.GetURL("dashboard")
	isHealthy := s.tunnels.CheckTunnelHealth("dashboard", 2*time.Second)

	if tunnelEnabled == "true" && tunnelURL == "" {
		port, _ := strconv.Atoi(os.Getenv("PORT"))
		if port == 0 {
			port = 3000
		}
		go s.tunnels.StartQuickTunnel("dashboard", port)
	}

	status := "Disabled"
	if tunnelEnabled == "true" {
		if isHealthy {
			status = "Active (Quick Tunnel)"
		} else {
			status = "Provisioning..."
		}
	}

	// Return actual key values so client can populate the settings form
	return c.JSON(fiber.Map{
		"tunnelEnabled": tunnelEnabled == "true",
		"tunnelURL":     tunnelURL,
		"tunnelHealthy": isHealthy,
		"status":        status,
		"openrouterKey": openrouterKey,
		"openaiKey":     openaiKey,
		"anthropicKey":  anthropicKey,
		"geminiKey":     geminiKey,
		"timezone":      timezone,
		"timeOffset":    timeOffset,
		"hasLLMKey":     openrouterKey != "" || openaiKey != "" || anthropicKey != "" || geminiKey != "",
	})
}

func (s *Server) HandleGetTunnelURL(c *fiber.Ctx) error {
	tunnelEnabled, _ := s.db.GetSetting("tunnel_enabled")
	if tunnelEnabled == "false" {
		return c.JSON(fiber.Map{
			"url":     "",
			"healthy": false,
		})
	}

	tunnelURL := s.tunnels.GetURL("dashboard")

	if tunnelURL == "" && s.tunnels.IsRunning("dashboard") {
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			tunnelURL = s.tunnels.GetURL("dashboard")
			if tunnelURL != "" {
				break
			}
		}
	}

	if tunnelURL == "" {
		port, _ := strconv.Atoi(os.Getenv("PORT"))
		if port == 0 {
			port = 3000
		}
		newURL, err := s.tunnels.StartQuickTunnel("dashboard", port)
		if err == nil {
			tunnelURL = newURL
		}
	}

	isHealthy := s.tunnels.CheckTunnelHealth("dashboard", 2*time.Second)

	return c.JSON(fiber.Map{
		"url":     tunnelURL,
		"healthy": isHealthy,
	})
}

func (s *Server) HandleSetSettings(c *fiber.Ctx) error {
	var req struct {
		TunnelEnabled *bool   `json:"tunnelEnabled"`
		OpenrouterKey *string `json:"openrouterKey"`
		OpenaiKey     *string `json:"openaiKey"`
		AnthropicKey  *string `json:"anthropicKey"`
		GeminiKey     *string `json:"geminiKey"`
		Timezone      *string `json:"timezone"`
		TimeOffset    *string `json:"timeOffset"`
	}
	c.BodyParser(&req)

	if req.TunnelEnabled != nil {
		if *req.TunnelEnabled {
			s.db.SetSetting("tunnel_enabled", "true")
		} else {
			s.db.SetSetting("tunnel_enabled", "false")
			if s.tunnels != nil {
				s.tunnels.StopTunnel("dashboard")
			}
		}
	}

	// Persist API keys exactly as submitted so /status and chat validation use the real saved state.
	// Reference: docs/frontend-backend-communication.md.
	if req.OpenrouterKey != nil {
		s.db.SetSetting("openrouter_api_key", strings.TrimSpace(*req.OpenrouterKey))
	}
	if req.OpenaiKey != nil {
		s.db.SetSetting("openai_api_key", strings.TrimSpace(*req.OpenaiKey))
	}
	if req.AnthropicKey != nil {
		s.db.SetSetting("anthropic_api_key", strings.TrimSpace(*req.AnthropicKey))
	}
	if req.GeminiKey != nil {
		s.db.SetSetting("gemini_api_key", strings.TrimSpace(*req.GeminiKey))
	}
	if req.Timezone != nil {
		s.db.SetSetting("timezone", strings.TrimSpace(*req.Timezone))
	}
	if req.TimeOffset != nil {
		s.db.SetSetting("time_offset", strings.TrimSpace(*req.TimeOffset))
	}

	return c.JSON(fiber.Map{"success": true})
}

// HandleGetTime returns the current time with user's offset applied.
// Docs: See docs/time-management.md for how offset is calculated and applied.
// Purpose: Allows displaying time in user's desired timezone regardless of server location.
func (s *Server) HandleGetTime(c *fiber.Ctx) error {
	timezone, _ := s.db.GetSetting("timezone")
	timeOffset, _ := s.db.GetSetting("time_offset")

	// Get current system time (already has offset applied via db.GetSystemTime)
	systemTime := s.db.GetSystemTime()
	// Get raw UTC time so clients can calculate their own offsets/previews accurately
	rawUtcTime := time.Now().UTC()

	return c.JSON(fiber.Map{
		"time":          systemTime.Format("03:04:05 PM"),
		"time12":        systemTime.Format("3:04 PM"),
		"date":          systemTime.Format("Mon, Jan 2"),
		"fullDate":      systemTime.Format("2006-01-02"),
		"datetime":      systemTime.Format(time.RFC3339),
		"systemTime":    systemTime.Format(time.RFC3339),
		"serverUtcTime": rawUtcTime.Format(time.RFC3339),
		"timezone":      timezone,
		"offset":        timeOffset,
	})
}

func (s *Server) HandleTelegramSendCode(c *fiber.Ctx) error {
	var req struct {
		Token  string `json:"token"`
		UserID string `json:"userId"`
	}
	c.BodyParser(&req)

	code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	s.verifyCodes[req.Token] = code
	s.verifyTokens[code] = req.Token

	tempBot := telegram.NewBot(req.Token)
	tempBot.SendMessage(req.UserID, "Your Hermit Dashboard Verification Code is: "+code)

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleTelegramVerify(c *fiber.Ctx) error {
	var req struct {
		Token  string `json:"token"`
		Code   string `json:"code"`
		UserID string `json:"userId"`
	}
	c.BodyParser(&req)

	if expected, ok := s.verifyCodes[req.Token]; ok && expected == req.Code {
		tempBot := telegram.NewBot(req.Token)
		tempBot.SendMessage(req.UserID, "Successfully connected this Telegram Bot to Hermit Agent OS.")
		delete(s.verifyCodes, req.Token)
		delete(s.verifyTokens, req.Code)
		return c.JSON(fiber.Map{"success": true})
	}
	return c.JSON(fiber.Map{"success": false, "error": "Invalid verification code."})
}

func (s *Server) HandleTestContract(c *fiber.Ctx) error {
	var req struct {
		Payload  string `json:"payload"`
		UserID   string `json:"userId"`
		AgentID  int64  `json:"agentId"`
		Platform string `json:"platform"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	log.Printf("[TEST CONTRACT] Received payload: %s", req.Payload)

	agent, _ := s.db.GetAgent(req.AgentID)
	if agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}
	if err := s.ensureConsoleTestAssets(agent); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to seed test assets: " + err.Error()})
	}

	platform := strings.TrimSpace(strings.ToLower(req.Platform))
	if platform == "" {
		platform = "telegram"
	}

	var feedback []map[string]interface{}
	switch platform {
	case "telegram":
		var agentBot *telegram.Bot
		if agent.TelegramToken != "" {
			agentBot = telegram.NewBot(agent.TelegramToken)
		}
		feedback = s.ExecuteXMLPayload(req.AgentID, req.UserID, req.Payload, agentBot)
		if req.UserID != "" && req.UserID != "test" && req.UserID != "test-user" {
			s.addHistoryAndBroadcast(agent.ID, "assistant", "assistant", req.Payload)
		}
	case "hermitchat":
		feedback = s.ExecuteXMLPayload(req.AgentID, "mobile-chat", req.Payload, nil)
		parsed := parser.ParseLLMOutput(req.Payload)
		files := []string{}
		for _, action := range parsed.Actions {
			if action.Type == "GIVE" {
				files = append(files, action.Value)
			}
		}
		if parsed.Message != "" || len(files) > 0 {
			s.addHistoryAndBroadcastWithFiles(agent.ID, "test-console", "assistant", parsed.Message, files)
		}
	default:
		return c.Status(400).JSON(fiber.Map{"error": "Unsupported platform"})
	}

	log.Printf("[TEST CONTRACT] Feedback: %v", feedback)

	return c.JSON(fiber.Map{
		"actionEffects": feedback,
	})
}

func (s *Server) ensureAgentContainer(agent *db.Agent) (string, error) {
	if agent == nil {
		return "hermit-test", nil
	}
	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
		agent.ContainerID = containerName
		s.db.UpdateAgent(agent)
	}

	if s.docker != nil {
		isRunning := s.docker.IsRunning(containerName)
		if !isRunning {
			log.Printf("Container %s not running for agent %s, attempting to start/create...", containerName, agent.Name)
			image, _ := s.db.GetSetting("default_agent_image")
			if image == "" {
				image = "hermit-agent:latest"
			}
			err := s.docker.Run(containerName, image, true)
			if err != nil {
				log.Printf("Failed to ensure container for agent %s: %v", agent.Name, err)
				s.db.LogAction(agent.ID, "docker", "container_start_failed", err.Error())
				return containerName, fmt.Errorf("failed to start container: %v", err)
			}
			s.db.LogAction(agent.ID, "docker", "container_started", "Container started (was stopped or recreated)")
			agent.Status = "running"
			s.db.UpdateAgent(agent)
			time.Sleep(1 * time.Second)
		}
	} else if agent.Status != "running" {
		agent.Status = "running"
		s.db.UpdateAgent(agent)
	}
	return containerName, nil
}

// ProcessTelegramUpdate processes a Telegram update from polling.
// This method decouples the message processing logic from the transport mechanism.
// Reference: See docs/telegram-integration.md for polling architecture.
func (s *Server) ProcessTelegramUpdate(agent *db.Agent, update telegram.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
	userText := strings.TrimSpace(update.Message.Text)
	userID := fmt.Sprintf("%d", update.Message.From.ID)

	// Log incoming message
	log.Printf("[Telegram] Agent=%s, From=%s(@%s), Message=%s", agent.Name, userID, update.Message.From.Username, userText)
	s.db.LogAction(agent.ID, "agent", "telegram_received", fmt.Sprintf("From: %s(@%s), Message: %s", userID, update.Message.From.Username, userText))

	// Authorization check
	allowed := false
	authReason := ""
	if agent.AllowedUsers == "" {
		allowed = true
		authReason = "no allowed_users set"
	} else {
		allowedUsers := strings.Split(agent.AllowedUsers, ",")
		for _, u := range allowedUsers {
			trimmed := strings.TrimSpace(u)
			if trimmed == userID {
				allowed = true
				authReason = "matched userID: " + userID
				break
			}
			if trimmed == update.Message.From.Username {
				allowed = true
				authReason = "matched username: " + update.Message.From.Username
				break
			}
		}
		if !allowed {
			authReason = fmt.Sprintf("userID=%s, username=%s, allowedUsers=%s", userID, update.Message.From.Username, agent.AllowedUsers)
		}
	}

	// Log the authorization attempt
	s.db.LogAction(agent.ID, "agent", "telegram_message", fmt.Sprintf("From: %s (ID: %s), Allowed: %v, Reason: %s", update.Message.From.Username, userID, allowed, authReason))

	if !allowed {
		tempBot := telegram.NewBot(agent.TelegramToken)
		tempBot.SendMessage(chatID, fmt.Sprintf("You are not authorized to use this agent.\n\nDebug info:\n- Your ID: %s\n- Your username: @%s\n- Allowed users: %s", userID, update.Message.From.Username, agent.AllowedUsers))
		return
	}

	// Handle Commands
	if strings.HasPrefix(userText, "/") {
		s.handleAgentCommand(agent, chatID, userText)
		return
	}

	// Log user message
	s.addHistoryAndBroadcast(agent.ID, userID, "user", userText)

	// Ensure container is running before processing AI request
	if _, err := s.ensureAgentContainer(agent); err != nil {
		log.Printf("Failed to ensure container for agent %s: %v", agent.Name, err)
	}

	s.mu.RLock()
	takeoverOn := s.takeoverMode[chatID]
	s.mu.RUnlock()

	if takeoverOn {
		tempBot := telegram.NewBot(agent.TelegramToken)
		s.handleTakeoverInput(agent.ID, chatID, userText, tempBot)
	} else {
		go s.processAgentAIRequest(agent, chatID, userID, userText)
	}
}

// StartAgentPoller starts a long-polling goroutine for a specific agent.
// It runs in a loop, fetching updates from Telegram and processing them.
// The poller stops when the context is cancelled.
// Reference: See docs/telegram-integration.md for polling architecture.
func (s *Server) StartAgentPoller(ctx context.Context, agent *db.Agent) {
	bot := telegram.NewBot(agent.TelegramToken)

	// Clear any existing webhook before polling
	if err := bot.DeleteWebhook(); err != nil {
		log.Printf("Agent %s: Failed to delete webhook: %v", agent.Name, err)
	}

	var offset int64 = 0
	log.Printf("Agent %s: Starting Telegram poller (offset=%d)", agent.Name, offset)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Agent %s: Stopping Telegram poller", agent.Name)
			return
		default:
			// 30 second timeout for long polling
			updates, err := bot.GetUpdates(offset, 30)
			if err != nil {
				log.Printf("Agent %s: Polling error: %v", agent.Name, err)
				time.Sleep(5 * time.Second) // backoff on error
				continue
			}

			for _, update := range updates {
				if update.UpdateID >= offset {
					offset = update.UpdateID + 1 // Advance offset to acknowledge
				}
				// Process update concurrently so polling doesn't block
				go s.ProcessTelegramUpdate(agent, update)
			}
		}
	}
}

// StartPollingForAgent starts a new polling goroutine for an agent.
// If a poller already exists for this agent, it will be stopped first.
// Reference: See docs/telegram-integration.md for polling architecture.
func (s *Server) StartPollingForAgent(agent *db.Agent) {
	if agent.TelegramToken == "" {
		log.Printf("Agent %s: No Telegram token, skipping poller start", agent.Name)
		return
	}

	// Stop existing poller if any
	s.StopPollingForAgent(agent.ID)

	ctx, cancel := context.WithCancel(context.Background())

	s.pollersMu.Lock()
	s.pollers[agent.ID] = cancel
	s.pollersMu.Unlock()

	go s.StartAgentPoller(ctx, agent)
	log.Printf("Agent %s: Poller started", agent.Name)
}

// StopPollingForAgent stops the polling goroutine for an agent.
// Reference: See docs/telegram-integration.md for polling architecture.
func (s *Server) StopPollingForAgent(agentID int64) {
	s.pollersMu.Lock()
	defer s.pollersMu.Unlock()

	if cancel, exists := s.pollers[agentID]; exists {
		cancel()
		delete(s.pollers, agentID)
		log.Printf("Agent ID %d: Poller stopped", agentID)
	}
}

// StopAllPollers stops all active polling goroutines.
// Reference: See docs/telegram-integration.md for polling architecture.
func (s *Server) StopAllPollers() {
	s.pollersMu.Lock()
	defer s.pollersMu.Unlock()

	for agentID, cancel := range s.pollers {
		cancel()
		log.Printf("Agent ID %d: Poller stopped", agentID)
	}
	s.pollers = make(map[int64]context.CancelFunc)
}

func (s *Server) handleAgentCommand(agent *db.Agent, chatID, text string) error {
	bot := telegram.NewBot(agent.TelegramToken)
	cmd := strings.Split(text, " ")[0]

	switch cmd {
	case "/status":
		config := s.getLLMConfigStatus(agent)
		statusMsg := fmt.Sprintf("🤖 *Agent Status: %s*\n\n", agent.Name)
		statusMsg += fmt.Sprintf("• Provider: `%s`\n", config.ProviderUI)
		statusMsg += fmt.Sprintf("• Model: `%s`\n", firstNonEmpty(config.Model, "Not set"))
		statusMsg += fmt.Sprintf("• Model Type: `%s`\n", config.ModelType)
		statusMsg += fmt.Sprintf("• API Key: `%s`\n", ternary(config.APIKeySet, "Configured", "Missing"))
		statusMsg += fmt.Sprintf("• LLM Ready: `%s`\n", ternary(config.Configured, "Yes", "No"))
		if !config.Configured {
			statusMsg += fmt.Sprintf("• Missing: `%s`\n", config.missingSummary())
		}
		statusMsg += fmt.Sprintf("• Context Window: `%d` tokens\n", agent.ContextWindow)
		statusMsg += fmt.Sprintf("• LLM API Calls: `%d`\n", agent.LLMAPICalls)

		containerStatus := "Stopped"
		if agent.ContainerID != "" && s.docker != nil {
			if s.docker.IsRunning(agent.ContainerID) {
				containerStatus = "Running ✅"
			} else {
				containerStatus = "Stopped ❌"
			}
		}
		statusMsg += fmt.Sprintf("• Container: `%s` (%s)\n", agent.ContainerID, containerStatus)

		statusMsg += "• Connection: Long Polling Active ✅\n"

		statusMsg += fmt.Sprintf("\n🔐 *Authorization*\n")
		statusMsg += fmt.Sprintf("• Allowed Users: `%s`\n", agent.AllowedUsers)
		statusMsg += fmt.Sprintf("• Your User ID: `%s`\n", chatID)
		if agent.AllowedUsers == "" {
			statusMsg += "• Status: ✅ No restrictions\n"
		} else {
			statusMsg += "• Status: ⚠️ Restricted\n"
		}

		tunnelURL := ""
		if s.tunnels != nil {
			tunnelURL = s.tunnels.GetURL("dashboard")
		}
		if tunnelURL != "" {
			statusMsg += fmt.Sprintf("\n🌐 *Dashboard*: `%s`\n", tunnelURL)
		}

		bot.SendMessage(chatID, statusMsg)

	case "/help":
		helpMsg := "🤖 *HermitShell Agent Commands*\n\n"
		helpMsg += "• /status - Show configuration & health\n"
		helpMsg += "• /clear - Wipe chat context\n"
		helpMsg += "• /reset - Restart container\n"
		helpMsg += "• /takeover - Toggle manual control\n"
		helpMsg += "• /give_system_prompt - Get persona\n"
		helpMsg += "• /give_context - Get full history"
		bot.SendMessage(chatID, helpMsg)

	case "/clear":
		s.db.ClearHistory(agent.ID)
		bot.SendMessage(chatID, "🧹 Context window and chat history cleared!")

	case "/reset":
		bot.SendMessage(chatID, "🔄 Container reset initiated...")
		if agent.ContainerID != "" && s.docker != nil {
			s.docker.Stop(agent.ContainerID)
			s.docker.Remove(agent.ContainerID)
			// Status will be updated by monitor
		}
		bot.SendMessage(chatID, "✅ Container has been reset. Fresh environment ready.")

	case "/takeover":
		s.mu.Lock()
		active := s.takeoverMode[chatID]
		s.takeoverMode[chatID] = !active
		newState := s.takeoverMode[chatID]
		s.mu.Unlock()

		if newState {
			bot.SendMessage(chatID, "🟢 *TAKEOVER MODE ENABLED*\nXML commands will be parsed directly. LLM is paused.")
		} else {
			bot.SendMessage(chatID, "🔴 *TAKEOVER MODE DISABLED*\nControl returned to LLM.")
		}

	case "/give_system_prompt":
		fileName := fmt.Sprintf("%s_personality.txt", agent.Name)
		os.WriteFile(fileName, []byte(agent.Personality), 0644)
		bot.SendDocument(chatID, fileName, "Agent Personality / System Prompt")
		os.Remove(fileName)

	case "/give_context":
		history, _ := s.db.GetHistory(agent.ID, 50)
		var sb strings.Builder
		for i := len(history) - 1; i >= 0; i-- {
			h := history[i]
			sb.WriteString(fmt.Sprintf("[%s] %s: %s\n\n", h.CreatedAt, h.Role, h.Content))
		}
		fileName := fmt.Sprintf("%s_context.txt", agent.Name)
		os.WriteFile(fileName, []byte(sb.String()), 0644)
		bot.SendDocument(chatID, fileName, "Full Conversation Context")
		os.Remove(fileName)

	default:
		bot.SendMessage(chatID, "Unknown command. Use /help (if implemented) or check the manual.")
	}

	return nil
}

func (s *Server) processAgentAIRequest(agent *db.Agent, chatID, userID, userText string) {
	tempBot := telegram.NewBot(agent.TelegramToken)

	// Inject current system time into human-friendly user message for AI context
	currentTime := s.db.GetSystemTime()
	formattedTime := currentTime.Format("2006-01-02 15:04:05")
	userTextWithTime := fmt.Sprintf("[Current System Time: %s] %s", formattedTime, userText)

	// Send "thinking" message first
	thinkingMsgID, _ := tempBot.SendMessageWithID(chatID, "🤔 Thinking...")
	if thinkingMsgID != "" {
		log.Printf("[%s] Sent thinking message (ID: %s)", agent.Name, thinkingMsgID)
	}

	// Log AI processing start
	s.db.LogAction(agent.ID, "agent", "ai_processing", fmt.Sprintf("Processing message from user %s", userID))

	// Fetch history for context
	history, _ := s.db.GetHistory(agent.ID, 10)
	var messages []llm.Message

	// System prompt: prepend context.md instructions to agent personality
	systemPrompt := agent.Personality
	contextPath := "./context.md"
	if content, err := os.ReadFile(contextPath); err == nil {
		contextStr := string(content)
		contextStr = strings.ReplaceAll(contextStr, "{{AGENT_NAME}}", agent.Name)
		contextStr = strings.ReplaceAll(contextStr, "{{AGENT_ROLE}}", agent.Role)
		contextStr = strings.ReplaceAll(contextStr, "{{AGENT_PERSONALITY}}", agent.Personality)
		systemPrompt = contextStr + "\n\n---\n\n" + agent.Personality
	}

	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})

	// Add user message with injected time
	messages = append(messages, llm.Message{Role: "user", Content: userTextWithTime})
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		role := h.Role
		if role == "system" {
			role = "user" // Simple mapping for now
		}
		messages = append(messages, llm.Message{Role: role, Content: h.Content})
	}

	// Get LLM Client
	client := s.getLLMClientForAgent(agent)
	if client == nil {
		tempBot.SendMessage(chatID, "Error: LLM client not configured for this agent.")
		if thinkingMsgID != "" {
			tempBot.DeleteMessage(chatID, thinkingMsgID)
		}
		s.addHistoryAndBroadcast(agent.ID, "system", "system", "Error: LLM client not configured")
		s.db.LogAction(agent.ID, "agent", "llm_error", "LLM client not configured")
		return
	}

	// Log LLM API request (network)
	s.db.LogAction(agent.ID, "network", "llm_request", fmt.Sprintf("Provider: %s, Model: %s, Messages: %d", agent.Provider, agent.Model, len(messages)))

	// Chat
	response, err := client.Chat(agent.Model, messages)

	// Increment API call counter and update context window
	s.db.IncrementLLMAPICalls(agent.ID)
	contextWindow := getModelContextWindow(agent.Model)
	s.db.UpdateAgentContextWindow(agent.ID, contextWindow)

	if err != nil {
		tempBot.SendMessage(chatID, "Error communicating with AI: "+err.Error())
		if thinkingMsgID != "" {
			tempBot.DeleteMessage(chatID, thinkingMsgID)
		}
		s.addHistoryAndBroadcast(agent.ID, "system", "system", "LLM Error: "+err.Error())
		s.db.LogAction(agent.ID, "agent", "llm_error", fmt.Sprintf("Error: %v", err))
		return
	}

	// Parse the LLM response to extract message content
	parsed := parser.ParseLLMOutput(response)

	// Log LLM response
	s.db.LogAction(agent.ID, "agent", "llm_response", fmt.Sprintf("Response: %.200s...", response))

	// Delete thinking message
	if thinkingMsgID != "" {
		tempBot.DeleteMessage(chatID, thinkingMsgID)
	}

	// Only broadcast the parsed message content, not raw response
	// This ensures <thought> tags are kept internal and only <message> content goes to users
	s.addHistoryAndBroadcast(agent.ID, "assistant", "assistant", parsed.Message)

	// Execute the XML actions from the response (terminal, give, calendar, etc.)
	feedback := s.ExecuteXMLPayload(agent.ID, chatID, response, tempBot)

	// Commit with <end> and feedback
	if len(feedback) > 0 {
		feedbackJSON, _ := json.Marshal(feedback)
		s.addHistoryAndBroadcast(agent.ID, "system", "system", string(feedbackJSON)+"\n<end>")
	}
}

func (s *Server) getLLMClientForAgent(agent *db.Agent) *llm.Client {
	provider := agent.Provider
	if provider == "" {
		provider = "openrouter"
	}

	var apiKey string
	var baseURL string
	switch provider {
	case "openai":
		apiKey, _ = s.db.GetSetting("openai_api_key")
		baseURL = "https://api.openai.com/v1"
	case "anthropic":
		apiKey, _ = s.db.GetSetting("anthropic_api_key")
		baseURL = "https://api.anthropic.com/v1"
	case "gemini":
		apiKey, _ = s.db.GetSetting("gemini_api_key")
		baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		apiKey, _ = s.db.GetSetting("openrouter_api_key")
		baseURL = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		return nil
	}

	return llm.NewClient(
		llm.WithProvider(llm.Provider(provider)),
		llm.WithBaseURL(baseURL),
		llm.WithAPIKey(apiKey),
		llm.WithModel(agent.Model),
	)
}

func (s *Server) handleTelegramCommand(chatID, text string) {
	if s.bot == nil {
		return
	}

	switch text {
	case "/start":
		s.bot.SendMessage(chatID, "Welcome to Hermit Agent OS! Use /help to see available commands.")
	case "/help":
		helpMsg := `Available commands:
/start - Welcome message
/help - Show this help
/clear - Clear context window
/tokens - Show context size
/reset - Reset container
/takeover - Toggle takeover mode
/give_system_prompt - Get agent system prompt
/give_context - Get current context`
		s.bot.SendMessage(chatID, helpMsg)

	case "/clear":
		s.mu.Lock()
		s.contextStore[chatID] = []string{}
		s.tokenCounters[chatID] = 0
		s.mu.Unlock()
		s.bot.SendMessage(chatID, "Context window cleared!")

	case "/tokens":
		s.mu.RLock()
		count := s.tokenCounters[chatID]
		s.mu.RUnlock()
		s.bot.SendMessage(chatID, fmt.Sprintf("Current context size: ~%d tokens", count))

	case "/reset":
		s.bot.SendMessage(chatID, "Container reset initiated...")
		s.bot.SendMessage(chatID, "Container has been reset with fresh state.")

	case "/takeover":
		s.mu.Lock()
		currentlyOn := s.takeoverMode[chatID]
		s.takeoverMode[chatID] = !currentlyOn
		newState := s.takeoverMode[chatID]
		s.mu.Unlock()
		if newState {
			s.bot.SendMessage(chatID, "Takeover mode ENABLED. You can now send XML commands directly.\n\nExample:\n<terminal>ls -la</terminal>\n<action type=\"GIVE\">file.txt</action>\n\nUse /takeover again to disable.")
		} else {
			s.bot.SendMessage(chatID, "Takeover mode DISABLED. Returning to AI agent control.")
		}

	case "/give_system_prompt":
		dataDir, _ := s.db.GetSetting("data_dir")
		if dataDir == "" {
			dataDir = "./data"
		}
		contextPath := filepath.Join(dataDir, "skills", "context.md")
		content, err := os.ReadFile(contextPath)
		if err != nil {
			s.bot.SendMessage(chatID, "Error reading system prompt")
			return
		}
		s.bot.SendMessage(chatID, "System Prompt:\n\n"+string(content))

	case "/give_context":
		s.mu.RLock()
		context := s.contextStore[chatID]
		s.mu.RUnlock()
		if len(context) == 0 {
			s.bot.SendMessage(chatID, "Context is empty.")
		} else {
			fullContext := strings.Join(context, "\n")
			if len(fullContext) > 4000 {
				fullContext = fullContext[:4000] + "...\n(context truncated)"
			}
			s.bot.SendMessage(chatID, "Current context:\n\n"+fullContext)
		}

	default:
		s.mu.RLock()
		takeoverOn := s.takeoverMode[chatID]
		s.mu.RUnlock()

		if takeoverOn {
			s.handleTakeoverInput(0, chatID, text, s.bot)
		} else {
			s.bot.SendMessage(chatID, "Message received. AI agent will respond shortly...")
		}
	}
}

func (s *Server) HandleListApps(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	agent, err := s.db.GetAgent(id)
	if err != nil || agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	containerName := agent.ContainerID
	if containerName == "" {
		containerName = "agent-" + strings.ToLower(agent.Name)
	}

	appsDir := "/app/workspace/apps"
	files, err := s.docker.ListContainerFiles(containerName, appsDir)
	if err != nil {
		return c.JSON([]interface{}{})
	}

	var apps []fiber.Map
	for _, f := range files {
		if f.IsDir {
			apps = append(apps, fiber.Map{
				"name": f.Name,
				"url":  fmt.Sprintf("/apps/%d/%s", id, f.Name),
			})
		}
	}
	return c.JSON(apps)
}

func (s *Server) HandleGetAllApps(c *fiber.Ctx) error {
	agents, err := s.db.ListAgents()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve agents"})
	}

	var allApps []fiber.Map
	appsDir := "/app/workspace/apps"

	for _, agent := range agents {
		if agent.Status != "running" || !agent.Active {
			continue
		}

		containerName := agent.ContainerID
		if containerName == "" {
			containerName = "agent-" + strings.ToLower(agent.Name)
		}

		files, err := s.docker.ListContainerFiles(containerName, appsDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir {
				allApps = append(allApps, fiber.Map{
					"agentId":     agent.ID,
					"agentName":   agent.Name,
					"profilePic":  agent.ProfilePic,
					"containerId": containerName,
					"name":        f.Name,
					"url":         fmt.Sprintf("/apps/%d/%s", agent.ID, f.Name),
				})
			}
		}
	}

	if allApps == nil {
		allApps = []fiber.Map{}
	}

	return c.JSON(allApps)
}

// ExecuteXMLPayload processes parsed XML tags from LLM response.
// Docs: See docs/xml-tags.md for all supported tags.
// Handles: <message>, <terminal>, <give>, <app>, <skill>, <calendar>, <thought>, <system>
func (s *Server) ExecuteXMLPayload(agentID int64, chatID, xmlInput string, bot *telegram.Bot) []map[string]interface{} {
	parsed := parser.ParseLLMOutput(xmlInput)

	// Debug logging
	if len(parsed.Calendars) > 0 {
		log.Printf("[DEBUG] Found %d calendar events in parsed response", len(parsed.Calendars))
		for i, cal := range parsed.Calendars {
			log.Printf("[DEBUG] Calendar %d: action=%s, datetime=%s, prompt=%s", i, cal.Action, cal.DateTime, cal.Prompt)
		}
	}

	var feedback []map[string]interface{}

	agent, _ := s.db.GetAgent(agentID)
	containerName, _ := s.ensureAgentContainer(agent)

	// 1. Handle Thought (Internal only, no feedback needed)
	if parsed.Thought != "" && agentID > 0 {
		s.db.LogAction(agentID, "system", "agent_thought", parsed.Thought)
	}

	// 2. Handle Message (Telegram user)
	if parsed.Message != "" && bot != nil {
		err := bot.SendMessage(chatID, parsed.Message)
		status := "SUCCESS"
		if err != nil {
			status = "FAILED: " + err.Error()
			s.db.LogAction(agentID, "system", "message_failed", fmt.Sprintf("Error: %v", err))
		} else {
			s.db.LogAction(agentID, "agent", "message_sent", parsed.Message)
		}
		feedback = append(feedback, map[string]interface{}{"action": "MESSAGE", "status": status})
	}

	// 3. Handle Terminals
	for _, cmd := range parsed.Terminals {
		s.db.LogAction(agentID, "system", "terminal_execute", fmt.Sprintf("Command: %s", cmd))
		out, err := s.docker.Exec(containerName, cmd)
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
			out = err.Error()
			s.db.LogAction(agentID, "system", "terminal_failed", fmt.Sprintf("Command: %s, Error: %v", cmd, err))
		} else {
			s.db.LogAction(agentID, "system", "terminal_success", fmt.Sprintf("Command: %s", cmd))
		}
		displayOut := out
		if len(out) > 500 {
			displayOut = out[:500] + "..."
		}
		feedback = append(feedback, map[string]interface{}{
			"terminal": cmd,
			"status":   status,
			"output":   displayOut,
		})
	}

	// 4. Handle <give> tag - Send file to user.
	// Reference: docs/xml-tags.md. Media type decides whether Telegram gets a document, photo, or video.
	for _, action := range parsed.Actions {
		if action.Type == "GIVE" {
			if agentID > 0 && bot != nil {
				containerFilePath := "/app/workspace/out/" + action.Value

				content, err := s.docker.ReadFile(containerName, containerFilePath)
				if err != nil {
					s.db.LogAction(agentID, "system", "action_give_failed", fmt.Sprintf("File: %s, Error: %v", action.Value, err))
					feedback = append(feedback, map[string]interface{}{"action": "GIVE", "file": action.Value, "status": "FAILED", "error": "File not found in container"})
					continue
				}

				tmpPath := filepath.Join(os.TempDir(), action.Value)
				os.WriteFile(tmpPath, []byte(content), 0644)
				defer os.Remove(tmpPath)

				err = s.sendTransportFile(bot, chatID, tmpPath, action.Value)
				status := "SUCCESS"
				if err != nil {
					status = "FAILED"
					log.Printf("GIVE error: %v", err)
					s.db.LogAction(agentID, "system", "action_give_failed", fmt.Sprintf("File: %s, Error: %v", action.Value, err))
				} else {
					s.db.LogAction(agentID, "system", "action_give", fmt.Sprintf("File: %s", action.Value))
				}
				feedback = append(feedback, map[string]interface{}{"action": "GIVE", "file": action.Value, "status": status})
			}
		}
	}

	// 5. Handle <app> tag - Create and publish web app
	for _, app := range parsed.Apps {
		if agentID > 0 && bot != nil {
			// Create app folder and files in container
			appFolder := "/app/workspace/apps/" + app.Name

			// Create index.html
			htmlContent := app.HTML
			if htmlContent == "" {
				htmlContent = "<h1>" + app.Name + "</h1>"
			}

			// Ensure basic HTML structure if missing
			if !strings.Contains(strings.ToLower(htmlContent), "<head>") {
				htmlContent = "<head><title>" + app.Name + "</title></head>" + htmlContent
			}
			if !strings.Contains(strings.ToLower(htmlContent), "<body>") {
				// Wrap non-head content in body if body is missing
				headIdx := strings.Index(strings.ToLower(htmlContent), "</head>")
				if headIdx != -1 {
					htmlContent = htmlContent[:headIdx+7] + "<body>" + htmlContent[headIdx+7:] + "</body>"
				} else {
					htmlContent = "<body>" + htmlContent + "</body>"
				}
			}
			if !strings.Contains(strings.ToLower(htmlContent), "<html>") {
				htmlContent = "<!DOCTYPE html><html>" + htmlContent + "</html>"
			}

			// Build index.html with embedded CSS and JS if provided
			indexHTML := htmlContent
			if app.CSS != "" {
				if strings.Contains(strings.ToLower(indexHTML), "</head>") {
					indexHTML = strings.Replace(indexHTML, "</head>", "<style>"+app.CSS+"</style></head>", 1)
				} else if strings.Contains(strings.ToLower(indexHTML), "<body>") {
					indexHTML = strings.Replace(indexHTML, "<body>", "<body><style>"+app.CSS+"</style>", 1)
				}
			}
			if app.JS != "" {
				if strings.Contains(strings.ToLower(indexHTML), "</body>") {
					indexHTML = strings.Replace(indexHTML, "</body>", "<script>"+app.JS+"</script></body>", 1)
				} else {
					indexHTML += "<script>" + app.JS + "</script>"
				}
			}

			// Create folder and files
			mkdirCmd := "mkdir -p " + appFolder
			s.docker.Exec(containerName, mkdirCmd)

			// Write index.html
			writeIndexCmd := fmt.Sprintf("echo '%s' > %s/index.html", strings.ReplaceAll(indexHTML, "'", "'\\''"), appFolder)
			s.docker.Exec(containerName, writeIndexCmd)

			// Log the action
			s.db.LogAction(agentID, "system", "action_app_created", fmt.Sprintf("App: %s created", app.Name))

			feedback = append(feedback, map[string]interface{}{
				"action": "APP",
				"app":    app.Name,
				"status": "SUCCESS",
			})
		}
	}

	// Handle <deploy>app-name</deploy>
	for _, appName := range parsed.Deploys {
		if agentID > 0 && bot != nil {
			s.db.LogAction(agentID, "system", "action_app_deployed", fmt.Sprintf("App: %s deployed", appName))

			tunnelURL := s.tunnels.GetURL("dashboard")
			publicURL := tunnelURL + fmt.Sprintf("/apps/%d/%s", agentID, appName)

			bot.SendMessage(chatID, "🚀 App Deployed: "+appName+"\n\nAccess it here: "+publicURL)

			feedback = append(feedback, map[string]interface{}{
				"action": "DEPLOY",
				"app":    appName,
				"status": "SUCCESS",
				"url":    publicURL,
			})
		}
	}

	// 6. Handle Actions (SKILL)
	for _, action := range parsed.Actions {
		if action.Type == "SKILL" {
			if agentID > 0 {
				skillName := action.Value
				if !strings.HasSuffix(skillName, ".md") {
					skillName += ".md"
				}
				skillPath := filepath.Join("data", "skills", skillName)
				content, err := os.ReadFile(skillPath)
				if err == nil {
					s.addHistoryAndBroadcast(agentID, "system", "system", "Skill loaded ["+skillName+"]:\n\n"+string(content)+"\n<end>")
					s.db.LogAction(agentID, "system", "action_skill", fmt.Sprintf("Skill: %s", action.Value))
					feedback = append(feedback, map[string]interface{}{"action": "SKILL", "skill": action.Value, "status": "SUCCESS"})
				} else {
					s.db.LogAction(agentID, "system", "action_skill_failed", fmt.Sprintf("Skill: %s, Error: %v", action.Value, err))
					feedback = append(feedback, map[string]interface{}{"action": "SKILL", "skill": action.Value, "status": "FAILED", "error": "Skill not found"})
				}
			}
		}
	}

	// 7. Handle System
	if parsed.System == "time" {
		feedback = append(feedback, map[string]interface{}{"system": "time", "value": s.db.GetSystemTime().Format(time.RFC3339)})
	} else if parsed.System == "memory" {
		if s.docker != nil {
			stats, _ := s.docker.LatestSystemMetrics()
			memMB := float64(stats.Host.MemoryUsed) / (1024 * 1024)
			feedback = append(feedback, map[string]interface{}{"system": "memory", "value": fmt.Sprintf("%.2f MB", memMB)})
		}
	}

	// 8. Handle Calendar (multiple events and CRUD)
	for _, cal := range parsed.Calendars {
		switch cal.Action {
		case "list":
			// Get all calendar events for this agent
			events, err := s.db.ListCalendarEventsByAgent(agentID)
			if err != nil {
				feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_LIST", "status": "ERROR", "error": err.Error()})
			} else {
				eventList := "📅 Existing Calendar Events:\n\n"
				for _, e := range events {
					status := "⏳ Pending"
					if e.Executed {
						status = "✅ Completed"
					}
					eventList += fmt.Sprintf("• ID: %d | %s at %s\n  %s [%s]\n\n", e.ID, e.Date, e.Time, e.Prompt, status)
				}
				if len(events) == 0 {
					eventList += "No calendar events found."
				}
				s.addHistoryAndBroadcast(agentID, "system", "system", eventList+"\n<end>")
				feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_LIST", "status": "SUCCESS", "events": events})
			}

		case "delete":
			// Delete a calendar event
			eventID, err := strconv.ParseInt(cal.ID, 10, 64)
			if err != nil {
				feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_DELETE", "status": "ERROR", "error": "Invalid event ID"})
			} else {
				err := s.db.DeleteCalendarEvent(eventID)
				if err != nil {
					feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_DELETE", "status": "ERROR", "error": err.Error()})
				} else {
					s.addHistoryAndBroadcast(agentID, "system", "system", fmt.Sprintf("Calendar Event Deleted: ID %d\n<end>", eventID))
					feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_DELETE", "status": "SUCCESS", "id": eventID})
				}
			}

		case "update":
			// Update a calendar event
			eventID, err := strconv.ParseInt(cal.ID, 10, 64)
			if err != nil {
				feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_UPDATE", "status": "ERROR", "error": "Invalid event ID"})
			} else {
				// Get existing event
				existing, err := s.db.GetCalendarEvent(eventID)
				if err != nil {
					feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_UPDATE", "status": "ERROR", "error": "Event not found"})
				} else {
					// Update fields
					newDate := existing.Date
					newTime := existing.Time
					newPrompt := existing.Prompt

					if cal.DateTime != "" {
						if len(cal.DateTime) >= 10 {
							newDate = cal.DateTime[:10]
						}
						if len(cal.DateTime) >= 16 {
							newTime = cal.DateTime[11:16]
						}
					}
					if cal.Prompt != "" {
						newPrompt = cal.Prompt
					}

					// Delete old and create new (simpler than update)
					s.db.DeleteCalendarEvent(eventID)
					_, err := s.db.CreateCalendarEvent(&db.CalendarEvent{
						AgentID:  agentID,
						Date:     newDate,
						Time:     newTime,
						Prompt:   newPrompt,
						Executed: false,
					})
					if err != nil {
						feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_UPDATE", "status": "ERROR", "error": err.Error()})
					} else {
						s.db.AddHistory(agentID, "system", "system", fmt.Sprintf("Calendar Event Updated: ID %d\n<end>", eventID))
						feedback = append(feedback, map[string]interface{}{"action": "CALENDAR_UPDATE", "status": "SUCCESS", "id": eventID})
					}
				}
			}

		default: // create
			// Support both absolute datetime and relative time (schedule) fields
			var dateStr, timeStr string
			var targetTime time.Time

			// Handle relative time via <schedule minutes="N" hours="N" days="N">
			if cal.ScheduleMinutes > 0 || cal.ScheduleHours > 0 || cal.ScheduleDays > 0 {
				// Calculate future time from current system time
				targetTime = s.db.GetSystemTime()
				targetTime = targetTime.Add(time.Duration(cal.ScheduleMinutes) * time.Minute)
				targetTime = targetTime.Add(time.Duration(cal.ScheduleHours) * time.Hour)
				targetTime = targetTime.Add(time.Duration(cal.ScheduleDays) * 24 * time.Hour)
				dateStr = targetTime.Format("2006-01-02")
				timeStr = targetTime.Format("15:04")
				log.Printf("[CALENDAR] Relative schedule: minutes=%d, hours=%d, days=%d -> scheduled for %s %s",
					cal.ScheduleMinutes, cal.ScheduleHours, cal.ScheduleDays, dateStr, timeStr)
			} else {
				// Handle absolute datetime via <datetime>
				dateTime := cal.DateTime
				if dateTime != "" {
					if len(dateTime) >= 10 {
						dateStr = dateTime[:10]
					}
					if len(dateTime) >= 19 {
						timeStr = dateTime[11:16]
					} else if len(dateTime) >= 16 {
						timeStr = dateTime[11:16]
					}
				}
			}

			// Create the calendar event if we have both date and time
			if dateStr != "" && timeStr != "" {
				_, err := s.db.CreateCalendarEvent(&db.CalendarEvent{
					AgentID:  agentID,
					Date:     dateStr,
					Time:     timeStr,
					Prompt:   cal.Prompt,
					Executed: false,
				})
				if err != nil {
					s.db.AddHistory(agentID, "system", "system", fmt.Sprintf("Calendar Event Failed: %v", err))
					feedback = append(feedback, map[string]interface{}{"action": "CALENDAR", "status": "ERROR", "error": err.Error()})
				} else {
					s.db.AddHistory(agentID, "system", "system", fmt.Sprintf("Calendar Event Scheduled: %s %s - %s\n<end>", dateStr, timeStr, cal.Prompt))
					feedback = append(feedback, map[string]interface{}{"action": "CALENDAR", "status": "SUCCESS"})
				}
			} else if cal.Prompt != "" {
				s.db.AddHistory(agentID, "system", "system", fmt.Sprintf("Calendar Event Failed: Invalid date/time\n<end>"))
				feedback = append(feedback, map[string]interface{}{"action": "CALENDAR", "status": "ERROR", "error": "Invalid date/time"})
			}
		}
	}

	return feedback
}

func (s *Server) handleTakeoverInput(agentId int64, chatID, xmlInput string, bot *telegram.Bot) {
	parsedInput := parser.ParseLLMOutput(xmlInput)
	processedLog := describeParsedTags(parsedInput)
	s.addHistoryAndBroadcast(agentId, "system", "system", processedLog)
	s.db.LogAction(agentId, "system", "takeover_input", processedLog)

	feedback := s.ExecuteXMLPayload(agentId, chatID, xmlInput, bot)
	response := formatSystemExecutionResponse(parsedInput, feedback)
	s.addHistoryAndBroadcast(agentId, "system", "system", response)
}

// HandleExportBackup exports all app data to a .zip file.
// Docs: See docs/backup-restore.md for detailed documentation on backup process.
// Data included: database (hermit.db), images (data/image), skills (data/skills),
// agent data (data/agents), and app logs.
func (s *Server) HandleExportBackup(c *fiber.Ctx) error {
	// Create a buffer to write the zip file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Base directories to backup
	baseDirs := []string{"data", "hermit.log"}

	// Walk through each directory and add files to zip
	for _, dir := range baseDirs {
		if _, err := os.Stat(dir); err == nil {
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip the database WAL and SHM files (they're not needed for restore)
				if strings.HasSuffix(path, ".db-wal") || strings.HasSuffix(path, ".db-shm") {
					return nil
				}

				// Calculate relative path for the archive
				relPath, err := filepath.Rel(filepath.Dir(dir), path)
				if err != nil {
					return err
				}

				// If it's the root directory itself, skip
				if relPath == "." {
					return nil
				}

				// For files in root like hermit.log
				if dir == "hermit.log" && path == "hermit.log" {
					relPath = "hermit.log"
				}

				// Add file to zip
				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}
				header.Name = relPath
				header.Method = zip.Deflate

				writer, err := zipWriter.CreateHeader(header)
				if err != nil {
					return err
				}

				if !info.IsDir() {
					content, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					_, err = writer.Write(content)
					if err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to archive " + dir + ": " + err.Error()})
			}
		}
	}

	// Close the zip writer
	if err := zipWriter.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create zip: " + err.Error()})
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("hermit-backup-%s.zip", timestamp)

	// Set headers for file download
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename="+filename)

	// Send the zip file
	return c.Send(buf.Bytes())
}

// HandleImportBackup restores app data from a .zip file.
// Docs: See docs/backup-restore.md for detailed documentation on restore process.
// Requires password verification for security.
// Warning: This will overwrite existing data.
func (s *Server) HandleImportBackup(c *fiber.Ctx) error {
	// Parse request with password
	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Verify password by checking against stored credentials
	session := c.Cookies("session")
	if session == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	userID, _ := strconv.ParseInt(session, 10, 64)
	username, _, err := s.db.GetUserByID(userID)
	if err != nil || username == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid session"})
	}

	// Verify password
	_, _, err = s.db.VerifyUser(username, req.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid password"})
	}

	// Get the uploaded file
	file, err := c.FormFile("backup")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "No backup file uploaded"})
	}

	// Open the uploaded file
	openedFile, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to open backup file"})
	}
	defer openedFile.Close()

	// Read the entire file content
	zipContent, err := io.ReadAll(openedFile)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read backup file"})
	}

	// Create a zip reader from the content
	zipReader, err := zip.NewReader(bytes.NewReader(zipContent), int64(len(zipContent)))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid backup file format"})
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "hermit-restore-*")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create temp directory"})
	}
	defer os.RemoveAll(tempDir)

	// Extract all files from the zip - convert to slice first
	files := make([]*zip.File, len(zipReader.File))
	copy(files, zipReader.File)

	for _, zipFile := range files {
		filename := zipFile.Name

		// Security check: prevent path traversal
		if strings.Contains(filename, "..") || strings.HasPrefix(filename, "/") {
			continue
		}

		targetPath := filepath.Join(tempDir, filename)

		// Create directory if needed
		if zipFile.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create directory: " + err.Error()})
		}

		// Extract file
		source, err := zipFile.Open()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read zip entry: " + err.Error()})
		}
		defer source.Close()

		dest, err := os.Create(targetPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create file: " + err.Error()})
		}
		defer dest.Close()

		if _, err := io.Copy(dest, source); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to extract file: " + err.Error()})
		}
	}

	// Now restore the files to their proper locations
	// 1. Restore database
	dbFile := filepath.Join(tempDir, "data", "hermit.db")
	if _, err := os.Stat(dbFile); err == nil {
		// Close existing database connection before replacing
		// The main app will need to reconnect after restart
		currentDB := "data/hermit.db"
		backupDB := "data/hermit.db.backup"

		// Backup current database
		if _, err := os.Stat(currentDB); err == nil {
			os.Rename(currentDB, backupDB)
		}

		// Copy new database
		if err := copyFile(dbFile, currentDB); err != nil {
			// Restore backup if failed
			if _, err := os.Stat(backupDB); err == nil {
				os.Rename(backupDB, currentDB)
			}
			return c.Status(500).JSON(fiber.Map{"error": "Failed to restore database: " + err.Error()})
		}

		// Remove backup after successful restore
		if _, err := os.Stat(backupDB); err == nil {
			os.Remove(backupDB)
		}
	}

	// 2. Restore data/image directory
	imageDir := filepath.Join(tempDir, "data", "image")
	if _, err := os.Stat(imageDir); err == nil {
		targetImageDir := "data/image"
		os.MkdirAll(targetImageDir, 0755)

		err := filepath.Walk(imageDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(imageDir, path)
			targetPath := filepath.Join(targetImageDir, relPath)
			return copyFile(path, targetPath)
		})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to restore images: " + err.Error()})
		}
	}

	// 3. Restore data/skills directory
	skillsDir := filepath.Join(tempDir, "data", "skills")
	if _, err := os.Stat(skillsDir); err == nil {
		targetSkillsDir := "data/skills"
		os.MkdirAll(targetSkillsDir, 0755)

		err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(skillsDir, path)
			targetPath := filepath.Join(targetSkillsDir, relPath)
			return copyFile(path, targetPath)
		})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to restore skills: " + err.Error()})
		}
	}

	// 4. Restore data/agents directory
	agentsDir := filepath.Join(tempDir, "data", "agents")
	if _, err := os.Stat(agentsDir); err == nil {
		targetAgentsDir := "data/agents"
		os.MkdirAll(targetAgentsDir, 0755)

		err := filepath.Walk(agentsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relPath, _ := filepath.Rel(agentsDir, path)
			targetPath := filepath.Join(targetAgentsDir, relPath)
			parentDir := filepath.Dir(targetPath)
			os.MkdirAll(parentDir, 0755)
			return copyFile(path, targetPath)
		})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to restore agent data: " + err.Error()})
		}
	}

	// 5. Restore hermit.log if exists
	logFile := filepath.Join(tempDir, "hermit.log")
	if _, err := os.Stat(logFile); err == nil {
		copyFile(logFile, "hermit.log")
	}

	// Log the restore action
	s.db.LogAction(0, username, "system", "backup_restored")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backup restored successfully. Some changes may require restart to take effect.",
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (s *Server) BroadcastMessage(msg string) {
	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()
	for client := range s.wsClients {
		if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			client.Close()
			delete(s.wsClients, client)
		}
	}
}

func (s *Server) addHistoryAndBroadcast(agentID int64, userID, role, content string) {
	s.addHistoryAndBroadcastWithFiles(agentID, userID, role, content, nil)
}

func (s *Server) addHistoryAndBroadcastWithRejection(agentID int64, userID, role, content string, isRejected bool) {
	s.db.AddHistoryWithRejection(agentID, userID, role, content, isRejected)

	payload := map[string]interface{}{
		"type":     "new_message",
		"agent_id": agentID,
		"user_id":  userID,
		"role":     role,
		"content":  content,
	}
	if jsonMsg, err := json.Marshal(payload); err == nil {
		s.BroadcastMessage(string(jsonMsg))
	}
}
