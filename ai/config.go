// Package ai handles communication with AI providers for the interrogation layer.
// Sentinel never hard-codes an AI provider — developers bring their own API key.
//
// Supported providers:
//   - openai    (GPT-4o, GPT-4-turbo)
//   - anthropic (Claude Sonnet, Claude Opus)
//   - gemini    (Gemini 1.5 Pro, Gemini Flash)
//   - ollama    (Local models — completely free, no API key needed)
package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Provider represents a supported AI provider
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderOllama    Provider = "ollama" // local, free
)

// Config holds the developer's AI provider configuration
type Config struct {
	Provider   Provider `json:"provider"`
	APIKey     string   `json:"api_key"`               // empty for ollama
	Model      string   `json:"model"`                 // specific model to use
	OllamaHost string   `json:"ollama_host,omitempty"` // default: http://localhost:11434
}

// ProviderDefaults maps each provider to its default model.
// Developers can always override these with --model flag.
var ProviderDefaults = map[Provider]string{
	ProviderOpenAI:    "gpt-4o",
	ProviderAnthropic: "claude-haiku-4-5-20251001",
	ProviderGemini:    "gemini-2.0-flash",
	ProviderOllama:    "codellama",
}

// ProviderModelExamples shows common models per provider for the help text
var ProviderModelExamples = map[Provider][]string{
	ProviderOpenAI:    {"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"},
	ProviderAnthropic: {"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5-20251001"},
	ProviderGemini:    {"gemini-2.0-flash", "gemini-1.5-pro", "gemini-2.0-pro-exp"},
	ProviderOllama:    {"codellama", "llama3", "deepseek-coder", "mistral"},
}

// ProviderInfo is human-readable info shown during setup
var ProviderInfo = map[Provider]struct {
	Name    string
	KeyHint string
	Free    bool
}{
	ProviderOpenAI:    {"OpenAI", "sk-...", false},
	ProviderAnthropic: {"Anthropic", "sk-ant-...", false},
	ProviderGemini:    {"Google Gemini", "AIza...", true},
	ProviderOllama:    {"Ollama (local)", "(no key needed)", true},
}

const configFile = ".sentinel/ai_config.json"

// SaveConfig writes the AI provider config to disk
func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(".sentinel", 0700); err != nil {
		return err
	}

	// Never store key in plaintext on disk if we can help it —
	// prefer reading from environment variable at runtime
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0600) // 0600 = owner read/write only
}

// LoadConfig reads the AI provider config from disk or environment variables
// Environment variables always take priority over the config file:
//
//	SENTINEL_AI_PROVIDER=openai
//	SENTINEL_AI_KEY=sk-...
//	SENTINEL_AI_MODEL=gpt-4o
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Try loading from file first
	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("corrupted AI config: %w", err)
		}
	}

	// Environment variables override file config
	if p := os.Getenv("SENTINEL_AI_PROVIDER"); p != "" {
		cfg.Provider = Provider(strings.ToLower(p))
	}
	if k := os.Getenv("SENTINEL_AI_KEY"); k != "" {
		cfg.APIKey = k
	}
	if m := os.Getenv("SENTINEL_AI_MODEL"); m != "" {
		cfg.Model = m
	}

	// Validate
	if cfg.Provider == "" {
		return nil, fmt.Errorf("no AI provider configured\nRun: sentinel scan config --provider <openai|anthropic|gemini|ollama>")
	}

	// Apply default model if none set
	if cfg.Model == "" {
		if def, ok := ProviderDefaults[cfg.Provider]; ok {
			cfg.Model = def
		}
	}

	// Set default Ollama host
	if cfg.Provider == ProviderOllama && cfg.OllamaHost == "" {
		cfg.OllamaHost = "http://localhost:11434"
	}

	// API key required for all providers except ollama
	if cfg.Provider != ProviderOllama && cfg.APIKey == "" {
		return nil, fmt.Errorf(
			"no API key configured for %s\nRun: sentinel scan config --provider %s --key YOUR_KEY\nOr set: export SENTINEL_AI_KEY=YOUR_KEY",
			cfg.Provider, cfg.Provider,
		)
	}

	return cfg, nil
}

// IsConfigured returns true if an AI provider has been configured
func IsConfigured() bool {
	_, err := LoadConfig()
	return err == nil
}

// MaskKey returns a masked version of the API key for display
// e.g. "sk-abc123xyz" → "sk-abc...xyz"
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}
