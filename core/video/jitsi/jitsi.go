// Package jitsi implements video.Provider for a self-hosted Jitsi Meet server.
// Self-hosting is the preferred option for NZ data sovereignty: all video
// data stays within the practice's infrastructure. Rooms are protected by
// JWT authentication (jitsi-meet's token_authentication_url is set to validate
// these tokens). Docker image: jitsi/jitsi-meet.
// API: Jitsi JWT + optional REST API for room management.
package jitsi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/video"
)

func init() {
	video.Register("jitsi", func(ctx context.Context, v *viper.Viper) (video.Provider, error) {
		return New(ctx, Config{
			AppID:     v.GetString("video.jitsi.app_id"),
			AppSecret: v.GetString("video.jitsi.app_secret"),
			BaseURL:   v.GetString("video.jitsi.base_url"),
		})
	})
}

// Config holds Jitsi Meet JWT credentials and server URL.
type Config struct {
	AppID     string // JWT "iss" claim; matches jitsi application_id
	AppSecret string // HMAC secret for JWT signing
	BaseURL   string // e.g. "https://meet.practice.nz"
}

// Provider implements video.Provider for Jitsi Meet.
type Provider struct {
	cfg    Config
	client *http.Client
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("jitsi: app_id and app_secret are required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("jitsi: base_url is required (e.g. https://meet.practice.nz)")
	}
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

// makeJWT builds a Jitsi JWT for the given room and participant.
func (p *Provider) makeJWT(roomName, participantName string, role video.ParticipantRole, expiry time.Time) (string, error) {
	isModerator := role == video.RoleHost
	claims := jwt.MapClaims{
		"iss": p.cfg.AppID,
		"sub": "*", // allow any domain
		"aud": p.cfg.AppID,
		"exp": expiry.Unix(),
		"nbf": time.Now().Unix(),
		"room": roomName,
		"context": map[string]any{
			"user": map[string]any{
				"name":        participantName,
				"moderator":   isModerator,
			},
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(p.cfg.AppSecret))
}

func (p *Provider) CreateRoom(ctx context.Context, opts video.RoomOptions) (*video.Room, error) {
	// Jitsi rooms are created implicitly on first join. We pre-generate the URLs.
	roomName := fmt.Sprintf("tpt-%s", opts.AppointmentID)
	expiry := time.Now().Add(opts.MaxDuration + 30*time.Minute)

	hostToken, err := p.makeJWT(roomName, "Clinician", video.RoleHost, expiry)
	if err != nil {
		return nil, fmt.Errorf("jitsi host jwt: %w", err)
	}
	guestToken, err := p.makeJWT(roomName, "Patient", video.RoleGuest, expiry)
	if err != nil {
		return nil, fmt.Errorf("jitsi guest jwt: %w", err)
	}

	hostURL := fmt.Sprintf("%s/%s?jwt=%s", p.cfg.BaseURL, roomName, hostToken)
	patientURL := fmt.Sprintf("%s/%s?jwt=%s", p.cfg.BaseURL, roomName, guestToken)

	_ = ctx
	return &video.Room{
		ExternalID: roomName,
		HostURL:    hostURL,
		PatientURL: patientURL,
		ExpiresAt:  expiry,
	}, nil
}

func (p *Provider) GetJoinURL(_ context.Context, roomID, participantName string, role video.ParticipantRole) (string, error) {
	expiry := time.Now().Add(2 * time.Hour)
	token, err := p.makeJWT(roomID, participantName, role, expiry)
	if err != nil {
		return "", fmt.Errorf("jitsi join jwt: %w", err)
	}
	return fmt.Sprintf("%s/%s?jwt=%s", p.cfg.BaseURL, roomID, token), nil
}

func (p *Provider) EndRoom(ctx context.Context, roomID string) error {
	// Jitsi REST API room termination (requires jitsi-videobridge REST API enabled).
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/colibri/conferences/%s", p.cfg.BaseURL, roomID), nil)
	if err != nil {
		return fmt.Errorf("jitsi end room: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("jitsi end room http: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (p *Provider) GetRecordingURL(_ context.Context, _ string) (string, error) {
	// Recording via Jitsi requires Jibri (Jitsi Broadcasting Infrastructure).
	// URL is available from the Jibri recording status endpoint after the session ends.
	return "", nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*video.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/_jitsi-meet-conference/config.js", nil)
	if err != nil {
		return &video.HealthStatus{OK: false, Provider: "jitsi", Err: err.Error()}, nil
	}
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "jitsi server unreachable"
		if err != nil {
			msg = err.Error()
		}
		return &video.HealthStatus{OK: false, Provider: "jitsi", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &video.HealthStatus{OK: true, Provider: "jitsi", Latency: latency}, nil
}

// _ suppresses unused import.
var _ = json.Marshal
