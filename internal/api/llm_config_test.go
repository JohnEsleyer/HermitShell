package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/JohnEsleyer/HermitShell/internal/db"
	"github.com/gofiber/fiber/v2"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "hermit-test.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestGetLLMConfigStatusRequiresModelProviderKey(t *testing.T) {
	database := newTestDB(t)
	server := &Server{db: database}

	if err := database.SetSetting("openai_api_key", "sk-test"); err != nil {
		t.Fatalf("set key: %v", err)
	}

	status := server.getLLMConfigStatus(&db.Agent{Provider: "openai", Model: "gpt-4o"})
	if !status.Configured {
		t.Fatalf("expected config to be ready, missing=%v", status.Missing)
	}
	if status.ModelType != "GPT" {
		t.Fatalf("expected GPT model type, got %q", status.ModelType)
	}

	missingModel := server.getLLMConfigStatus(&db.Agent{Provider: "openai", Model: ""})
	if missingModel.Configured {
		t.Fatalf("expected missing model to fail readiness")
	}

	missingKey := server.getLLMConfigStatus(&db.Agent{Provider: "anthropic", Model: "claude-3-5-sonnet"})
	if missingKey.Configured {
		t.Fatalf("expected missing key to fail readiness")
	}
}

func TestHandleSetSettingsPersistsKeys(t *testing.T) {
	database := newTestDB(t)
	server := &Server{db: database}

	app := fiber.New()
	app.Post("/settings", server.HandleSetSettings)

	body, err := json.Marshal(map[string]any{
		"openaiKey":     " sk-openai ",
		"geminiKey":     "gem-key",
		"timezone":      "Asia/Manila",
		"timeOffset":    "8",
		"tunnelEnabled": false,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest("POST", "/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	openaiKey, _ := database.GetSetting("openai_api_key")
	if openaiKey != "sk-openai" {
		t.Fatalf("expected trimmed openai key, got %q", openaiKey)
	}
	geminiKey, _ := database.GetSetting("gemini_api_key")
	if geminiKey != "gem-key" {
		t.Fatalf("expected gemini key, got %q", geminiKey)
	}
}
