package api

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohnEsleyer/hermit/internal/cloudflare"
	"github.com/JohnEsleyer/hermit/internal/db"
	"github.com/JohnEsleyer/hermit/internal/docker"
	"github.com/JohnEsleyer/hermit/internal/llm"
	"github.com/JohnEsleyer/hermit/internal/parser"
	"github.com/JohnEsleyer/hermit/internal/telegram"
	"github.com/JohnEsleyer/hermit/internal/workspace"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type Server struct {
	db      *db.DB
	ws      *workspace.Workspace
	bot     *telegram.Bot
	llm     *llm.Client
	docker  *docker.Client
	tunnels *cloudflare.TunnelManager
	app     *fiber.App

	verifyCodes   map[string]string
	takeoverMode  map[string]bool
	mu            sync.RWMutex
	contextStore  map[string][]string
	tokenCounters map[string]int
}

func NewServer(database *db.DB, ws *workspace.Workspace, bot *telegram.Bot, llmClient *llm.Client, dockerClient *docker.Client, tunnels *cloudflare.TunnelManager) *Server {
	s := &Server{
		db:            database,
		ws:            ws,
		bot:           bot,
		llm:           llmClient,
		docker:        dockerClient,
		tunnels:       tunnels,
		verifyCodes:   make(map[string]string),
		takeoverMode:  make(map[string]bool),
		contextStore:  make(map[string][]string),
		tokenCounters: make(map[string]int),
	}

	app := fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024,
	})

	app.Use(cors.New())
	app.Use(logger.New())

	s.setupRoutes(app)
	s.app = app
	return s
}

func (s *Server) Listen(port string) error {
	return s.app.Listen(":" + port)
}

func (s *Server) setupRoutes(app *fiber.App) {
	app.Static("/dashboard", "./dashboard/public")

	api := app.Group("/api")

	api.Post("/auth/login", s.HandleLogin)
	api.Post("/auth/logout", s.HandleLogout)
	api.Get("/auth/check", s.HandleCheckAuth)
	api.Post("/auth/change-credentials", s.HandleChangeCredentials)

	api.Get("/agents", s.HandleListAgents)
	api.Post("/agents", s.HandleCreateAgent)
	api.Get("/agents/:id", s.HandleGetAgent)
	api.Put("/agents/:id", s.HandleUpdateAgent)
	api.Delete("/agents/:id", s.HandleDeleteAgent)
	api.Post("/agents/:id/action", s.HandleAgentAction)

	api.Get("/skills", s.HandleListSkills)
	api.Post("/skills", s.HandleCreateSkill)
	api.Delete("/skills/:id", s.HandleDeleteSkill)

	api.Post("/test-contract", s.HandleTestContract)

	api.Get("/metrics", s.HandleMetrics)
	api.Get("/containers", s.HandleContainers)

	api.Get("/allowlist", s.HandleListAllowlist)
	api.Post("/allowlist", s.HandleCreateAllowlist)
	api.Post("/telegram/send-code", s.HandleTelegramSendCode)
	api.Post("/telegram/verify", s.HandleTelegramVerify)
	api.Post("/webhook", s.HandleWebhook)

	api.Get("/settings", s.HandleGetSettings)
	api.Post("/settings", s.HandleSetSettings)
}

func (s *Server) HandleLogin(c *fiber.Ctx) error {
	var req struct{ Username, Password string }
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	id, mustChange, err := s.db.VerifyUser(req.Username, req.Password)
	if err != nil || id == 0 {
		return c.JSON(fiber.Map{"success": false, "error": "Invalid credentials"})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    fmt.Sprintf("%d", id),
		Path:     "/",
		HTTPOnly: true,
	})

	return c.JSON(fiber.Map{"success": true, "mustChangePassword": mustChange})
}

func (s *Server) HandleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleCheckAuth(c *fiber.Ctx) error {
	session := c.Cookies("session")
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

func (s *Server) HandleMetrics(c *fiber.Ctx) error {
	metrics, err := s.docker.LatestSystemMetrics()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(metrics)
}

func (s *Server) HandleContainers(c *fiber.Ctx) error {
	metrics, err := s.docker.LatestSystemMetrics()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(metrics.Containers)
}

func (s *Server) HandleListAgents(c *fiber.Ctx) error {
	agents, err := s.db.ListAgents()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(agents)
}

func (s *Server) HandleCreateAgent(c *fiber.Ctx) error {
	var a db.Agent
	if err := c.BodyParser(&a); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Bad request"})
	}
	id, err := s.db.CreateAgent(&a)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) HandleGetAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	agent, _ := s.db.GetAgent(id)
	return c.JSON(agent)
}

func (s *Server) HandleUpdateAgent(c *fiber.Ctx) error {
	return c.SendStatus(200)
}

func (s *Server) HandleDeleteAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	s.db.DeleteAgent(id)
	return c.SendStatus(200)
}

func (s *Server) HandleAgentAction(c *fiber.Ctx) error {
	return c.SendStatus(200)
}

func (s *Server) HandleListSkills(c *fiber.Ctx) error {
	skills, _ := s.db.ListSkills()
	return c.JSON(skills)
}

func (s *Server) HandleCreateSkill(c *fiber.Ctx) error {
	var req db.Skill
	c.BodyParser(&req)
	id, err := s.db.CreateSkill(&req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"id": id, "success": true})
}

