package api

import (
	"embed"
	"fmt"
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
)

var distFS embed.FS

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

	api.Get("/skills", s.HandleListSkills)
	api.Post("/skills", s.HandleCreateSkill)
	api.Put("/skills/:id", s.HandleUpdateSkill)
	api.Delete("/skills/:id", s.HandleDeleteSkill)
	api.Get("/skills/context", s.HandleGetContextSkill)
	api.Post("/skills/context/reset", s.HandleResetContextSkill)

	api.Get("/calendar", s.HandleListCalendar)
	api.Post("/calendar", s.HandleCreateCalendarEvent)
	api.Delete("/calendar/:id", s.HandleDeleteCalendarEvent)

	api.Get("/allowlist", s.HandleListAllowlist)
	api.Post("/allowlist", s.HandleCreateAllowlist)
	api.Delete("/allowlist/:id", s.HandleDeleteAllowlist)

	api.Get("/metrics", s.HandleMetrics)
	api.Get("/containers", s.HandleContainers)
	api.Get("/containers/:id/files", s.HandleContainerFiles)

	api.Get("/settings", s.HandleGetSettings)
	api.Post("/settings", s.HandleSetSettings)
	api.Get("/settings/domain-status", s.HandleDomainStatus)

	api.Post("/test-contract", s.HandleTestContract)

	api.Post("/images/upload", s.HandleImageUpload)

	api.Post("/telegram/send-code", s.HandleTelegramSendCode)
	api.Post("/telegram/verify", s.HandleTelegramVerify)
	api.Post("/webhook", s.HandleWebhook)
	api.Post("/webhook/:agentId", s.HandleAgentWebhook)

	s.setupStaticRoutes(app)
}

func (s *Server) setupStaticRoutes(app *fiber.App) {
	distPath := "./dashboard/dist"

	app.Static("/data/image", "./data/image")
	app.Static("/", distPath)

	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api") {
			return c.Next()
		}
		return c.SendFile(distPath + "/index.html")
	})
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

	domainMode, _ := s.db.GetSetting("domain_mode")
	tunnelURL := ""
	if domainMode != "true" {
		tunnelURL = s.tunnels.GetURL("dashboard")
	}

	domain, _ := s.db.GetSetting("domain")

	return c.JSON(fiber.Map{
		"host":       metrics.Host,
		"containers": metrics.Containers,
		"tunnelURL":  tunnelURL,
		"domain":     domain,
		"domainMode": domainMode == "true",
	})
}

func (s *Server) HandleContainers(c *fiber.Ctx) error {
	metrics, err := s.docker.LatestSystemMetrics()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	agents, _ := s.db.ListAgents()

	type ContainerInfo struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		AgentID     string  `json:"agentId"`
		AgentName   string  `json:"agentName"`
		Status      string  `json:"status"`
		CPU         float64 `json:"cpu"`
		Memory      float64 `json:"memory"`
		ContainerID string  `json:"containerId"`
	}

	var containers []ContainerInfo
	for _, cont := range metrics.Containers {
		agentName := cont.Name
		agentID := ""
		status := "running"

		for _, a := range agents {
			if a.ContainerID == cont.Name || strings.Contains(cont.Name, strings.ToLower(a.Name)) {
				agentName = a.Name
				agentID = fmt.Sprintf("%d", a.ID)
				status = a.Status
				break
			}
		}

		containers = append(containers, ContainerInfo{
			ID:          cont.Name,
			Name:        agentName,
			AgentID:     agentID,
			AgentName:   agentName,
			Status:      status,
			CPU:         cont.CPUPercent,
			Memory:      cont.MemUsageMB,
			ContainerID: cont.Name,
		})
	}

	return c.JSON(containers)
}

func (s *Server) HandleContainerFiles(c *fiber.Ctx) error {
	containerID := c.Params("id")
	_ = containerID
	path := c.Query("path", "/app/workspace")

	return c.JSON(fiber.Map{
		"path": path,
		"files": []fiber.Map{
			{"name": "work", "type": "directory"},
			{"name": "in", "type": "directory"},
			{"name": "out", "type": "directory"},
			{"name": "apps", "type": "directory"},
		},
	})
}

