// Package teams implements video.Provider for Microsoft Teams Online Meetings
// via the Microsoft Graph API. Required for practices already on Microsoft 365
// (common in DHB-funded services and hospital outpatient departments).
// Requires an Azure AD app registration with Calendars.ReadWrite and
// OnlineMeetings.ReadWrite.All permissions.
// API docs: https://learn.microsoft.com/en-us/graph/api/application-post-onlinemeetings
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/video"
)

func init() {
	video.Register("teams", func(ctx context.Context, v *viper.Viper) (video.Provider, error) {
		return New(ctx, Config{
			TenantID:     v.GetString("video.teams.tenant_id"),
			ClientID:     v.GetString("video.teams.client_id"),
			ClientSecret: v.GetString("video.teams.client_secret"),
			OrganizerID:  v.GetString("video.teams.organizer_id"), // Azure AD user object ID
		})
	})
}

// Config holds Microsoft Azure AD app credentials.
type Config struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	OrganizerID  string // Azure AD object ID of the meeting organizer (clinician user)
}

// Provider implements video.Provider for Microsoft Teams.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.TenantID == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("teams: tenant_id, client_id, and client_secret are required")
	}
	if cfg.OrganizerID == "" {
		return nil, fmt.Errorf("teams: organizer_id (Azure AD user object ID) is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "teams"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) accessToken(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
		"scope":         {"https://graph.microsoft.com/.default"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.cfg.TenantID),
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("teams token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("teams token http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("teams token decode: %w", err)
	}
	return tok.AccessToken, nil
}

func (p *Provider) graphPost(ctx context.Context, path string, body any) ([]byte, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(body)
	var result []byte
	err = resilience.Do(ctx, p.breaker, "teams", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.microsoft.com/v1.0"+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("teams request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("teams http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("teams read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &video.Error{Provider: "teams", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) CreateRoom(ctx context.Context, opts video.RoomOptions) (*video.Room, error) {
	start := time.Now().UTC()
	end := start.Add(opts.MaxDuration)
	payload := map[string]any{
		"startDateTime": start.Format(time.RFC3339),
		"endDateTime":   end.Format(time.RFC3339),
		"subject":       fmt.Sprintf("Telehealth appointment %s", opts.AppointmentID),
	}
	raw, err := p.graphPost(ctx, fmt.Sprintf("/users/%s/onlineMeetings", p.cfg.OrganizerID), payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID              string `json:"id"`
		JoinWebUrl      string `json:"joinWebUrl"`
		JoinMeetingIdSettings struct {
			IsPasscodeRequired bool   `json:"isPasscodeRequired"`
		} `json:"joinMeetingIdSettings"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("teams create decode: %w", err)
	}
	return &video.Room{
		ExternalID: resp.ID,
		HostURL:    resp.JoinWebUrl,
		PatientURL: resp.JoinWebUrl,
		ExpiresAt:  end.Add(30 * time.Minute),
	}, nil
}

func (p *Provider) GetJoinURL(_ context.Context, roomID, _ string, _ video.ParticipantRole) (string, error) {
	// Teams join URL is the same for all participants; role is managed by the Teams client.
	return fmt.Sprintf("https://teams.microsoft.com/l/meetup-join/meeting/%s", roomID), nil
}

func (p *Provider) EndRoom(ctx context.Context, roomID string) error {
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("https://graph.microsoft.com/v1.0/users/%s/onlineMeetings/%s", p.cfg.OrganizerID, roomID), nil)
	if err != nil {
		return fmt.Errorf("teams end room: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("teams end room http: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (p *Provider) GetRecordingURL(_ context.Context, _ string) (string, error) {
	// Teams recordings are stored in OneDrive/SharePoint; retrieval requires
	// additional Graph API calls to the drive. Returns empty for now.
	return "", nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*video.HealthStatus, error) {
	start := time.Now()
	_, err := p.accessToken(ctx)
	latency := time.Since(start)
	if err != nil {
		return &video.HealthStatus{OK: false, Provider: "teams", Latency: latency, Err: err.Error()}, nil
	}
	return &video.HealthStatus{OK: true, Provider: "teams", Latency: latency}, nil
}
