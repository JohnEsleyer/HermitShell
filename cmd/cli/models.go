package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	apiBase = getEnv("HERMIT_API_BASE", "http://localhost:3000")
	cliUser = getEnv("HERMIT_CLI_USER", "")
	cliPass = getEnv("HERMIT_CLI_PASS", "")
)

type Agent struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Status      string `json:"status"`
	TunnelURL   string `json:"tunnelUrl"`
	ContainerID string `json:"containerId"`
}

type AgentStats struct {
	WordCount     int     `json:"wordCount"`
	TokenEstimate int     `json:"tokenEstimate"`
	ContextWindow int     `json:"contextWindow"`
	HistoryCount  int     `json:"historyCount"`
	EstimatedCost float64 `json:"estimatedCost"`
}

type LoginModel struct {
	username string
	password string
	focused  int
	err      string
	loggedIn bool
	token    string
	loading  bool
	agents   []Agent
	selected int
	view     string
	stats    AgentStats
}

type MainModel struct {
	agents   []Agent
	selected int
	err      string
	quitting bool
	view     string
	stats    AgentStats
	loading  bool
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func NewLoginModel() LoginModel {
	m := LoginModel{focused: 0}
	if cliUser != "" && cliPass != "" {
		m.username = cliUser
		m.password = cliPass
		m.loggedIn = true
	}
	return m
}

func (m LoginModel) Init() tea.Cmd {
	if m.loggedIn {
		return func() tea.Msg { return authMsg{success: true} }
	}
	return nil
}

func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.focused = (m.focused + 1) % 2
		case "enter":
			if m.username == "" || m.password == "" {
				m.err = "Please enter username and password"
				return m, nil
			}
			m.err = ""
			m.loading = true
			return m, login(m.username, m.password)
		}
	case authMsg:
		m.loading = false
		if msg.success {
			m.loggedIn = true
			return m, func() tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} }
		}
		m.err = msg.err
	case tea.WindowSizeMsg:
		if m.loggedIn {
			newM := MainModel{agents: m.agents, selected: m.selected, view: m.view, stats: m.stats}
			return newM, fetchAgents
		}
	}
	return m, nil
}

func (m LoginModel) View() string {
	if m.loggedIn {
		return "Logging in..."
	}

	s := lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	usernameLabel := "Username: "
	passwordLabel := "Password: "
	if m.focused == 0 {
		usernameLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("Username: ")
	}
	if m.focused == 1 {
		passwordLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("Password: ")
	}

	str := `
╭────────────────────────────────────────╮
│          Hermit CLI Login              │
╰────────────────────────────────────────╯

` + usernameLabel + m.username + "\n"
	str += passwordLabel + strings.Repeat("•", len(m.password)) + "\n\n"

	if m.err != "" {
		str += errStyle.Render("✗ "+m.err) + "\n\n"
	}

	str += s.Render("Press Tab to switch fields, Enter to login, Ctrl+C to quit")
	if m.loading {
		str += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render("Logging in...")
	}

	return str
}

type authMsg struct {
	success bool
	err     string
	token   string
}

func login(username, password string) tea.Cmd {
	return func() tea.Msg {
		reqBody := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
		req, _ := http.NewRequest("POST", apiBase+"/api/auth/login",
			strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return authMsg{err: err.Error()}
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if resp.StatusCode != 200 {
			return authMsg{err: "Invalid credentials"}
		}

		return authMsg{success: true}
	}
}

func NewMainModel() MainModel {
	return MainModel{selected: -1, view: "list"}
}

func (m MainModel) Init() tea.Cmd {
	return fetchAgents
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.view == "detail" {
				m.view = "list"
				return m, nil
			}
			return m, tea.Quit
		case "j", "down":
			if m.selected < len(m.agents)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		case "enter":
			if m.selected >= 0 && m.selected < len(m.agents) {
				m.view = "detail"
				m.loading = true
				return m, fetchStats(m.agents[m.selected].ID)
			}
		case "r":
			m.loading = true
			m.agents = nil
			return m, fetchAgents
		case "esc":
			if m.view == "detail" {
				m.view = "list"
			}
		}
	case agentsMsg:
		m.agents = msg
		m.loading = false
		if len(m.agents) > 0 && m.selected == -1 {
			m.selected = 0
		}
	case statsMsg:
		m.stats = msg.stats
		m.loading = false
	}
	return m, nil
}

