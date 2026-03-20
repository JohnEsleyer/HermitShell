package api

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JohnEsleyer/HermitShell/internal/db"
	"github.com/JohnEsleyer/HermitShell/internal/telegram"
)

// Reference: docs/xml-tags.md. These fixtures keep the Test Console transport checks deterministic.
var (
	//go:embed testdata/hermitchat-test-image.jpg
	testConsoleImage []byte
	//go:embed testdata/hermitchat-test-video.mp4
	testConsoleVideo []byte
)

const (
	testConsoleTextFile  = "hermitchat-test.txt"
	testConsoleImageFile = "hermitchat-test-image.jpg"
	testConsoleVideoFile = "hermitchat-test-video.mp4"
)

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func isVideoFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mov", ".webm", ".m4v":
		return true
	default:
		return false
	}
}

func (s *Server) broadcastConversationCleared(agentID int64) {
	payload := map[string]interface{}{
		"type":     "conversation_cleared",
		"agent_id": agentID,
	}
	if jsonMsg, err := json.Marshal(payload); err == nil {
		s.BroadcastMessage(string(jsonMsg))
	}
}

func (s *Server) addHistoryAndBroadcastWithFiles(agentID int64, userID, role, content string, files []string) {
	s.db.AddHistory(agentID, userID, role, content)

	payload := map[string]interface{}{
		"type":     "new_message",
		"agent_id": agentID,
		"user_id":  userID,
		"role":     role,
		"content":  content,
	}
	if len(files) > 0 {
		payload["files"] = files
	}
	if jsonMsg, err := json.Marshal(payload); err == nil {
		s.BroadcastMessage(string(jsonMsg))
	}
}

func (s *Server) sendTransportFile(bot *telegram.Bot, chatID, filePath, displayName string) error {
	if bot == nil {
		return nil
	}
	caption := "Requested file: " + displayName
	if isImageFile(displayName) {
		return bot.SendPhoto(chatID, filePath, caption)
	}
	if isVideoFile(displayName) {
		return bot.SendVideo(chatID, filePath, caption)
	}
	return bot.SendDocument(chatID, filePath, caption)
}

func (s *Server) seedConsoleAsset(containerName, fileName string, content []byte) error {
	encoded := base64.StdEncoding.EncodeToString(content)
	targetPath := "/app/workspace/out/" + fileName
	cmd := fmt.Sprintf("mkdir -p /app/workspace/out && printf '%%s' '%s' | base64 -d > '%s'", encoded, targetPath)
	_, err := s.docker.Exec(containerName, cmd)
	return err
}

func (s *Server) ensureConsoleTestAssets(agent *db.Agent) error {
	containerName, err := s.ensureAgentContainer(agent)
	if err != nil {
		return err
	}
	textPayload := []byte("HermitChat Test Console fixture.\nReference: docs/xml-tags.md\n")
	if err := s.seedConsoleAsset(containerName, testConsoleTextFile, textPayload); err != nil {
		return err
	}
	if err := s.seedConsoleAsset(containerName, testConsoleImageFile, testConsoleImage); err != nil {
		return err
	}
	if err := s.seedConsoleAsset(containerName, testConsoleVideoFile, testConsoleVideo); err != nil {
		return err
	}
	return nil
}
