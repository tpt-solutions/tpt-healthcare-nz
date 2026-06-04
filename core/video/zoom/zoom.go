// Package zoom implements video.Provider for the Zoom Meeting API v2.
// Zoom is popular with patients; OAuth2 Server-to-Server app required.
// API docs: https://developers.zoom.us/docs/api/meetings/
package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/video"
)

func init() {
	video.Register("zoom", func(ctx context.Context, v *viper.Viper) (video.Provider, error) {
		return New(ctx, Config{
			AccountID:    v.GetString("video.zoom.account_id"),
			ClientID:     v.GetString("video.zoom.client_id"),
			ClientSecret: v.GetString("video.zoom.client_secret"),
			HostEmail:    v.GetString("video.zoom.host_email"),
		})
	})
}

const baseURL = "https://api.zoom.us/v2"

// Config holds Zoom Server-to-Server OAuth2 credentials.
type Config struct {
	AccountID    string
	ClientID     string
	ClientSecret string
	HostEmail    string // Zoom user to host meetings under
}

// Provider implements video.Provider for Zoom.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccountID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("zoom: account_id, client_id, and client_secret are required")
	}
	if cfg.HostEmail == "" {
		return nil, fmt.Errorf("zoom: host_email is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "zoom"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) accessToken(ctx context.Context) (string, error) {
	// Server-to-Server OAuth2 token request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://zoom.us/oauth/token?grant_type=account_credentials&account_id="+p.cfg.AccountID, nil)
	if err != nil {
		return "", fmt.Errorf("zoom token request: %w", err)
	}
	req.SetBasicAuth(p.cfg.ClientID, p.cfg.ClientSecret)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("zoom token http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("zoom token decode: %w", err)
	}
	return tok.AccessToken, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(body)
	var result []byte
	err = resilience.Do(ctx, p.breaker, "zoom", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("zoom request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("zoom http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("zoom read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &video.Error{Provider: "zoom", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) CreateRoom(ctx context.Context, opts video.RoomOptions) (*video.Room, error) {
	durationMins := int(opts.MaxDuration.Minutes())
	if durationMins == 0 {
		durationMins = 30
	}
	payload := map[string]any{
		"topic":    fmt.Sprintf("Appointment %s", opts.AppointmentID),
		"type":     2, // scheduled meeting
		"duration": durationMins,
		"settings": map[string]any{
			"join_before_host":  false,
			"waiting_room":      true,
			"auto_recording":    map[string]string{"recording": "none"}[fmt.Sprint(opts.Recording)],
		},
	}
	raw, err := p.post(ctx, "/users/"+p.cfg.HostEmail+"/meetings", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID        int64  `json:"id"`
		JoinURL   string `json:"join_url"`
		StartURL  string `json:"start_url"`
		StartTime string `json:"start_time"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("zoom create decode: %w", err)
	}
	return &video.Room{
		ExternalID: fmt.Sprint(resp.ID),
		HostURL:    resp.StartURL,
		PatientURL: resp.JoinURL,
		ExpiresAt:  time.Now().Add(opts.MaxDuration + time.Hour),
	}, nil
}

func (p *Provider) GetJoinURL(_ context.Context, roomID, _ string, role video.ParticipantRole) (string, error) {
	// Zoom join/start URLs are fixed per meeting; role determines which URL to return.
	// In production, fetch the meeting and return join_url (guest) or start_url (host).
	_ = role
	return fmt.Sprintf("https://zoom.us/j/%s", roomID), nil
}

func (p *Provider) EndRoom(ctx context.Context, roomID string) error {
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/meetings/"+roomID, nil)
	if err != nil {
		return fmt.Errorf("zoom end room: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("zoom end room http: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (p *Provider) GetRecordingURL(ctx context.Context, roomID string) (string, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/meetings/"+roomID+"/recordings", nil)
	if err != nil {
		return "", fmt.Errorf("zoom recordings: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("zoom recordings http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var rec struct {
		RecordingFiles []struct {
			DownloadURL string `json:"download_url"`
		} `json:"recording_files"`
	}
	_ = json.Unmarshal(raw, &rec)
	if len(rec.RecordingFiles) > 0 {
		return rec.RecordingFiles[0].DownloadURL, nil
	}
	return "", nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*video.HealthStatus, error) {
	start := time.Now()
	_, err := p.accessToken(ctx)
	latency := time.Since(start)
	if err != nil {
		return &video.HealthStatus{OK: false, Provider: "zoom", Latency: latency, Err: err.Error()}, nil
	}
	return &video.HealthStatus{OK: true, Provider: "zoom", Latency: latency}, nil
}
