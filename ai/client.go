package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const httpTimeout = 60 * time.Second

// Client is the unified AI client — same interface regardless of provider
type Client struct {
	cfg *Config
}

// NewClient creates a new AI client from the saved config
func NewClient() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return &Client{cfg: cfg}, nil
}

// NewClientWithConfig creates a client with a specific config
func NewClientWithConfig(cfg *Config) *Client {
	return &Client{cfg: cfg}
}

// Complete sends a prompt to the configured AI provider and returns the response
func (c *Client) Complete(prompt string) (string, error) {
	switch c.cfg.Provider {
	case ProviderOpenAI:
		return c.completeOpenAI(prompt)
	case ProviderAnthropic:
		return c.completeAnthropic(prompt)
	case ProviderGemini:
		return c.completeGemini(prompt)
	case ProviderOllama:
		return c.completeOllama(prompt)
	default:
		return "", fmt.Errorf("unknown provider: %s", c.cfg.Provider)
	}
}

// ProviderName returns a human-readable provider name
func (c *Client) ProviderName() string {
	if info, ok := ProviderInfo[c.cfg.Provider]; ok {
		return info.Name
	}
	return string(c.cfg.Provider)
}

// Model returns the model being used
func (c *Client) Model() string {
	return c.cfg.Model
}

// ─── OpenAI ──────────────────────────────────────────────────────────────────

type openAIRequest struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) completeOpenAI(prompt string) (string, error) {
	reqBody := openAIRequest{
		Model: c.cfg.Model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 2048,
	}

	resp, err := c.post(
		"https://api.openai.com/v1/chat/completions",
		map[string]string{
			"Authorization": "Bearer " + c.cfg.APIKey,
			"Content-Type":  "application/json",
		},
		reqBody,
	)
	if err != nil {
		return "", err
	}

	var result openAIResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned empty response")
	}

	return result.Choices[0].Message.Content, nil
}

// ─── Anthropic ───────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) completeAnthropic(prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.cfg.Model,
		MaxTokens: 2048,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.post(
		"https://api.anthropic.com/v1/messages",
		map[string]string{
			"x-api-key":         c.cfg.APIKey,
			"anthropic-version": "2023-06-01",
			"Content-Type":      "application/json",
		},
		reqBody,
	)
	if err != nil {
		return "", err
	}

	var result anthropicResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("Anthropic returned empty response")
	}

	return result.Content[0].Text, nil
}

// ─── Google Gemini ───────────────────────────────────────────────────────────

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) completeGemini(prompt string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.cfg.Model, c.cfg.APIKey,
	)

	resp, err := c.post(url, map[string]string{"Content-Type": "application/json"}, reqBody)
	if err != nil {
		return "", err
	}

	var result geminiResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini returned empty response")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ─── Ollama (local, free) ─────────────────────────────────────────────────────

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (c *Client) completeOllama(prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  c.cfg.Model,
		Prompt: prompt,
		Stream: false,
	}

	host := c.cfg.OllamaHost
	if host == "" {
		host = "http://localhost:11434"
	}

	resp, err := c.post(
		host+"/api/generate",
		map[string]string{"Content-Type": "application/json"},
		reqBody,
	)
	if err != nil {
		return "", fmt.Errorf("Ollama error (is it running? try: ollama serve): %w", err)
	}

	var result ollamaResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("Ollama error: %s", result.Error)
	}

	return result.Response, nil
}

// ─── HTTP helper ─────────────────────────────────────────────────────────────

func (c *Client) post(url string, headers map[string]string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}