func (m MainModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render("Loading...")
	}

	if m.view == "detail" {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m MainModel) viewList() string {
	if len(m.agents) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("No agents found. Press r to refresh.")
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36")).Render("╭─ Agents ─╮\n")

	for i, agent := range m.agents {
		sel := "  "
		if i == m.selected {
			sel = "► "
		}

		statusColor := "245"
		if agent.Status == "running" {
			statusColor = "46"
		} else if agent.Status == "standby" {
			statusColor = "226"
		}

		status := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render("●") + " " + agent.Status
		title += sel + lipgloss.NewStyle().Bold(true).Render(agent.Name) + " " + status + "\n"
		title += "   " + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(agent.Role+" | "+agent.Model) + "\n"
	}

	title += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("↑↓ Navigate | Enter View | r Refresh | q Quit")
	return title
}

func (m MainModel) viewDetail() string {
	if m.selected < 0 || m.selected >= len(m.agents) {
		return "No agent selected"
	}

	agent := m.agents[m.selected]

	usagePercent := 0
	if m.stats.ContextWindow > 0 {
		usagePercent = (m.stats.TokenEstimate * 100) / m.stats.ContextWindow
	}

	barLen := 30
	filled := (barLen * usagePercent) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("36")).Render("╭─ " + agent.Name + " ─╮\n")
	title += "\n  " + lipgloss.NewStyle().Bold(true).Render("Role:") + " " + agent.Role + "\n"
	title += "  " + lipgloss.NewStyle().Bold(true).Render("Provider:") + " " + agent.Provider + "\n"
	title += "  " + lipgloss.NewStyle().Bold(true).Render("Model:") + " " + agent.Model + "\n"
	title += "  " + lipgloss.NewStyle().Bold(true).Render("Status:") + " " + agent.Status + "\n"
	title += "  " + lipgloss.NewStyle().Bold(true).Render("Tunnel:") + " " + agent.TunnelURL + "\n"

	title += "\n  " + lipgloss.NewStyle().Bold(true).Render("Context:") + "\n"
	title += "    Tokens: " + formatNumber(m.stats.TokenEstimate) + " / " + formatNumber(m.stats.ContextWindow) + "\n"
	title += "    [" + bar + "] " + fmt.Sprintf("%d%%\n", usagePercent)
	title += "    Words: " + formatNumber(m.stats.WordCount) + "\n"
	title += "    Messages: " + fmt.Sprintf("%d", m.stats.HistoryCount) + "\n"

	if m.stats.EstimatedCost > 0 {
		title += "    Est. Cost: $" + fmt.Sprintf("%.4f", m.stats.EstimatedCost) + "\n"
	}

	title += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("← Back | r Refresh | q Quit")
	return title
}

func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

type agentsMsg []Agent
type statsMsg struct {
	stats AgentStats
	id    int
}

func fetchAgents() tea.Msg {
	req, _ := http.NewRequest("GET", apiBase+"/api/agents", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return agentsMsg{}
	}
	defer resp.Body.Close()

	var agents []Agent
	json.NewDecoder(resp.Body).Decode(&agents)
	return agentsMsg(agents)
}

func fetchStats(agentID int) tea.Cmd {
	return func() tea.Msg {
		req, _ := http.NewRequest("GET", apiBase+fmt.Sprintf("/api/agents/%d/stats", agentID), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return statsMsg{}
		}
		defer resp.Body.Close()

		var stats AgentStats
		json.NewDecoder(resp.Body).Decode(&stats)
		return statsMsg{stats: stats, id: agentID}
	}
}
