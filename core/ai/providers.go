package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// NoopProvider — used when AI is disabled
// ---------------------------------------------------------------------------

// NoopProvider is a Provider that returns empty results. It is used when the
// tenant has not enabled AI features or when no provider is configured.
type NoopProvider struct{}

func (NoopProvider) Name() string { return "noop" }

func (NoopProvider) Complete(_ context.Context, _ CompletionRequest) (*CompletionResponse, error) {
	return nil, fmt.Errorf("ai: no AI provider configured")
}

func (NoopProvider) Translate(_ context.Context, _ TranslationRequest) (string, error) {
	return "", fmt.Errorf("ai: no AI provider configured")
}

func (NoopProvider) Transcribe(_ context.Context, _ STTRequest) (string, error) {
	return "", fmt.Errorf("ai: no AI provider configured")
}

// ---------------------------------------------------------------------------
// LocalProvider — Ollama / llama.cpp-compatible HTTP server
// ---------------------------------------------------------------------------

// LocalProvider calls a locally hosted Ollama or llama.cpp HTTP server.
// No PHI leaves the server boundary when using this provider.
type LocalProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewLocalProvider creates a LocalProvider targeting baseURL.
// model is the Ollama/llama.cpp model name (e.g. "llama3.2:8b-instruct-q4_K_M").
func NewLocalProvider(baseURL, model string) *LocalProvider {
	return &LocalProvider{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *LocalProvider) Name() string { return "local:" + p.model }

func (p *LocalProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	type ollamaMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type ollamaRequest struct {
		Model    string          `json:"model"`
		Messages []ollamaMessage `json:"messages"`
		Stream   bool            `json:"stream"`
		Options  map[string]any  `json:"options,omitempty"`
	}

	msgs := []ollamaMessage{}
	if req.SystemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, ollamaMessage{Role: "user", Content: req.UserPrompt})

	opts := map[string]any{}
	if req.MaxTokens > 0 {
		opts["num_predict"] = req.MaxTokens
	}
	if req.Temperature >= 0 {
		opts["temperature"] = req.Temperature
	}

	body, _ := json.Marshal(ollamaRequest{
		Model:    p.model,
		Messages: msgs,
		Stream:   false,
		Options:  opts,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai local: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai local: POST /api/chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai local: server returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEvalCount int `json:"prompt_eval_count"`
		EvalCount       int `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ai local: decoding response: %w", err)
	}

	return &CompletionResponse{
		Text:         result.Message.Content,
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		Model:        p.model,
	}, nil
}

// Translate uses the local model for translation by wrapping as a completion.
func (p *LocalProvider) Translate(ctx context.Context, req TranslationRequest) (string, error) {
	resp, err := p.Complete(ctx, CompletionRequest{
		SystemPrompt: fmt.Sprintf(
			"You are a professional translator. Translate the following text from %s to %s. Output only the translation, nothing else.",
			req.SourceLanguage, req.TargetLanguage,
		),
		UserPrompt:  req.Text,
		MaxTokens:   500,
		Temperature: 0.1,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

// Transcribe is not supported by the local provider (requires a dedicated STT server).
func (p *LocalProvider) Transcribe(_ context.Context, _ STTRequest) (string, error) {
	return "", fmt.Errorf("ai local: STT not supported by local provider; use Web Speech API or configure a cloud provider")
}

// ---------------------------------------------------------------------------
// ClaudeProvider — Anthropic Claude API
// ---------------------------------------------------------------------------

// ClaudeProvider calls the Anthropic Claude API.
// PHI is sent to Anthropic's servers; requires explicit tenant DPA acceptance.
type ClaudeProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClaudeProvider creates a ClaudeProvider.
// model should be a Claude model ID, e.g. "claude-sonnet-4-6" or "claude-haiku-4-5-20251001".
func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-haiku-4-5-20251001" // cheapest model as default for clinical assistants
	}
	return &ClaudeProvider{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *ClaudeProvider) Name() string { return "claude:" + p.model }

func (p *ClaudeProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	type contentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type message struct {
		Role    string         `json:"role"`
		Content []contentBlock `json:"content"`
	}
	type requestBody struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system,omitempty"`
		Messages  []message `json:"messages"`
	}

	maxTok := req.MaxTokens
	if maxTok <= 0 {
		maxTok = 1024
	}

	body, _ := json.Marshal(requestBody{
		Model:     p.model,
		MaxTokens: maxTok,
		System:    req.SystemPrompt,
		Messages: []message{
			{Role: "user", Content: []contentBlock{{Type: "text", Text: req.UserPrompt}}},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai claude: building request: %w", err)
	}
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai claude: POST /v1/messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai claude: API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ai claude: decoding response: %w", err)
	}

	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}

	return &CompletionResponse{
		Text:         text,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		Model:        result.Model,
	}, nil
}

func (p *ClaudeProvider) Translate(ctx context.Context, req TranslationRequest) (string, error) {
	resp, err := p.Complete(ctx, CompletionRequest{
		SystemPrompt: fmt.Sprintf(
			"You are a professional translator. Translate the following text from %s to %s. Output only the translation, nothing else.",
			req.SourceLanguage, req.TargetLanguage,
		),
		UserPrompt:  req.Text,
		MaxTokens:   500,
		Temperature: 0.1,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

func (p *ClaudeProvider) Transcribe(_ context.Context, _ STTRequest) (string, error) {
	return "", fmt.Errorf("ai claude: STT not supported via the Claude API; use a dedicated Whisper endpoint or cloud STT provider")
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

// New creates the appropriate Provider from a Config.
// Returns a NoopProvider when the provider is "" or "noop".
func New(cfg Config) Provider {
	switch strings.ToLower(cfg.Provider) {
	case "local":
		return NewLocalProvider(cfg.BaseURL, cfg.Model)
	case "claude":
		return NewClaudeProvider(cfg.APIKey, cfg.Model)
	case "openai":
		return NewOpenAIProvider(cfg.APIKey, cfg.BaseURL, cfg.Model)
	default:
		return NoopProvider{}
	}
}