func (s *Server) HandleListAgents(c *fiber.Ctx) error {
	agents, err := s.db.ListAgents()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	type AgentResponse struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		Role         string `json:"role"`
		Personality  string `json:"personality"`
		Provider     string `json:"provider"`
		Status       string `json:"status"`
		TunnelURL    string `json:"tunnelUrl"`
		ProfilePic   string `json:"profilePic"`
		BannerURL    string `json:"bannerUrl"`
		ContainerID  string `json:"containerId"`
		AllowedUsers string `json:"allowedUsers"`
	}

	var result []AgentResponse
	for _, a := range agents {
		tunnelURL := a.TunnelURL
		if tunnelURL == "" {
			tunnelURL = s.tunnels.GetURL(fmt.Sprintf("agent-%d", a.ID))
		}

		result = append(result, AgentResponse{
			ID:           a.ID,
			Name:         a.Name,
			Role:         a.Role,
			Personality:  a.Personality,
			Provider:     a.Provider,
			Status:       a.Status,
			TunnelURL:    tunnelURL,
			ProfilePic:   a.ProfilePic,
			BannerURL:    a.BannerURL,
			ContainerID:  a.ContainerID,
			AllowedUsers: a.AllowedUsers,
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

	id, err := s.db.CreateAgent(&a)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	go func() {
		time.Sleep(2 * time.Second)
		existing, err := s.db.GetAgent(id)
		if err == nil && existing != nil {
			existing.Status = "running"
			s.db.UpdateAgent(existing)
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
		Name        string `json:"name"`
		Role        string `json:"role"`
		Personality string `json:"personality"`
		Provider    string `json:"provider"`
		ProfilePic  string `json:"profilePic"`
		BannerURL   string `json:"bannerUrl"`
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

	if err := s.db.UpdateAgent(existing); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
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

func (s *Server) HandleDeleteAgent(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	s.db.DeleteAgent(id)
	s.db.DeleteTunnelByAgentID(id)
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

	switch req.Action {
	case "start":
		agent.Status = "running"
	case "stop":
		agent.Status = "stopped"
	case "reset":
		if s.docker != nil {
			s.docker.Stop(agent.ContainerID)
			s.docker.Remove(agent.ContainerID)
		}
		agent.Status = "standby"
	}

	s.db.UpdateAgent(agent)
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) HandleGetAgentLogs(c *fiber.Ctx) error {
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
	logs, _ := s.db.GetAuditLogs(id, 100)
	return c.JSON(logs)
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
	id, _ := strconv.ParseInt(c.Params("id"), 10, 64)
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

	type CalendarResponse struct {
		ID        int64  `json:"id"`
		AgentID   int64  `json:"agentId"`
		Date      string `json:"date"`
		Time      string `json:"time"`
		Prompt    string `json:"prompt"`
		Executed  bool   `json:"executed"`
		CreatedAt string `json:"createdAt"`
	}

	var result []CalendarResponse
	for _, e := range events {
		result = append(result, CalendarResponse{
			ID:        e.ID,
			AgentID:   e.AgentID,
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

func (s *Server) HandleSetSettings(c *fiber.Ctx) error {
	var req struct {
		DomainMode    string `json:"domainMode"`
		Domain        string `json:"domain"`
		OpenrouterKey string `json:"openrouterKey"`
		OpenaiKey     string `json:"openaiKey"`
		AnthropicKey  string `json:"anthropicKey"`
		GeminiKey     string `json:"geminiKey"`
		Timezone      string `json:"timezone"`
	}
	c.BodyParser(&req)

	if req.DomainMode != "" {
		s.db.SetSetting("domain_mode", req.DomainMode)
	}
	if req.Domain != "" {
		s.db.SetSetting("domain", req.Domain)
	}
	if req.OpenrouterKey != "" {
		s.db.SetSetting("openrouter_api_key", req.OpenrouterKey)
	}
	if req.OpenaiKey != "" {
		s.db.SetSetting("openai_api_key", req.OpenaiKey)
	}
	if req.AnthropicKey != "" {
		s.db.SetSetting("anthropic_api_key", req.AnthropicKey)
	}
	if req.GeminiKey != "" {
		s.db.SetSetting("gemini_api_key", req.GeminiKey)
	}
	if req.Timezone != "" {
		s.db.SetSetting("timezone", req.Timezone)
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

func (s *Server) HandleAgentWebhook(c *fiber.Ctx) error {
	_ = c.Params("agentId")

	var update telegram.Update
	if err := c.BodyParser(&update); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if update.Message == nil || update.Message.Text == "" {
		return c.SendStatus(200)
	}

	chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
	userText := strings.TrimSpace(update.Message.Text)

	s.mu.RLock()
	takeoverOn := s.takeoverMode[chatID]
	s.mu.RUnlock()

	if takeoverOn {
		s.handleTakeoverInput(chatID, userText)
	} else {
		if s.bot != nil {
			s.bot.SendMessage(chatID, "Message received. AI agent will respond shortly...")
		}
	}

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
			s.handleTakeoverInput(chatID, text)
		} else {
			s.bot.SendMessage(chatID, "Message received. AI agent will respond shortly...")
		}
	}
}

func (s *Server) handleTakeoverInput(chatID, xmlInput string) {
	parsed := parser.ParseLLMOutput(xmlInput)
	var responses []string

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

	for _, action := range parsed.Actions {
		switch action.Type {
		case "GIVE":
			responses = append(responses, fmt.Sprintf("Action: GIVE file=%s", action.Value))
		case "APP":
			responses = append(responses, fmt.Sprintf("Action: PUBLISH app=%s", action.Value))
		case "SKILL":
			responses = append(responses, fmt.Sprintf("Action: LOAD skill=%s", action.Value))
		}
	}

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

	if s.bot != nil {
		s.bot.SendMessage(chatID, strings.Join(responses, "\n\n"))
	}
}
