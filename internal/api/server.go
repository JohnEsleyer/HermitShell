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
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
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
	return s
}

func (s *Server) Listen(port string) error {
	return s.app.Listen(":" + port)
}

// setupRoutes registers all API endpoints with Fiber router.
// Docs: See docs/api-endpoints.md for how to add new endpoints.
// Docs: See docs/frontend-backend-communication.md for frontend integration.
func (s *Server) setupRoutes(app *fiber.App) {
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
	api.Get("/agents/:id/logs", s.HandleGetAgentLogs)
	api.Get("/agents/:id/stats", s.HandleGetAgentStats)

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
	api.Get("/containers/:id/download", s.HandleContainerDownload)

	api.Get("/settings", s.HandleGetSettings)
	api.Post("/settings", s.HandleSetSettings)
	api.Get("/settings/domain-status", s.HandleDomainStatus)
	api.Get("/time", s.HandleGetTime)

	api.Post("/test-contract", s.HandleTestContract)

	api.Post("/images/upload", s.HandleImageUpload)

	api.Post("/telegram/send-code", s.HandleTelegramSendCode)
	api.Post("/telegram/verify", s.HandleTelegramVerify)
	api.Post("/webhook", s.HandleWebhook)
	api.Post("/webhook/:agentId", s.HandleAgentWebhook)

	// Agent Specific Skills
	api.Get("/agents/:id/skills", s.HandleListAgentSkills)
	api.Post("/agents/:id/skills", s.HandleSaveSkill)
	api.Delete("/agents/:id/skills/:skillId", s.HandleDeleteSkill)

	// App serving
	api.Get("/apps/:agentId/:appName/*", s.HandleServeApp)
	api.Get("/apps/:agentId/:appName", s.HandleServeApp)

	s.setupStaticRoutes(app)
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
			"cpuPercent": cpu, // for legacy HealthTab
			"memory":     mem,
			"memUsageMB": mem, // for legacy HealthTab
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

	domainMode, _ := s.db.GetSetting("domain_mode")
	tunnelURL := ""
	if domainMode != "true" {
		tunnelURL = s.tunnels.GetURL("dashboard")
	}

	domain, _ := s.db.GetSetting("domain")

	return c.JSON(fiber.Map{
		"host":       metrics.Host,
		"containers": allContainers,
		"tunnelURL":  tunnelURL,
		"domain":     domain,
		"domainMode": domainMode == "true",
	})
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
		TelegramToken string `json:"telegramToken"`
		TelegramID    string `json:"telegramId"`
		Status        string `json:"status"`
		Model         string `json:"model"`
		AllowedUsers  string `json:"allowedUsers"`
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
		TelegramToken: req.TelegramToken,
		TelegramID:    req.TelegramID,
		Status:        "standby",
		Active:        true,
	}
	if a.Role == "" {
		a.Role = "assistant"
	}
	if a.Provider == "" {
		a.Provider = "openrouter"
	}
	a.Model = req.Model
	a.AllowedUsers = req.AllowedUsers

	id, err := s.db.CreateAgent(&a)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Log agent creation
	s.db.LogAction(id, "system", "agent_created", fmt.Sprintf("Agent '%s' created with provider=%s, model=%s", a.Name, a.Provider, a.Model))

	// Create agent-specific skills folder with context.md
	agentSkillsPath := fmt.Sprintf("data/agents/%d/skills", id)
	os.MkdirAll(agentSkillsPath, 0755)
	os.WriteFile(filepath.Join(agentSkillsPath, "context.md"), []byte(a.Personality), 0644)

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
		Model         string `json:"model"`
		AllowedUsers  string `json:"allowedUsers"`
		TelegramID    string `json:"telegramId"`
		TelegramToken string `json:"telegramToken"`
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
	existing.Model = req.Model
	existing.AllowedUsers = req.AllowedUsers
	if req.TelegramID != "" {
		existing.TelegramID = req.TelegramID
	}
	if req.TelegramToken != "" {
		existing.TelegramToken = req.TelegramToken
	}

	if err := s.db.UpdateAgent(existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleDeleteAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)

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
	domainMode, _ := s.db.GetSetting("domain_mode")
	domain, _ := s.db.GetSetting("domain")
	openrouterKey, _ := s.db.GetSetting("openrouter_api_key")
	openaiKey, _ := s.db.GetSetting("openai_api_key")
	anthropicKey, _ := s.db.GetSetting("anthropic_api_key")
	geminiKey, _ := s.db.GetSetting("gemini_api_key")
	timezone, _ := s.db.GetSetting("timezone")
	timeOffset, _ := s.db.GetSetting("time_offset")

	tunnelURL := s.tunnels.GetURL("dashboard")
	isHealthy := s.tunnels.CheckTunnelHealth("dashboard", 2*time.Second)

	if domainMode != "true" && tunnelURL == "" {
		port, _ := strconv.Atoi(os.Getenv("PORT"))
		if port == 0 {
			port = 3000
		}
		go s.tunnels.StartQuickTunnel("dashboard", port)
	}

	return c.JSON(fiber.Map{
		"domainMode":    domainMode == "true",
		"domain":        domain,
		"tunnelURL":     tunnelURL,
		"tunnelHealthy": isHealthy,
		"status":        s.getTunnelStatus(domainMode == "true", isHealthy),
		"openrouterKey": openrouterKey != "",
		"openaiKey":     openaiKey != "",
		"anthropicKey":  anthropicKey != "",
		"geminiKey":     geminiKey != "",
		"timezone":      timezone,
		"timeOffset":    timeOffset,
		"hasLLMKey":     openrouterKey != "" || openaiKey != "" || anthropicKey != "" || geminiKey != "",
	})
}

