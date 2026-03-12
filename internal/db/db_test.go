package db

import (
	"os"
	"testing"
)

func TestNewDB(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hermit-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Error("expected db to not be nil")
	}
}

func TestAgentCRUD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hermit-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	agent := &Agent{
		Name:       "test-agent",
		Role:       "assistant",
		Model:      "openai/gpt-4",
		Context:    "You are a helpful assistant.",
		Provider:   "openrouter",
		TelegramID: "123456789",
		Active:     true,
	}

	id, err := db.CreateAgent(agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	retrieved, err := db.GetAgent(id)
	if err != nil {
		t.Fatalf("failed to get agent: %v", err)
	}
	if retrieved.Name != agent.Name {
		t.Errorf("expected name %s, got %s", agent.Name, retrieved.Name)
	}

	agents, err := db.ListAgents()
	if err != nil {
		t.Fatalf("failed to list agents: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	err = db.DeleteAgent(id)
	if err != nil {
		t.Fatalf("failed to delete agent: %v", err)
	}

	agents, err = db.ListAgents()
	if err != nil {
		t.Fatalf("failed to list agents after delete: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(agents))
	}
}

func TestSettingsCRUD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hermit-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	err = db.SetSetting("test_key", "test_value")
	if err != nil {
		t.Fatalf("failed to set setting: %v", err)
	}

	val, err := db.GetSetting("test_key")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}
	if val != "test_value" {
		t.Errorf("expected test_value, got %s", val)
	}

	val, err = db.GetSetting("nonexistent")
	if err != nil {
		t.Fatalf("failed to get nonexistent setting: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for nonexistent key, got %s", val)
	}
}

func TestAuditLog(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hermit-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	err = db.LogAction(1, "test_user", "terminal", "echo hello")
	if err != nil {
		t.Fatalf("failed to log action: %v", err)
	}

	logs, err := db.GetAuditLogs(1, 10)
	if err != nil {
		t.Fatalf("failed to get audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Action != "terminal" {
		t.Errorf("expected action terminal, got %s", logs[0].Action)
	}
}

func TestUpdateCredentials(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "hermit-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	db, err := NewDB(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.InitDefaultUser(); err != nil {
		t.Fatalf("failed to init default user: %v", err)
	}

	id, mustChange, err := db.VerifyUser("admin", "hermit123")
	if err != nil || id == 0 {
		t.Fatalf("expected default credentials to work, err=%v id=%d", err, id)
	}
	if !mustChange {
		t.Fatalf("expected default user to require password change")
	}

	if err := db.UpdateCredentials("admin", "operator", "new-secret"); err != nil {
		t.Fatalf("failed to update credentials: %v", err)
	}

	oldID, _, err := db.VerifyUser("admin", "hermit123")
	if err != nil {
		t.Fatalf("failed to verify old credentials: %v", err)
	}
	if oldID != 0 {
		t.Fatalf("expected old credentials to fail after update")
	}

	newID, newMustChange, err := db.VerifyUser("operator", "new-secret")
	if err != nil || newID == 0 {
		t.Fatalf("expected new credentials to work, err=%v id=%d", err, newID)
	}
	if newMustChange {
		t.Fatalf("expected credentials update to clear must_change_password")
	}
}
