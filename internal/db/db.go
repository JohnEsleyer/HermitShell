// Package db provides database operations for Hermit.
//
// Documentation:
// - authentication.md: User table, password hashing, session management
// - logging.md: Audit logs (audit_logs table)
// - container-management.md: Agents table, container_id tracking
// - security-measures.md: Allowlist and access control
package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/JohnEsleyer/HermitShell/internal/crypto"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db        *sql.DB
	cryptoKey []byte
}

type Agent struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Role          string `json:"role"`
	Personality   string `json:"personality"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Context       string `json:"context"`
	TelegramID    string `json:"telegram_id"`
	TelegramToken string `json:"telegram_token"`
	ProfilePic    string `json:"profile_pic"`
	BannerURL     string `json:"banner_url"`
	TunnelID      string `json:"tunnel_id"`
	TunnelURL     string `json:"tunnel_url"`
	AllowedUsers  string `json:"allowed_users"`
	ContainerID   string `json:"container_id"`
	Status        string `json:"status"`
	Platform      string `json:"platform"` // "telegram" or "hermitchat"
	Active        bool   `json:"active"`
	LLMAPICalls   int64  `json:"llm_api_calls"`  // Total LLM API calls
	ContextWindow int    `json:"context_window"` // Context window size in tokens
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type Skill struct {
	ID          int64  `json:"id"`
	AgentID     int64  `json:"agent_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
}

type AuditLog struct {
	ID        int64
	AgentID   int64
	UserID    string
	Action    string
	Details   string
	CreatedAt string
}

type HistoryEntry struct {
	ID        int64  `json:"id"`
	AgentID   int64  `json:"agent_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func NewDB(path string) (*DB, error) {
	connStr := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", path)
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	d := &DB{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) SetCryptoKey(key []byte) {
	d.cryptoKey = key
}

// GetSystemTime returns the current time based on the global UTC offset setting.
func (d *DB) GetSystemTime() time.Time {
	offsetStr, _ := d.GetSetting("time_offset")
	offsetHours, _ := strconv.Atoi(offsetStr)
	return time.Now().UTC().Add(time.Duration(offsetHours) * time.Hour)
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		role TEXT NOT NULL DEFAULT 'assistant',
		personality TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT 'openrouter',
		model TEXT NOT NULL DEFAULT 'openai/gpt-4',
		system_prompt TEXT NOT NULL DEFAULT '',
		context TEXT NOT NULL DEFAULT '',
		telegram_id TEXT NOT NULL DEFAULT '',
		telegram_token TEXT NOT NULL DEFAULT '',
		profile_pic TEXT NOT NULL DEFAULT '',
		banner_url TEXT NOT NULL DEFAULT '',
		tunnel_id TEXT NOT NULL DEFAULT '',
		tunnel_url TEXT NOT NULL DEFAULT '',
		allowed_users TEXT NOT NULL DEFAULT '',
		container_id TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'stopped',
		platform TEXT NOT NULL DEFAULT 'telegram',
		active INTEGER NOT NULL DEFAULT 1,
		llm_api_calls INTEGER NOT NULL DEFAULT 0,
		context_window INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		action TEXT NOT NULL,
		details TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS skills (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL,
		is_core INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS credentials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		provider TEXT NOT NULL,
		api_key TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		must_change_password INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS allowlist (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_user_id TEXT NOT NULL,
		friendly_name TEXT NOT NULL DEFAULT '',
		notes TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS calendar (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		date TEXT NOT NULL,
		time TEXT NOT NULL,
		prompt TEXT NOT NULL,
		executed INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);

	CREATE TABLE IF NOT EXISTS tunnels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		tunnel_uuid TEXT NOT NULL,
		tunnel_name TEXT NOT NULL,
		public_hostname TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'inactive',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		last_seen TEXT NOT NULL DEFAULT (datetime('now')),
		FOREIGN KEY(agent_id) REFERENCES agents(id)
	);
	`

	if _, err := d.db.Exec(schema); err != nil {
		return err
	}

	// Add new columns to existing databases if they don't exist
	if err := d.addColumnIfNotExists("agents", "llm_api_calls", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("agents", "context_window", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("agents", "banner_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("agents", "platform", "TEXT NOT NULL DEFAULT 'telegram'"); err != nil {
		return err
	}
	if err := d.addColumnIfNotExists("agents", "active", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}

	return nil
}

func (d *DB) addColumnIfNotExists(table, column, def string) error {
	// Check if column exists
	var count int
	err := d.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table), column).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
		return err
	}
	return nil
}

