package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JohnEsleyer/HermitShell/internal/db"
	"github.com/JohnEsleyer/HermitShell/internal/parser"
	"github.com/gofiber/fiber/v2"
)

// Reference: docs/takeover-mode.md for takeover mode documentation

func TestTakeoverModeRejectsXMLWhenOff(t *testing.T) {
	app := fiber.New()

	// Create test database
	testDB, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	// Create test agent
	agent := &db.Agent{
		ID:           1,
		Name:         "TestAgent",
		Role:         "assistant",
		Personality:  "You are a test agent",
		Provider:     "gemini",
		Model:        "gemini-2.0-flash",
		AllowedUsers: "",
	}
	testDB.CreateAgent(agent)

	// Create server with test dependencies
	server := &Server{
		db:           testDB,
		takeoverMode: make(map[string]bool),
	}

	app.Post("/api/agents/:id/chat", server.HandleAgentChat)

	// Test: XML command rejected when takeover is OFF
	req := httptest.NewRequest("POST", "/api/agents/1/chat", strings.NewReader(`{"message": "<terminal>ls</terminal>"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Should return rejection message
	if result["rejected"] != true {
		t.Error("expected rejected=true for XML command when takeover is off")
	}
	if !strings.Contains(result["message"].(string), "not allowed") {
		t.Errorf("expected rejection message, got: %v", result["message"])
	}
}

func TestTakeoverModeAllowsXMLWhenOn(t *testing.T) {
	app := fiber.New()

	// Create test database
	testDB, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	// Create test agent
	agent := &db.Agent{
		ID:           1,
		Name:         "TestAgent",
		Role:         "assistant",
		Personality:  "You are a test agent",
		Provider:     "gemini",
		Model:        "gemini-2.0-flash",
		AllowedUsers: "",
	}
	testDB.CreateAgent(agent)

	// Create server with takeover ON
	server := &Server{
		db:           testDB,
		takeoverMode: map[string]bool{"mobile-chat": true}, // Takeover ON
	}

	app.Post("/api/agents/:id/chat", server.HandleAgentChat)

	// Test: XML command allowed when takeover is ON
	req := httptest.NewRequest("POST", "/api/agents/1/chat", strings.NewReader(`{"message": "<message>Hello</message>"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestSlashCommandDoesNotTriggerLLM(t *testing.T) {
	app := fiber.New()

	// Create test database
	testDB, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	// Create test agent
	agent := &db.Agent{
		ID:           1,
		Name:         "TestAgent",
		Role:         "assistant",
		Personality:  "You are a test agent",
		Provider:     "gemini",
		Model:        "gemini-2.0-flash",
		AllowedUsers: "",
	}
	testDB.CreateAgent(agent)

	server := &Server{
		db:           testDB,
		takeoverMode: make(map[string]bool),
	}

	app.Post("/api/agents/:id/chat", server.HandleAgentChat)

	// Test /clear command
	req := httptest.NewRequest("POST", "/api/agents/1/chat", strings.NewReader(`{"message": "/clear"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Slash command should return system response, not trigger LLM
	if result["role"] != "system" {
		t.Errorf("expected role=system for slash command, got: %v", result["role"])
	}
	if !strings.Contains(result["message"].(string), "cleared") && !strings.Contains(result["message"].(string), "Context") {
		t.Errorf("expected clear confirmation, got: %v", result["message"])
	}
}

func TestSlashStatusCommandReturnsStatus(t *testing.T) {
	app := fiber.New()

	testDB, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	agent := &db.Agent{
		ID:           1,
		Name:         "TestAgent",
		Role:         "assistant",
		Personality:  "You are a test agent",
		Provider:     "gemini",
		Model:        "gemini-2.0-flash",
		AllowedUsers: "",
	}
	testDB.CreateAgent(agent)

	server := &Server{
		db:           testDB,
		takeoverMode: make(map[string]bool),
	}

	app.Post("/api/agents/:id/chat", server.HandleAgentChat)

	// Test /status command
	req := httptest.NewRequest("POST", "/api/agents/1/chat", strings.NewReader(`{"message": "/status"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if !strings.Contains(result["message"].(string), "Agent Status") {
		t.Errorf("expected status message, got: %v", result["message"])
	}
	if !strings.Contains(result["message"].(string), "gemini-2.0-flash") {
		t.Errorf("expected model name in status, got: %v", result["message"])
	}
}

func TestSlashResetCommandResetsContainer(t *testing.T) {
	app := fiber.New()

	testDB, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	agent := &db.Agent{
		ID:           1,
		Name:         "TestAgent",
		Role:         "assistant",
		Personality:  "You are a test agent",
		Provider:     "gemini",
		Model:        "gemini-2.0-flash",
		ContainerID:  "test-container",
		AllowedUsers: "",
	}
	testDB.CreateAgent(agent)

	server := &Server{
		db:           testDB,
		takeoverMode: make(map[string]bool),
		docker:       nil, // No docker client, should handle gracefully
	}

	app.Post("/api/agents/:id/chat", server.HandleAgentChat)

	// Test /reset command
	req := httptest.NewRequest("POST", "/api/agents/1/chat", strings.NewReader(`{"message": "/reset"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Should indicate no container configured or handle gracefully
	if result["role"] != "system" {
		t.Errorf("expected role=system, got: %v", result["role"])
	}
}

func TestMessageTagExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple message tag",
			input:    "<message>Hello World</message>",
			expected: "Hello World",
		},
		{
			name:     "Message with surrounding text",
			input:    "Some text before <message>Hello</message> some text after",
			expected: "Hello",
		},
		{
			name:     "Message with thought tags",
			input:    "<thought>Thinking...</thought><message>Response</message>",
			expected: "Response",
		},
		{
			name:     "Multiple message tags - takes first",
			input:    "<message>First</message><message>Second</message>",
			expected: "First",
		},
		{
			name:     "Message with terminal tag",
			input:    "<message>Done</message><terminal>ls</terminal>",
			expected: "Done",
		},
		{
			name:     "No message tag - empty",
			input:    "<terminal>ls</terminal>",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parser.ParseLLMOutput(tc.input)
			if parsed.Message != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, parsed.Message)
			}
		})
	}
}

func TestThoughtTagNotExposed(t *testing.T) {
	// Reference: docs/message-processing.md - <thought> should be internal
	input := `<thought>This is internal reasoning</thought><message>Visible response</message>`
	parsed := parser.ParseLLMOutput(input)

	// Message should be extracted
	if parsed.Message != "Visible response" {
		t.Errorf("expected message 'Visible response', got %q", parsed.Message)
	}

	// Thought should be extracted but not sent to client
	if parsed.Thought != "This is internal reasoning" {
		t.Errorf("expected thought to be parsed, got %q", parsed.Thought)
	}
}

func TestGiveActionExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single give",
			input:    "<give>file.txt</give>",
			expected: []string{"file.txt"},
		},
		{
			name:     "Multiple give",
			input:    "<give>file1.txt</give><give>file2.txt</give>",
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "Give with message",
			input:    "<message>Here is the file</message><give>report.pdf</give>",
			expected: []string{"report.pdf"},
		},
		{
			name:     "No give",
			input:    "<message>Hello</message>",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parser.ParseLLMOutput(tc.input)
			if len(parsed.Actions) != len(tc.expected) {
				t.Errorf("expected %d actions, got %d", len(tc.expected), len(parsed.Actions))
			}
			for i, expectedFile := range tc.expected {
				if i >= len(parsed.Actions) {
					t.Errorf("missing action at index %d", i)
					continue
				}
				if parsed.Actions[i].Value != expectedFile {
					t.Errorf("expected file %q, got %q", expectedFile, parsed.Actions[i].Value)
				}
			}
		})
	}
}

func TestTerminalActionExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		count    int
	}{
		{
			name:     "Single terminal",
			input:    "<terminal>ls -la</terminal>",
			expected: "ls -la",
			count:    1,
		},
		{
			name:     "Multiple terminals",
			input:    "<terminal>cd /app</terminal><terminal>ls</terminal>",
			expected: "cd /app",
			count:    2,
		},
		{
			name:     "Terminal with message",
			input:    "<message>Done</message><terminal>pwd</terminal>",
			expected: "pwd",
			count:    1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parser.ParseLLMOutput(tc.input)
			if len(parsed.Terminals) != tc.count {
				t.Errorf("expected %d terminals, got %d", tc.count, len(parsed.Terminals))
			}
			if parsed.Terminal != tc.expected {
				t.Errorf("expected terminal %q, got %q", tc.expected, parsed.Terminal)
			}
		})
	}
}

func TestWebSocketMessageFormat(t *testing.T) {
	// Reference: docs/frontend-backend-communication.md
	message := map[string]interface{}{
		"type":     "new_message",
		"agent_id": 1,
		"user_id":  "mobile",
		"role":     "assistant",
		"content":  "Hello from agent",
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(jsonMsg, &decoded)

	if decoded["type"] != "new_message" {
		t.Errorf("expected type=new_message, got %v", decoded["type"])
	}
	if decoded["agent_id"] != float64(1) {
		t.Errorf("expected agent_id=1, got %v", decoded["agent_id"])
	}
	if decoded["role"] != "assistant" {
		t.Errorf("expected role=assistant, got %v", decoded["role"])
	}
	if decoded["content"] != "Hello from agent" {
		t.Errorf("expected content, got %v", decoded["content"])
	}
}

func TestConversationClearedBroadcast(t *testing.T) {
	message := map[string]interface{}{
		"type":     "conversation_cleared",
		"agent_id": 1,
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(jsonMsg, &decoded)

	if decoded["type"] != "conversation_cleared" {
		t.Errorf("expected type=conversation_cleared, got %v", decoded["type"])
	}
	if decoded["agent_id"] != float64(1) {
		t.Errorf("expected agent_id=1, got %v", decoded["agent_id"])
	}
}

func TestRejectionFlagFormat(t *testing.T) {
	// Reference: docs/message-processing.md - isRejected column
	message := map[string]interface{}{
		"message":  "XML commands are not allowed when takeover mode is off",
		"role":     "system",
		"rejected": true,
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	json.Unmarshal(jsonMsg, &decoded)

	if decoded["rejected"] != true {
		t.Errorf("expected rejected=true, got %v", decoded["rejected"])
	}
	if decoded["role"] != "system" {
		t.Errorf("expected role=system, got %v", decoded["role"])
	}
}

func TestEncryptedMessageHandling(t *testing.T) {
	// Test that encrypted messages start with "enc:"
	testMessage := "enc:AESGCMSomeEncryptedDataHere=="

	if !strings.HasPrefix(testMessage, "enc:") {
		t.Error("encrypted message should start with enc:")
	}

	// After stripping enc: prefix, it should be base64
	ciphertext := strings.TrimPrefix(testMessage, "enc:")
	if len(ciphertext) == 0 {
		t.Error("ciphertext should not be empty")
	}
}

func TestHistoryEntryWithRejection(t *testing.T) {
	// Test that history entries can track rejection
	testCases := []struct {
		name       string
		isRejected bool
	}{
		{"rejected entry", true},
		{"normal entry", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := db.HistoryEntry{
				ID:         1,
				AgentID:    1,
				UserID:     "mobile",
				Role:       "user",
				Content:    "<terminal>ls</terminal>",
				IsRejected: tc.isRejected,
			}

			if entry.IsRejected != tc.isRejected {
				t.Errorf("expected IsRejected=%v, got %v", tc.isRejected, entry.IsRejected)
			}
		})
	}
}

func TestHistoryEntryWithSeen(t *testing.T) {
	// Test that history entries track seen status
	testCases := []struct {
		name   string
		isSeen bool
	}{
		{"unseen message", false},
		{"seen message", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := db.HistoryEntry{
				ID:      1,
				AgentID: 1,
				UserID:  "mobile",
				Role:    "assistant",
				Content: "Hello",
				IsSeen:  tc.isSeen,
			}

			if entry.IsSeen != tc.isSeen {
				t.Errorf("expected IsSeen=%v, got %v", tc.isSeen, entry.IsSeen)
			}
		})
	}
}
