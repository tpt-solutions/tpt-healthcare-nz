// Package ai provides a provider-agnostic AI inference interface for clinical
// decision support features. All AI features are opt-in and controlled by
// per-tenant and per-feature configuration flags. No PHI is sent to a cloud
// AI service without explicit tenant consent — when a tenant opts in to a
// cloud provider, they accept the provider's DPA (Data Processing Agreement).
//
// Supported provider backends:
//   - local:   HTTP call to a locally hosted inference server (Ollama, llama.cpp server)
//   - claude:  Anthropic Claude API (requires ANTHROPIC_API_KEY)
//   - openai:  OpenAI-compatible API (Azure OpenAI, OpenAI, Together, etc.)
//   - noop:    No-op provider that returns empty results; used when AI is disabled
//
// Feature flags are checked before each call so that a practice can enable
// AI-assisted referral drafting without enabling diagnostic suggestions, etc.
package ai

import (
	"context"
	"fmt"
)

// Feature is a named AI capability that can be toggled independently.
type Feature string

const (
	// FeatureReferralDraft generates a draft referral letter from SOAP notes.
	FeatureReferralDraft Feature = "referral_draft"
	// FeatureDischargeSummary generates a draft discharge summary.
	FeatureDischargeSummary Feature = "discharge_summary"
	// FeatureCodingSuggestion suggests ICD-10/SNOMED codes from clinical text.
	FeatureCodingSuggestion Feature = "coding_suggestion"
	// FeatureDiagnosisDifferential suggests differential diagnoses from symptoms.
	FeatureDiagnosisDifferential Feature = "diagnosis_differential"
	// FeatureTranslation translates UI text or patient-facing content between languages.
	FeatureTranslation Feature = "translation"
	// FeatureSTT is speech-to-text transcription (cloud fallback only; Web Speech API
	// is used client-side as the primary STT path).
	FeatureSTT Feature = "stt"
)

// Config holds the AI provider configuration for a tenant.
type Config struct {
	// Provider selects the backend: "local", "claude", "openai", or "noop".
	Provider string `json:"provider"`
	// BaseURL is required for "local" and "openai" providers.
	BaseURL string `json:"baseUrl,omitempty"`
	// Model is the model name to use (e.g. "llama3.2", "claude-sonnet-4-6", "gpt-4o").
	Model string `json:"model,omitempty"`
	// APIKey is used for "claude" and "openai" providers.
	// It is stored encrypted in the tenant settings table.
	APIKey string `json:"-"`
	// EnabledFeatures lists the features that are active for this tenant.
	// An empty list means all AI features are disabled.
	EnabledFeatures []Feature `json:"enabledFeatures"`
}

// IsEnabled returns true when the given feature is in the tenant's enabled list.
func (c *Config) IsEnabled(f Feature) bool {
	for _, enabled := range c.EnabledFeatures {
		if enabled == f {
			return true
		}
	}
	return false
}

// CompletionRequest is a generic text completion request.
type CompletionRequest struct {
	// SystemPrompt is the system/instruction prompt.
	SystemPrompt string
	// UserPrompt is the user turn content.
	UserPrompt string
	// MaxTokens is the maximum tokens to generate. 0 uses the provider default.
	MaxTokens int
	// Temperature controls randomness (0.0–1.0). -1 uses provider default.
	Temperature float64
}

// CompletionResponse holds the model's generated text.
type CompletionResponse struct {
	Text         string `json:"text"`
	InputTokens  int    `json:"inputTokens"`
	OutputTokens int    `json:"outputTokens"`
	Model        string `json:"model"`
}

// TranslationRequest is a request to translate text between languages.
type TranslationRequest struct {
	Text           string `json:"text"`
	SourceLanguage string `json:"sourceLanguage"` // BCP 47, e.g. "en", "mi", "sm"
	TargetLanguage string `json:"targetLanguage"`
}

// STTRequest is a speech-to-text transcription request.
type STTRequest struct {
	// Audio is the raw audio bytes (WAV or WebM/Opus).
	Audio    []byte `json:"-"`
	// Language is the expected spoken language as BCP 47 code.
	Language string `json:"language,omitempty"`
}

// Provider is the interface all AI backends must implement.
type Provider interface {
	// Complete sends a text completion request and returns the generated text.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// Translate translates text between two languages.
	Translate(ctx context.Context, req TranslationRequest) (string, error)
	// Transcribe converts audio to text.
	Transcribe(ctx context.Context, req STTRequest) (string, error)
	// Name returns the provider name for logging.
	Name() string
}