func (d *DB) CreateAgent(a *Agent) (int64, error) {
	res, err := d.db.Exec(`
		INSERT INTO agents (name, role, personality, provider, model, system_prompt, telegram_id, telegram_token, profile_pic, banner_url, tunnel_id, tunnel_url, allowed_users, container_id, status, platform, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.Name, a.Role, a.Personality, a.Provider, a.Model, a.Context, a.TelegramID, a.TelegramToken, a.ProfilePic, a.BannerURL, a.TunnelID, a.TunnelURL, a.AllowedUsers, a.ContainerID, a.Status, a.Platform, a.Active)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func (d *DB) GetAgent(id int64) (*Agent, error) {
	a := &Agent{}
	err := d.db.QueryRow(`
		SELECT id, name, role, personality, provider, model, system_prompt, telegram_id, telegram_token, profile_pic, banner_url, tunnel_id, tunnel_url, allowed_users, container_id, status, platform, active, llm_api_calls, context_window, created_at, updated_at
		FROM agents WHERE id = ?
	`, id).Scan(&a.ID, &a.Name, &a.Role, &a.Personality, &a.Provider, &a.Model, &a.Context, &a.TelegramID, &a.TelegramToken, &a.ProfilePic, &a.BannerURL, &a.TunnelID, &a.TunnelURL, &a.AllowedUsers, &a.ContainerID, &a.Status, &a.Platform, &a.Active, &a.LLMAPICalls, &a.ContextWindow, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (d *DB) GetAgentByName(name string) (*Agent, error) {
	a := &Agent{}
	err := d.db.QueryRow(`
		SELECT id, name, role, personality, provider, model, system_prompt, telegram_id, telegram_token, profile_pic, banner_url, tunnel_id, tunnel_url, allowed_users, container_id, status, platform, active, llm_api_calls, context_window, created_at, updated_at
		FROM agents WHERE name = ?
	`, name).Scan(&a.ID, &a.Name, &a.Role, &a.Personality, &a.Provider, &a.Model, &a.Context, &a.TelegramID, &a.TelegramToken, &a.ProfilePic, &a.BannerURL, &a.TunnelID, &a.TunnelURL, &a.AllowedUsers, &a.ContainerID, &a.Status, &a.Platform, &a.Active, &a.LLMAPICalls, &a.ContextWindow, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (d *DB) ListAgents() ([]*Agent, error) {
	rows, err := d.db.Query(`
		SELECT id, name, role, personality, provider, model, system_prompt, telegram_id, telegram_token, profile_pic, banner_url, tunnel_id, tunnel_url, allowed_users, container_id, status, platform, active, created_at, updated_at
		FROM agents ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		a := &Agent{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Role, &a.Personality, &a.Provider, &a.Model, &a.Context, &a.TelegramID, &a.TelegramToken, &a.ProfilePic, &a.BannerURL, &a.TunnelID, &a.TunnelURL, &a.AllowedUsers, &a.ContainerID, &a.Status, &a.Platform, &a.Active, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (d *DB) UpdateAgent(a *Agent) error {
	_, err := d.db.Exec(`
		UPDATE agents SET name=?, role=?, personality=?, provider=?, model=?, system_prompt=?, telegram_id=?, telegram_token=?, profile_pic=?, banner_url=?, tunnel_id=?, tunnel_url=?, allowed_users=?, container_id=?, status=?, platform=?, active=?, updated_at=datetime('now')
		WHERE id=?
	`, a.Name, a.Role, a.Personality, a.Provider, a.Model, a.Context, a.TelegramID, a.TelegramToken, a.ProfilePic, a.BannerURL, a.TunnelID, a.TunnelURL, a.AllowedUsers, a.ContainerID, a.Status, a.Platform, a.Active, a.ID)
	return err
}

func (d *DB) DeleteAgent(id int64) error {
	_, err := d.db.Exec("DELETE FROM agents WHERE id = ?", id)
	return err
}

func (d *DB) IncrementLLMAPICalls(agentID int64) error {
	_, err := d.db.Exec("UPDATE agents SET llm_api_calls = llm_api_calls + 1 WHERE id = ?", agentID)
	return err
}

func (d *DB) UpdateAgentContextWindow(agentID int64, contextWindow int) error {
	_, err := d.db.Exec("UPDATE agents SET context_window = ? WHERE id = ?", contextWindow, agentID)
	return err
}

func (d *DB) SetSetting(key, value string) error {
	_, err := d.db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')
	`, key, value, value)
	return err
}

func (d *DB) GetSetting(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) InitDefaultUser() error {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash := hashPassword("hermit123")
	_, err = d.db.Exec(`
		INSERT INTO users (username, password_hash, role, must_change_password)
		VALUES (?, ?, 'admin', 1)
	`, "admin", hash)
	return err
}

// VerifyUser authenticates user by comparing password hash.
// Docs: See docs/authentication.md for authentication flow.
// Docs: See docs/security-measures.md for password hashing details.
func (d *DB) VerifyUser(username, password string) (int64, bool, error) {
	var id int64
	var hash string
	var mustChange int
	err := d.db.QueryRow("SELECT id, password_hash, must_change_password FROM users WHERE username = ?", username).Scan(&id, &hash, &mustChange)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if hash != hashPassword(password) {
		return 0, false, nil
	}
	return id, mustChange == 1, nil
}

func (d *DB) ChangePassword(username, newPassword string) error {
	hash := hashPassword(newPassword)
	_, err := d.db.Exec(`
		UPDATE users SET password_hash = ?, must_change_password = 0, updated_at = datetime('now')
		WHERE username = ?
	`, hash, username)
	return err
}

func (d *DB) UpdateCredentials(currentUsername, newUsername, newPassword string) error {
	hash := hashPassword(newPassword)

	_, err := d.db.Exec(`
		UPDATE users
		SET username = ?, password_hash = ?, must_change_password = 0, updated_at = datetime('now')
		WHERE username = ?
	`, newUsername, hash, currentUsername)

	return err
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func (d *DB) GetUserByID(id int64) (string, bool, error) {
	var username string
	var mustChange int
	var err error

	if id == 0 {
		err = fmt.Errorf("invalid id")
	} else {
		err = d.db.QueryRow("SELECT username, must_change_password FROM users WHERE id = ?", id).Scan(&username, &mustChange)
	}

	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return username, mustChange == 1, nil
}

// LogAction inserts an audit log entry for tracking system events.
// Docs: See docs/logging.md for log categories and usage patterns.
func (d *DB) LogAction(agentID int64, userID, action, details string) error {
	_, err := d.db.Exec(`
		INSERT INTO audit_logs (agent_id, user_id, action, details)
		VALUES (?, ?, ?, ?)
	`, agentID, userID, action, details)
	return err
}

func (d *DB) GetAuditLogs(agentID int64, limit int) ([]*AuditLog, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, user_id, action, details, created_at
		FROM audit_logs WHERE agent_id = ? ORDER BY created_at DESC LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		if err := rows.Scan(&l.ID, &l.AgentID, &l.UserID, &l.Action, &l.Details, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (d *DB) GetAllAuditLogs(category string, limit int) ([]*AuditLog, error) {
	query := `
		SELECT id, agent_id, user_id, action, details, created_at
		FROM audit_logs
	`
	var args []interface{}

	if category != "" && category != "all" {
		query += " WHERE action LIKE ?"
		args = append(args, category+"%")
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		if err := rows.Scan(&l.ID, &l.AgentID, &l.UserID, &l.Action, &l.Details, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
func (d *DB) AddHistory(agentID int64, userID, role, content string) error {
	if d.cryptoKey != nil {
		encrypted, err := crypto.Encrypt(content, d.cryptoKey)
		if err == nil {
			content = "enc:" + encrypted
		}
	}
	_, err := d.db.Exec(`
		INSERT INTO history (agent_id, user_id, role, content)
		VALUES (?, ?, ?, ?)
	`, agentID, userID, role, content)
	return err
}

func (d *DB) GetHistory(agentID int64, limit int) ([]*HistoryEntry, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, user_id, role, content, created_at
		FROM history WHERE agent_id = ? ORDER BY id DESC LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*HistoryEntry
	for rows.Next() {
		e := &HistoryEntry{}
		if err := rows.Scan(&e.ID, &e.AgentID, &e.UserID, &e.Role, &e.Content, &e.CreatedAt); err != nil {
			return nil, err
		}
		if d.cryptoKey != nil && len(e.Content) > 4 && e.Content[:4] == "enc:" {
			decrypted, err := crypto.Decrypt(e.Content[4:], d.cryptoKey)
			if err == nil {
				e.Content = decrypted
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) ClearHistory(agentID int64) error {
	_, err := d.db.Exec("DELETE FROM history WHERE agent_id = ?", agentID)
	return err
}

type AllowListEntry struct {
	ID             int64
	TelegramUserID string
	FriendlyName   string
	Notes          string
	CreatedAt      string
}

func (d *DB) CreateAllowListEntry(e *AllowListEntry) (int64, error) {
	res, err := d.db.Exec(`
		INSERT INTO allowlist (telegram_user_id, friendly_name, notes)
		VALUES (?, ?, ?)
	`, e.TelegramUserID, e.FriendlyName, e.Notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetAllowListEntry(id int64) (*AllowListEntry, error) {
	e := &AllowListEntry{}
	err := d.db.QueryRow(`
		SELECT id, telegram_user_id, friendly_name, notes, created_at
		FROM allowlist WHERE id = ?
	`, id).Scan(&e.ID, &e.TelegramUserID, &e.FriendlyName, &e.Notes, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (d *DB) ListAllowList() ([]*AllowListEntry, error) {
	rows, err := d.db.Query(`
		SELECT id, telegram_user_id, friendly_name, notes, created_at
		FROM allowlist ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AllowListEntry
	for rows.Next() {
		e := &AllowListEntry{}
		if err := rows.Scan(&e.ID, &e.TelegramUserID, &e.FriendlyName, &e.Notes, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (d *DB) UpdateAllowListEntry(e *AllowListEntry) error {
	_, err := d.db.Exec(`
		UPDATE allowlist SET telegram_user_id=?, friendly_name=?, notes=?
		WHERE id=?
	`, e.TelegramUserID, e.FriendlyName, e.Notes, e.ID)
	return err
}

func (d *DB) DeleteAllowListEntry(id int64) error {
	_, err := d.db.Exec("DELETE FROM allowlist WHERE id = ?", id)
	return err
}

type CalendarEvent struct {
	ID        int64
	AgentID   int64
	Date      string
	Time      string
	Prompt    string
	Executed  bool
	CreatedAt string
}

func (d *DB) CreateCalendarEvent(e *CalendarEvent) (int64, error) {
	res, err := d.db.Exec(`
		INSERT INTO calendar (agent_id, date, time, prompt, executed)
		VALUES (?, ?, ?, ?, ?)
	`, e.AgentID, e.Date, e.Time, e.Prompt, e.Executed)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetCalendarEvent(id int64) (*CalendarEvent, error) {
	e := &CalendarEvent{}
	var executed int
	err := d.db.QueryRow(`
		SELECT id, agent_id, date, time, prompt, executed, created_at
		FROM calendar WHERE id = ?
	`, id).Scan(&e.ID, &e.AgentID, &e.Date, &e.Time, &e.Prompt, &executed, &e.CreatedAt)
	e.Executed = executed == 1
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (d *DB) ListCalendarEvents() ([]*CalendarEvent, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, date, time, prompt, executed, created_at
		FROM calendar ORDER BY date ASC, time ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*CalendarEvent
	for rows.Next() {
		e := &CalendarEvent{}
		var executed int
		if err := rows.Scan(&e.ID, &e.AgentID, &e.Date, &e.Time, &e.Prompt, &executed, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Executed = executed == 1
		events = append(events, e)
	}
	return events, nil
}

func (d *DB) ListCalendarEventsByAgent(agentID int64) ([]*CalendarEvent, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, date, time, prompt, executed, created_at
		FROM calendar WHERE agent_id = ? ORDER BY date ASC, time ASC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*CalendarEvent
	for rows.Next() {
		e := &CalendarEvent{}
		var executed int
		if err := rows.Scan(&e.ID, &e.AgentID, &e.Date, &e.Time, &e.Prompt, &executed, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Executed = executed == 1
		events = append(events, e)
	}
	return events, nil
}

func (d *DB) GetPendingCalendarEvents() ([]*CalendarEvent, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, date, time, prompt, executed, created_at
		FROM calendar WHERE executed = 0 ORDER BY date ASC, time ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*CalendarEvent
	for rows.Next() {
		e := &CalendarEvent{}
		var executed int
		if err := rows.Scan(&e.ID, &e.AgentID, &e.Date, &e.Time, &e.Prompt, &executed, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Executed = executed == 1
		events = append(events, e)
	}
	return events, nil
}

func (d *DB) MarkCalendarEventExecuted(id int64) error {
	_, err := d.db.Exec("UPDATE calendar SET executed = 1 WHERE id = ?", id)
	return err
}

func (d *DB) DeleteCalendarEvent(id int64) error {
	_, err := d.db.Exec("DELETE FROM calendar WHERE id = ?", id)
	return err
}

func (d *DB) UpdateCalendarEvent(id, agentID int64, date, time, prompt string) error {
	_, err := d.db.Exec(`
		UPDATE calendar SET agent_id = ?, date = ?, time = ?, prompt = ?
		WHERE id = ?
	`, agentID, date, time, prompt, id)
	return err
}

type Tunnel struct {
	ID             int64
	AgentID        int64
	TunnelUUID     string
	TunnelName     string
	PublicHostname string
	Status         string
	CreatedAt      string
	LastSeen       string
}

func (d *DB) CreateTunnel(t *Tunnel) (int64, error) {
	res, err := d.db.Exec(`
		INSERT INTO tunnels (agent_id, tunnel_uuid, tunnel_name, public_hostname, status)
		VALUES (?, ?, ?, ?, ?)
	`, t.AgentID, t.TunnelUUID, t.TunnelName, t.PublicHostname, t.Status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) GetTunnel(id int64) (*Tunnel, error) {
	t := &Tunnel{}
	err := d.db.QueryRow(`
		SELECT id, agent_id, tunnel_uuid, tunnel_name, public_hostname, status, created_at, last_seen
		FROM tunnels WHERE id = ?
	`, id).Scan(&t.ID, &t.AgentID, &t.TunnelUUID, &t.TunnelName, &t.PublicHostname, &t.Status, &t.CreatedAt, &t.LastSeen)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *DB) GetTunnelByUUID(uuid string) (*Tunnel, error) {
	t := &Tunnel{}
	err := d.db.QueryRow(`
		SELECT id, agent_id, tunnel_uuid, tunnel_name, public_hostname, status, created_at, last_seen
		FROM tunnels WHERE tunnel_uuid = ?
	`, uuid).Scan(&t.ID, &t.AgentID, &t.TunnelUUID, &t.TunnelName, &t.PublicHostname, &t.Status, &t.CreatedAt, &t.LastSeen)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *DB) GetTunnelByAgentID(agentID int64) (*Tunnel, error) {
	t := &Tunnel{}
	err := d.db.QueryRow(`
		SELECT id, agent_id, tunnel_uuid, tunnel_name, public_hostname, status, created_at, last_seen
		FROM tunnels WHERE agent_id = ?
	`, agentID).Scan(&t.ID, &t.AgentID, &t.TunnelUUID, &t.TunnelName, &t.PublicHostname, &t.Status, &t.CreatedAt, &t.LastSeen)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *DB) ListTunnels() ([]*Tunnel, error) {
	rows, err := d.db.Query(`
		SELECT id, agent_id, tunnel_uuid, tunnel_name, public_hostname, status, created_at, last_seen
		FROM tunnels ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tunnels []*Tunnel
	for rows.Next() {
		t := &Tunnel{}
		if err := rows.Scan(&t.ID, &t.AgentID, &t.TunnelUUID, &t.TunnelName, &t.PublicHostname, &t.Status, &t.CreatedAt, &t.LastSeen); err != nil {
			return nil, err
		}
		tunnels = append(tunnels, t)
	}
	return tunnels, nil
}

func (d *DB) UpdateTunnelStatus(id int64, status string) error {
	_, err := d.db.Exec(`
		UPDATE tunnels SET status = ?, last_seen = datetime('now')
		WHERE id = ?
	`, status, id)
	return err
}

func (d *DB) DeleteTunnel(id int64) error {
	_, err := d.db.Exec("DELETE FROM tunnels WHERE id = ?", id)
	return err
}

func (d *DB) DeleteTunnelByAgentID(agentID int64) error {
	_, err := d.db.Exec("DELETE FROM tunnels WHERE agent_id = ?", agentID)
	return err
}

func (d *DB) CreateSkill(s *Skill) (int64, error) {
	res, err := d.db.Exec("INSERT INTO skills (agent_id, title, description, content) VALUES (?, ?, ?, ?)", s.AgentID, s.Title, s.Description, s.Content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) ListSkills() ([]*Skill, error) {
	rows, err := d.db.Query("SELECT id, agent_id, title, description, content, created_at FROM skills")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []*Skill
	for rows.Next() {
		s := &Skill{}
		rows.Scan(&s.ID, &s.AgentID, &s.Title, &s.Description, &s.Content, &s.CreatedAt)
		skills = append(skills, s)
	}
	return skills, nil
}

func (d *DB) DeleteSkill(id int64) error {
	_, err := d.db.Exec("DELETE FROM skills WHERE id = ?", id)
	return err
}

func (d *DB) UpdateSkill(s *Skill) error {
	_, err := d.db.Exec("UPDATE skills SET title=?, description=?, content=? WHERE id=?", s.Title, s.Description, s.Content, s.ID)
	return err
}
