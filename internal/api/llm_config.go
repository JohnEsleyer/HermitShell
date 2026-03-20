package api

import (
	"fmt"
	"strings"

	"github.com/JohnEsleyer/HermitShell/internal/db"
)

type llmConfigStatus struct {
	Provider   string
	ProviderUI string
	Model      string
	ModelType  string
	APIKeySet  bool
	Configured bool
	Missing    []string
}

func normalizeLLMProvider(provider string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return "openrouter"
	}
	return provider
}

func llmProviderLabel(provider string) string {
	switch normalizeLLMProvider(provider) {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "gemini":
		return "Gemini"
	case "openrouter":
		return "OpenRouter"
	default:
		return strings.TrimSpace(provider)
	}
}

func llmProviderSettingKey(provider string) string {
	switch normalizeLLMProvider(provider) {
	case "openai":
		return "openai_api_key"
	case "anthropic":
		return "anthropic_api_key"
	case "gemini":
		return "gemini_api_key"
	default:
		return "openrouter_api_key"
	}
}

func inferModelType(provider, model string) string {
	provider = normalizeLLMProvider(provider)
	model = strings.TrimSpace(strings.ToLower(model))
	if model == "" {
		return "Not set"
	}

	switch {
	case strings.HasPrefix(model, "openai/"):
		return "OpenAI via OpenRouter"
	case strings.HasPrefix(model, "anthropic/"):
		return "Anthropic via OpenRouter"
	case strings.HasPrefix(model, "google/"):
		return "Google via OpenRouter"
	case strings.Contains(model, "gpt"):
		if provider == "openrouter" {
			return "GPT via OpenRouter"
		}
		return "GPT"
	case strings.Contains(model, "claude"):
		if provider == "openrouter" {
			return "Claude via OpenRouter"
		}
		return "Claude"
	case strings.Contains(model, "gemini"):
		if provider == "openrouter" {
			return "Gemini via OpenRouter"
		}
		return "Gemini"
	case strings.Contains(model, "llama"):
		if provider == "openrouter" {
			return "Llama via OpenRouter"
		}
		return "Llama"
	default:
		return llmProviderLabel(provider)
	}
}

// getLLMConfigStatus centralizes the same readiness rules used by /status and chat execution.
// Reference: docs/frontend-backend-communication.md and docs/telegram-commands.md.
func (s *Server) getLLMConfigStatus(agent *db.Agent) llmConfigStatus {
	provider := normalizeLLMProvider(agent.Provider)
	model := strings.TrimSpace(agent.Model)
	status := llmConfigStatus{
		Provider:   provider,
		ProviderUI: llmProviderLabel(provider),
		Model:      model,
		ModelType:  inferModelType(provider, model),
	}

	if provider == "" {
		status.Missing = append(status.Missing, "provider")
	}
	if model == "" {
		status.Missing = append(status.Missing, "model")
	}

	apiKey, _ := s.db.GetSetting(llmProviderSettingKey(provider))
	status.APIKeySet = strings.TrimSpace(apiKey) != ""
	if !status.APIKeySet {
		status.Missing = append(status.Missing, fmt.Sprintf("%s API key", status.ProviderUI))
	}

	status.Configured = len(status.Missing) == 0
	return status
}

func (status llmConfigStatus) missingSummary() string {
	if len(status.Missing) == 0 {
		return ""
	}
	return strings.Join(status.Missing, ", ")
}
