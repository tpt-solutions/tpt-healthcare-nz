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

// OpenAIProvider calls any OpenAI-compatible API: OpenAI, Azure OpenAI,
// Together AI, Groq, etc. Set baseURL to "https://api.openai.com" for
// OpenAI, or the Azure endpoint for Azure OpenAI.
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider creates an OpenAIProvider.
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIProvider{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string { return "openai:" + p.model }

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type requestBody struct {
		Model       string        `json:"model"`
		Messages    []chatMessage `json:"messages"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		Temperature *float64      `json:"temperature,omitempty"`
	}

	msgs := []chatMessage{}
	if req.SystemPrompt != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: req.UserPrompt})

	reqBody := requestBody{
		Model:    p.model,
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		reqBody.MaxTokens = req.MaxTokens
	}
	if req.Temperature >= 0 {
		t := req.Temperature
		reqBody.Temperature = &t
	}

	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai openai: building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai openai: POST /v1/chat/completions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai openai: API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ai openai: decoding response: %w", err)
	}

	text := ""
	if len(result.Choices) > 0 {
		text = result.Choices[0].Message.Content
	}

	return &CompletionResponse{
		Text:         text,
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
		Model:        result.Model,
	}, nil
}

func (p *OpenAIProvider) Translate(ctx context.Context, req TranslationRequest) (string, error) {
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

// Transcribe sends audio to the OpenAI Whisper API for transcription.
func (p *OpenAIProvider) Transcribe(ctx context.Context, req STTRequest) (string, error) {
	if len(req.Audio) == 0 {
		return "", fmt.Errorf("ai openai: audio data is required for transcription")
	}

	var buf bytes.Buffer
	// Build multipart form manually to avoid importing mime/multipart.
	boundary := "tpt-stt-boundary"
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString(`Content-Disposition: form-data; name="file"; filename="audio.webm"` + "\r\n")
	buf.WriteString("Content-Type: audio/webm\r\n\r\n")
	buf.Write(req.Audio)
	buf.WriteString("\r\n--" + boundary + "\r\n")
	buf.WriteString(`Content-Disposition: form-data; name="model"` + "\r\n\r\n")
	buf.WriteString("whisper-1\r\n")
	if req.Language != "" {
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString(`Content-Disposition: form-data; name="language"` + "\r\n\r\n")
		buf.WriteString(req.Language + "\r\n")
	}
	buf.WriteString("--" + boundary + "--\r\n")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/audio/transcriptions", &buf)
	if err != nil {
		return "", fmt.Errorf("ai openai whisper: building request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ai openai whisper: POST /v1/audio/transcriptions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai openai whisper: API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ai openai whisper: decoding response: %w", err)
	}
	return result.Text, nil
}