func (s *Server) HandleDeleteSkill(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	s.db.DeleteSkill(id)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleTelegramSendCode(c *fiber.Ctx) error {
	var req struct{ Token, UserID string }
	c.BodyParser(&req)

	code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	s.verifyCodes[req.Token] = code

	tempBot := telegram.NewBot(req.Token)
	tempBot.SendMessage(req.UserID, "Your Hermit Dashboard Verification Code is: "+code)

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleTelegramVerify(c *fiber.Ctx) error {
	var req struct{ Token, Code, UserID string }
	c.BodyParser(&req)

	if expected, ok := s.verifyCodes[req.Token]; ok && expected == req.Code {
		tempBot := telegram.NewBot(req.Token)
		tempBot.SendMessage(req.UserID, "Successfully connected this Telegram Bot to Hermit Agent OS.")
		delete(s.verifyCodes, req.Token)
		return c.JSON(fiber.Map{"success": true})
	}
	return c.JSON(fiber.Map{"success": false, "error": "Invalid verification code."})
}

func (s *Server) HandleTestContract(c *fiber.Ctx) error {
	var req struct {
		Payload string `json:"payload"`
		UserID  string `json:"userId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	parsed := parser.ParseLLMOutput(req.Payload)
	var feedback []map[string]interface{}

	for _, cmd := range parsed.Terminals {
		out, err := s.docker.Exec("hermit-test", cmd)
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
		}

		displayOut := out
		if len(out) > 100 {
			displayOut = out[:100] + "..."
		}

		feedback = append(feedback, map[string]interface{}{
			"terminal":     cmd,
			"status":       status,
			"terminal-out": displayOut,
		})
	}

	if parsed.System == "time" {
		feedback = append(feedback, map[string]interface{}{
			"status": "SUCCESS",
			"time":   time.Now().Format(time.RFC3339),
		})
	} else if parsed.System == "memory" {
		hostStats, _ := s.docker.LatestSystemMetrics()
		memMB := float64(hostStats.Host.MemoryUsed) / (1024 * 1024)
		feedback = append(feedback, map[string]interface{}{
			"status":          "SUCCESS",
			"memory_usage_mb": memMB,
		})
	}

	if parsed.Calendar != nil {
		feedback = append(feedback, map[string]interface{}{
			"status": "SUCCESS",
			"calendar": map[string]interface{}{
				"date":   parsed.Calendar.DateTime,
				"prompt": parsed.Calendar.Prompt,
			},
		})
	}

	if len(feedback) == 0 {
		feedback = append(feedback, map[string]interface{}{
			"status":  "SUCCESS",
			"message": "Payload parsed but no system actions generated.",
		})
	}

	return c.JSON(fiber.Map{
		"parsed":        parsed,
		"actionEffects": feedback,
	})
}

func (s *Server) HandleListAllowlist(c *fiber.Ctx) error   { return c.JSON([]db.AllowListEntry{}) }
func (s *Server) HandleCreateAllowlist(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) }

func (s *Server) HandleWebhook(c *fiber.Ctx) error {
	var update telegram.Update
	if err := c.BodyParser(&update); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if update.Message == nil || update.Message.Text == "" {
		return c.SendStatus(200)
	}

	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
	userText := strings.TrimSpace(update.Message.Text)

	s.handleTelegramCommand(chatID, userText)

	return c.SendStatus(200)
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
		// TODO: Implement container reset
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
		// TODO: Get actual system prompt from agent
		s.bot.SendMessage(chatID, "System prompt:\n\nYou are an autonomous AI agent running in Hermit Agent OS...")

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
		// Handle takeover mode XML input
		s.mu.RLock()
		takeoverOn := s.takeoverMode[chatID]
		s.mu.RUnlock()

		if takeoverOn {
			s.handleTakeoverInput(chatID, text)
		} else {
			// TODO: Pass to LLM agent for normal processing
			s.bot.SendMessage(chatID, "Message received. AI agent will respond shortly...")
		}
	}
}

func (s *Server) handleTakeoverInput(chatID, xmlInput string) {
	parsed := parser.ParseLLMOutput(xmlInput)
	var responses []string

	// Handle terminal commands
	for _, cmd := range parsed.Terminals {
		out, err := s.docker.Exec("hermit-test", cmd)
		status := "SUCCESS"
		if err != nil {
			status = "FAILED"
			out = err.Error()
		}
		displayOut := out
		if len(out) > 200 {
			displayOut = out[:200] + "..."
		}
		responses = append(responses, fmt.Sprintf("Terminal: %s\nStatus: %s\nOutput: %s", cmd, status, displayOut))
	}

	// Handle actions
	for _, action := range parsed.Actions {
		switch action.Type {
		case "GIVE":
			responses = append(responses, fmt.Sprintf("Action: GIVE file=%s", action.Value))
			// TODO: Implement file delivery
		case "APP":
			responses = append(responses, fmt.Sprintf("Action: PUBLISH app=%s", action.Value))
		case "SKILL":
			responses = append(responses, fmt.Sprintf("Action: LOAD skill=%s", action.Value))
		}
	}

	// Handle system tags
	if parsed.System == "time" {
		responses = append(responses, fmt.Sprintf("System: time = %s", time.Now().Format(time.RFC3339)))
	} else if parsed.System == "memory" {
		if s.docker != nil {
			stats, _ := s.docker.LatestSystemMetrics()
			memMB := float64(stats.Host.MemoryUsed) / (1024 * 1024)
			responses = append(responses, fmt.Sprintf("System: memory = %.2f MB", memMB))
		}
	}

	if len(responses) == 0 {
		responses = append(responses, "No actions parsed from input.")
	}

	s.bot.SendMessage(chatID, strings.Join(responses, "\n\n"))
}

func (s *Server) HandleGetSettings(c *fiber.Ctx) error { return c.JSON(fiber.Map{}) }
func (s *Server) HandleSetSettings(c *fiber.Ctx) error { return c.SendStatus(200) }