// FeatureGuard wraps a Provider and enforces feature-flag checks.
// Call Guard.Check(feature) before any provider method to get a type-safe
// interface that is guaranteed to only be used when the feature is enabled.
type FeatureGuard struct {
	provider Provider
	config   *Config
}

// NewFeatureGuard creates a FeatureGuard.
func NewFeatureGuard(p Provider, cfg *Config) *FeatureGuard {
	return &FeatureGuard{provider: p, config: cfg}
}

// EnabledFor returns the provider if feature is enabled, or an error if not.
func (g *FeatureGuard) EnabledFor(feature Feature) (Provider, error) {
	if g.config == nil || !g.config.IsEnabled(feature) {
		return nil, fmt.Errorf("ai: feature %q is not enabled for this tenant", feature)
	}
	return g.provider, nil
}

// ---------------------------------------------------------------------------
// Built-in clinical prompt templates
// ---------------------------------------------------------------------------

// ReferralDraftPrompt returns the system and user prompts for generating a
// referral letter from a SOAP note.
func ReferralDraftPrompt(soapNote, referralType, patientAge, patientSex string) CompletionRequest {
	return CompletionRequest{
		SystemPrompt: `You are a clinical documentation assistant for a New Zealand general practice.
Your role is to draft professional referral letters based on consultation notes.
Follow NZ referral etiquette: address the specialist by name/department, include
patient demographics (age, sex), presenting complaint, relevant history, examination
findings, current medications, and reason for referral. Keep the letter concise (under
300 words) and professional. Do not invent clinical findings not present in the note.`,
		UserPrompt: fmt.Sprintf(
			"Patient: %s-year-old %s\nReferral type: %s\n\nConsultation note:\n%s\n\nPlease draft a referral letter.",
			patientAge, patientSex, referralType, soapNote,
		),
		MaxTokens:   600,
		Temperature: 0.3,
	}
}

// DischargeSummaryPrompt returns the prompts for drafting a hospital discharge summary.
func DischargeSummaryPrompt(admissionReason, procedures, medications, followUp string) CompletionRequest {
	return CompletionRequest{
		SystemPrompt: `You are a clinical documentation assistant generating hospital discharge summaries
for New Zealand secondary care. Include: reason for admission, clinical findings,
procedures performed, medications on discharge, and follow-up plan. Comply with
Te Whatu Ora discharge summary requirements. Keep under 400 words.`,
		UserPrompt: fmt.Sprintf(
			"Admission reason: %s\nProcedures: %s\nDischarge medications: %s\nFollow-up plan: %s\n\nPlease draft the discharge summary.",
			admissionReason, procedures, medications, followUp,
		),
		MaxTokens:   800,
		Temperature: 0.2,
	}
}

// CodingSuggestionPrompt returns prompts for suggesting ICD-10-AM codes.
func CodingSuggestionPrompt(clinicalNote string) CompletionRequest {
	return CompletionRequest{
		SystemPrompt: `You are a clinical coding assistant familiar with ICD-10-AM (the Australian/NZ
edition of ICD-10). Given a clinical note, suggest up to 5 relevant ICD-10-AM codes
with their descriptions. Format each as: CODE — Description. Prioritise the principal
diagnosis first. Do not suggest codes you are uncertain about.`,
		UserPrompt:  fmt.Sprintf("Clinical note:\n%s\n\nSuggest ICD-10-AM codes.", clinicalNote),
		MaxTokens:   300,
		Temperature: 0.1,
	}
}

// DifferentialDiagnosisPrompt returns prompts for generating a differential diagnosis list.
func DifferentialDiagnosisPrompt(symptoms, vitals, patientAge, patientSex string) CompletionRequest {
	return CompletionRequest{
		SystemPrompt: `You are a clinical decision support tool for a New Zealand primary care setting.
Given patient symptoms, vitals, age, and sex, provide a ranked differential diagnosis
list (up to 5 conditions) with the most likely first. For each condition, include the
key discriminating features and a note on urgency. Note any red-flag symptoms that
warrant immediate action. Base suggestions on NZ HealthPathways-aligned practice.`,
		UserPrompt: fmt.Sprintf(
			"Patient: %s-year-old %s\nSymptoms: %s\nVitals: %s\n\nProvide differential diagnoses.",
			patientAge, patientSex, symptoms, vitals,
		),
		MaxTokens:   500,
		Temperature: 0.4,
	}
}
