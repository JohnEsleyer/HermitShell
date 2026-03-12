package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Bot struct {
	token  string
	apiURL string
	http   *http.Client
}

type Option func(*Bot)

func WithAPIURL(url string) Option {
	return func(b *Bot) {
		b.apiURL = strings.TrimRight(url, "/")
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(b *Bot) {
		b.http = httpClient
	}
}

func NewBot(token string, opts ...Option) *Bot {
	b := &Bot{
		token:  token,
		apiURL: "https://api.telegram.org",
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int64       `json:"message_id"`
	From      *User       `json:"from"`
	Chat      *Chat       `json:"chat"`
	Text      string      `json:"text"`
	Document  *Document   `json:"document"`
	Photo     []PhotoSize `json:"photo"`
}

type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type CallbackQuery struct {
	ID   string `json:"id"`
	From *User  `json:"from"`
	Data string `json:"data"`
}

type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

type PhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int    `json:"file_size"`
}

type SendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool   `json:"disable_notification,omitempty"`
	ReplyToMessageID      int64  `json:"reply_to_message_id,omitempty"`
}

func (b *Bot) SendMessage(chatID, text string) error {
	req := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", b.apiURL, b.token)
	resp, err := b.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (b *Bot) SendPhoto(chatID, filePath, caption string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("chat_id", chatID)
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	part, err := writer.CreateFormFile("photo", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	writer.Close()

	url := fmt.Sprintf("%s/bot%s/sendPhoto", b.apiURL, b.token)
	resp, err := b.http.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (b *Bot) SendDocument(chatID, filePath, caption string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("chat_id", chatID)
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	writer.Close()

	url := fmt.Sprintf("%s/bot%s/sendDocument", b.apiURL, b.token)
	resp, err := b.http.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (b *Bot) AnswerCallbackQuery(callbackID, text string) error {
	req := map[string]string{
		"callback_query_id": callbackID,
	}
	if text != "" {
		req["text"] = text
	}

	body, _ := json.Marshal(req)
	url := fmt.Sprintf("%s/bot%s/answerCallbackQuery", b.apiURL, b.token)
	b.http.Post(url, "application/json", bytes.NewReader(body))

	return nil
}

func (b *Bot) SetWebhook(webhookURL string) error {
	url := fmt.Sprintf("%s/bot%s/setWebhook?url=%s&drop_pending_updates=true", b.apiURL, b.token, url.QueryEscape(webhookURL))
	resp, err := b.http.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}
	return nil
}

type WebhookInfo struct {
	URL                  string `json:"url"`
	HasCustomCertificate bool   `json:"has_custom_certificate"`
	PendingUpdateCount   int    `json:"pending_update_count"`
	LastErrorDate        int64  `json:"last_error_date"`
	LastErrorMessage     string `json:"last_error_message"`
	MaxConnections       int    `json:"max_connections"`
	IPAddress            string `json:"ip_address"`
}

func (b *Bot) GetWebhookInfo() (*WebhookInfo, error) {
	endpoint := fmt.Sprintf("%s/bot%s/getWebhookInfo", b.apiURL, b.token)
	resp, err := b.http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		OK     bool        `json:"ok"`
		Result WebhookInfo `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Result, nil
}

func (b *Bot) GetFile(fileID string) (string, error) {
	url := fmt.Sprintf("%s/bot%s/getFile?file_id=%s", b.apiURL, b.token, fileID)
	resp, err := b.http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Result.FilePath, nil
}

func (b *Bot) DownloadFile(filePath, destPath string) error {
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, filePath)
	resp, err := b.http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