func (s *Server) getTunnelStatus(domainMode, healthy bool) string {
	if domainMode {
		return "Domain Mode Active"
	}
	if healthy {
		return "Active (Quick Tunnel)"
	}
	return "Provisioning..."
}

// HandleSetSettings saves system settings including timezone and time offset.
// Docs: See docs/time-management.md for time management flow and persistence.
func (s *Server) HandleSetSettings(c *fiber.Ctx) error {
	var req struct {
		DomainMode    string `json:"domainMode"`
		Domain        string `json:"domain"`
		OpenrouterKey string `json:"openrouterKey"`
		OpenaiKey     string `json:"openaiKey"`
		AnthropicKey  string `json:"anthropicKey"`
		GeminiKey     string `json:"geminiKey"`
		Timezone      string `json:"timezone"`
		TimeOffset    string `json:"timeOffset"`
	}
	c.BodyParser(&req)

	if req.DomainMode != "" {
		s.db.SetSetting("domain_mode", req.DomainMode)
	}
	if req.Domain != "" {
		s.db.SetSetting("domain", req.Domain)
	}
	// Save timezone and time offset settings
	if req.Timezone != "" {
		s.db.SetSetting("timezone", req.Timezone)
	}
	if req.TimeOffset != "" {
		s.db.SetSetting("time_offset", req.TimeOffset)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleDomainStatus(c *fiber.Ctx) error {
	domain, _ := s.db.GetSetting("domain")

	return c.JSON(fiber.Map{
		"domain":     domain,
		"configured": domain != "",
		"message":    "DNS A record should point to this server's IP",
	})
}

// HandleGetTime returns the current time with user's offset applied.
// Docs: See docs/time-management.md for how offset is calculated and applied.
// Purpose: Allows displaying time in user's desired timezone regardless of server location.
func (s *Server) HandleGetTime(c *fiber.Ctx) error {
	timezone, _ := s.db.GetSetting("timezone")
	timeOffset, _ := s.db.GetSetting("time_offset")

	// Get current UTC time from server
	currentTime := time.Now().UTC()

	// Apply offset to get user's desired time
	// Formula: displayed_time = server_utc_time + offset
	offsetHours := 0
	if timeOffset != "" {
		fmt.Sscanf(timeOffset, "%d", &offsetHours)
	}
	currentTime = currentTime.Add(time.Duration(offsetHours) * time.Hour)

	// Note: We don't convert to timezone here because the offset already
	// represents the user's desired timezone difference from UTC.
	// The timezone setting is kept for reference but offset is primary.

	// Format time in UTC for consistent display regardless of server timezone
	utcTime := currentTime.UTC()

	return c.JSON(fiber.Map{
		"time":       utcTime.Format("03:04:05 PM"),
		"time12":     utcTime.Format("3:04 PM"),
		"date":       utcTime.Format("Mon, Jan 2"),
		"fullDate":   utcTime.Format("2006-01-02"),
		"datetime":   utcTime.Format(time.RFC3339),
		"timezone":   timezone,
		"timeOffset": timeOffset,
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
		Payload string `json:"payload"`
		UserID  string `json:"userId"`
		AgentID int64  `json:"agentId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	log.Printf("[TEST CONTRACT] Received payload: %s", req.Payload)

	agent, _ := s.db.GetAgent(req.AgentID)
	var agentBot *telegram.Bot

	// Create bot if agent exists - userId can be a real Telegram chat ID for testing
	if agent != nil {
		agentBot = telegram.NewBot(agent.TelegramToken)
	}

	feedback := s.ExecuteXMLPayload(req.AgentID, req.UserID, req.Payload, agentBot)

	log.Printf("[TEST CONTRACT] Feedback: %v", feedback)

	// Only add to history if userId is not a test placeholder
	if req.UserID != "test" && req.UserID != "test-user" && agent != nil {
		s.db.AddHistory(agent.ID, "assistant", "assistant", req.Payload)
	}

	return c.JSON(fiber.Map{
		"actionEffects": feedback,
	})
}

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

// HandleAgentWebhook receives Telegram updates and processes messages.
// Docs: See docs/telegram-integration.md for complete Telegram flow.
// Docs: See docs/security-measures.md for allowlist authorization.
func (s *Server) HandleAgentWebhook(c *fiber.Ctx) error {
	agentId, _ := strconv.ParseInt(c.Params("agentId"), 10, 64)
	agent, err := s.db.GetAgent(agentId)
	if err != nil || agent == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Agent not found"})
	}

	var update telegram.Update
	if err := c.BodyParser(&update); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if update.Message == nil || update.Message.Text == "" {
		return c.SendStatus(200)
	}

	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
	userText := strings.TrimSpace(update.Message.Text)
	userID := fmt.Sprintf("%d", update.Message.From.ID)

	// Log incoming message
	log.Printf("[Telegram] Agent=%s, From=%s(@%s), Message=%s", agent.Name, userID, update.Message.From.Username, userText)
	s.db.LogAction(agentId, "agent", "telegram_received", fmt.Sprintf("From: %s(@%s), Message: %s", userID, update.Message.From.Username, userText))

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
	s.db.LogAction(agentId, "agent", "telegram_message", fmt.Sprintf("From: %s (ID: %s), Allowed: %v, Reason: %s", update.Message.From.Username, userID, allowed, authReason))

	if !allowed {
		tempBot := telegram.NewBot(agent.TelegramToken)
		tempBot.SendMessage(chatID, fmt.Sprintf("You are not authorized to use this agent.\n\nDebug info:\n- Your ID: %s\n- Your username: @%s\n- Allowed users: %s", userID, update.Message.From.Username, agent.AllowedUsers))
		return c.SendStatus(200)
	}

	// Handle Commands
	if strings.HasPrefix(userText, "/") {
		return s.handleAgentCommand(agent, chatID, userText)
	}

	// Log user message
	s.db.AddHistory(agentId, userID, "user", userText)

	// Ensure container is running before processing AI request
	if _, err := s.ensureAgentContainer(agent); err != nil {
		log.Printf("Failed to ensure container for agent %s: %v", agent.Name, err)
	}

	s.mu.RLock()
	takeoverOn := s.takeoverMode[chatID]
	s.mu.RUnlock()

	if takeoverOn {
		tempBot := telegram.NewBot(agent.TelegramToken)
		s.handleTakeoverInput(agentId, chatID, userText, tempBot)
	} else {
		go s.processAgentAIRequest(agent, chatID, userID, userText)
	}

	return c.SendStatus(200)
}

func (s *Server) handleAgentCommand(agent *db.Agent, chatID, text string) error {
	bot := telegram.NewBot(agent.TelegramToken)
	cmd := strings.Split(text, " ")[0]

	switch cmd {
	case "/status":
		statusMsg := fmt.Sprintf("🤖 *Agent Status: %s*\n\n", agent.Name)
		statusMsg += fmt.Sprintf("• Model: `%s`\n", agent.Model)
		statusMsg += fmt.Sprintf("• Provider: `%s`\n", agent.Provider)
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

		info, err := bot.GetWebhookInfo()
		if err != nil {
			statusMsg += "• Webhook: ❌ Error fetching\n"
		} else {
			webhookStatus := "Mismatch ⚠️"
			currentTunnel := s.tunnels.GetURL("dashboard")
			if strings.HasPrefix(info.URL, currentTunnel) {
				webhookStatus = "Active ✅"
			}
			statusMsg += fmt.Sprintf("• Webhook: %s (`%s`)\n", webhookStatus, info.URL)
		}

		statusMsg += fmt.Sprintf("\n🔐 *Authorization*\n")
		statusMsg += fmt.Sprintf("• Allowed Users: `%s`\n", agent.AllowedUsers)
		statusMsg += fmt.Sprintf("• Your User ID: `%s`\n", chatID)
		if agent.AllowedUsers == "" {
			statusMsg += "• Status: ✅ No restrictions\n"
		} else {
			statusMsg += "• Status: ⚠️ Restricted\n"
		}

		statusMsg += fmt.Sprintf("\n🌐 *Dashboard*: `%s`\n", s.tunnels.GetURL("dashboard"))

		bot.SendMessage(chatID, statusMsg)

	case "/help":
		helpMsg := "🤖 *Hermit Agent Commands*\n\n"
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

	// Inject current time with timezone offset into user message
	timeOffset, _ := s.db.GetSetting("time_offset")
	offsetHours := 0
	if timeOffset != "" {
		fmt.Sscanf(timeOffset, "%d", &offsetHours)
	}
	currentTime := time.Now().Add(time.Duration(offsetHours) * time.Hour)
	formattedTime := currentTime.Format("2006-01-02 15:04:05")
	userTextWithTime := fmt.Sprintf("[Current time: %s] %s", formattedTime, userText)

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
		s.db.AddHistory(agent.ID, "system", "system", "Error: LLM client not configured")
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
		s.db.AddHistory(agent.ID, "system", "system", "LLM Error: "+err.Error())
		s.db.LogAction(agent.ID, "agent", "llm_error", fmt.Sprintf("Error: %v", err))
		return
	}

	// Log LLM response
	s.db.LogAction(agent.ID, "agent", "llm_response", fmt.Sprintf("Response: %.200s...", response))

	// Delete thinking message
	if thinkingMsgID != "" {
		tempBot.DeleteMessage(chatID, thinkingMsgID)
	}

	// Log agent response (Full trace)
	s.db.AddHistory(agent.ID, "assistant", "assistant", response)

	// Execute the XML actions from the response
	feedback := s.ExecuteXMLPayload(agent.ID, chatID, response, tempBot)

	// Commit with <end> and feedback
	if len(feedback) > 0 {
		feedbackJSON, _ := json.Marshal(feedback)
		s.db.AddHistory(agent.ID, "system", "system", string(feedbackJSON)+"\n<end>")
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

	// 4. Handle <give> tag - Send file to user
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

				err = bot.SendDocument(chatID, tmpPath, "Requested file: "+action.Value)
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
				htmlContent = "<!DOCTYPE html><html><head><title>" + app.Name + "</title></head><body><h1>" + app.Name + "</h1></body></html>"
			}

			// Build index.html with embedded CSS and JS if provided
			indexHTML := htmlContent
			if app.CSS != "" {
				indexHTML = strings.Replace(indexHTML, "</head>", "<style>"+app.CSS+"</style></head>", 1)
			}
			if app.JS != "" {
				indexHTML = strings.Replace(indexHTML, "</body>", "<script>"+app.JS+"</script></body>", 1)
			}

			// Create folder and files
			mkdirCmd := "mkdir -p " + appFolder
			s.docker.Exec(containerName, mkdirCmd)

			// Write index.html
			writeIndexCmd := fmt.Sprintf("echo '%s' > %s/index.html", strings.ReplaceAll(indexHTML, "'", "'\\''"), appFolder)
			s.docker.Exec(containerName, writeIndexCmd)

			// Log the action
			s.db.LogAction(agentID, "system", "action_app_created", fmt.Sprintf("App: %s created", app.Name))

			// Generate public URL
			publicURL := s.tunnels.GetURL("dashboard") + fmt.Sprintf("/api/apps/%d/%s", agentID, app.Name)
			bot.SendMessage(chatID, "🚀 App Created: "+app.Name+"\n\nAccess it here: "+publicURL)

			feedback = append(feedback, map[string]interface{}{
				"action": "APP",
				"app":    app.Name,
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
					s.db.AddHistory(agentID, "system", "system", "Skill loaded ["+skillName+"]:\n\n"+string(content)+"\n<end>")
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
		feedback = append(feedback, map[string]interface{}{"system": "time", "value": time.Now().Format(time.RFC3339)})
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
				s.db.AddHistory(agentID, "system", "system", eventList+"\n<end>")
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
					s.db.AddHistory(agentID, "system", "system", fmt.Sprintf("Calendar Event Deleted: ID %d\n<end>", eventID))
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
			// Parse DateTime into date and time components
			dateTime := cal.DateTime
			var dateStr, timeStr string

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
	// Execute the actions
	feedback := s.ExecuteXMLPayload(agentId, chatID, xmlInput, bot)

	// Log feedback and add <end>
	feedbackJSON, _ := json.Marshal(feedback)
	s.db.AddHistory(agentId, "system", "system", string(feedbackJSON)+"\n<end>")
}
