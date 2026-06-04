package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OrthancClient communicates with an Orthanc DICOM server via its DICOMweb
// plugin (QIDO-RS / WADO-RS / STOW-RS) and REST management API.
type OrthancClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewOrthancClient returns a client pointed at baseURL. If apiKey is non-empty
// it is sent as "Authorization: Bearer <key>" on every request.
func NewOrthancClient(baseURL, apiKey string, logger *slog.Logger) *OrthancClient {
	return &OrthancClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			// DICOM transfers may be large — use a long timeout.
			Timeout: 10 * time.Minute,
		},
		logger: logger,
	}
}

// dicomWebURL builds a full DICOMweb URL from a path relative to /dicom-web.
func (c *OrthancClient) dicomWebURL(path string) string {
	return c.baseURL + "/dicom-web" + path
}

// newRequest constructs an authenticated HTTP request to Orthanc.
func (c *OrthancClient) newRequest(ctx context.Context, method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

// ProxyQIDO executes a QIDO-RS query (study/series/instance search) against
// Orthanc. path is relative to /dicom-web (e.g. "/studies"). Returns the raw
// application/dicom+json response body and its Content-Type.
func (c *OrthancClient) ProxyQIDO(ctx context.Context, path string, query url.Values) ([]byte, string, error) {
	rawURL := c.dicomWebURL(path)
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build QIDO request: %w", err)
	}
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("QIDO request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read QIDO response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("Orthanc QIDO %d: %s", resp.StatusCode, string(body))
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// ProxyWADO opens a WADO-RS retrieve stream from Orthanc. The caller MUST
// close the returned ReadCloser. path is relative to /dicom-web.
func (c *OrthancClient) ProxyWADO(ctx context.Context, path, accept string) (io.ReadCloser, string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.dicomWebURL(path), nil)
	if err != nil {
		return nil, "", fmt.Errorf("build WADO request: %w", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "multipart/related; type=\"application/dicom\"")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("WADO request: %w", err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, "", fmt.Errorf("Orthanc WADO %d", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// ProxySTOW forwards a STOW-RS multipart upload to Orthanc. path is relative
// to /dicom-web (e.g. "/studies" or "/studies/{uid}").
func (c *OrthancClient) ProxySTOW(ctx context.Context, path, contentType string, body io.Reader) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodPost, c.dicomWebURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("build STOW request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("STOW request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read STOW response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Orthanc STOW %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// Ping checks that the Orthanc server is reachable by calling GET /system.
func (c *OrthancClient) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, c.baseURL+"/system", nil)
	if err != nil {
		return fmt.Errorf("build ping request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Orthanc ping: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Orthanc ping returned %d", resp.StatusCode)
	}
	return nil
}
