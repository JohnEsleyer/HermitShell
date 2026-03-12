package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL  string
	apiKey   string
	model    string
	http     *http.Client
	provider Provider
}

type Provider string

const (
	ProviderOpenAI     Provider = "openai"
	ProviderOpenRouter Provider = "openrouter"
	ProviderAnthropic  Provider = "anthropic"
	ProviderGemini     Provider = "gemini"
)

type Option func(*Client)

func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

func WithProvider(p Provider) Option {
	return func(c *Client) {
		c.provider = p
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		baseURL:  "https://api.openai.com/v1",
		model:    "gpt-4o-mini",
		provider: ProviderOpenAI,
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message Message `json:"message"`
	Delta   Message `json:"delta"`
	Index   int     `json:"index"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func FormatMessages(messages []Message) string {
	b, _ := json.Marshal(messages)
	return string(b)
}

func (c *Client) chatEndpoint() string {
	switch c.provider {
	case ProviderOpenAI:
		if c.baseURL != "" {
			return strings.TrimRight(c.baseURL, "/") + "/chat/completions"
		}
		return "https://api.openai.com/v1/chat/completions"
	case ProviderAnthropic:
		if c.baseURL != "" {
			return strings.TrimRight(c.baseURL, "/") + "/messages"
		}
		return "https://api.anthropic.com/v1/messages"
	case ProviderGemini:
		if c.baseURL != "" {
			return strings.TrimRight(c.baseURL, "/") + "/chat/completions"
		}
		return "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	default:
		return strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	}
}

func (c *Client) setHeaders(req *http.Request, stream bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if c.provider == ProviderOpenRouter {
		req.Header.Set("HTTP-Referer", "https://hermit.sh")
		req.Header.Set("X-Title", "Hermit")
	}
	if c.provider == ProviderAnthropic {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func (c *Client) Chat(model string, messages []Message) (string, error) {
	reqBody := ChatRequest{Model: model, Messages: messages}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.chatEndpoint(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, false)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return chatResp.Choices[0].Message.Content, nil
}

type StreamCallback func(content string) error

func (c *Client) StreamChat(model string, messages []Message, callback StreamCallback) error {
	reqBody := ChatRequest{Model: model, Messages: messages, Stream: true}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.chatEndpoint(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(req, true)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta Message `json:"delta"`
				Index int     `json:"index"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := callback(chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
