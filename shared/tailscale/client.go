package tailscale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the base URL for the Tailscale API
	DefaultBaseURL = "https://api.tailscale.com"

	// DefaultAPIVersion is the API version to use
	DefaultAPIVersion = "v2"
)

// Client provides methods to interact with the Tailscale API
type Client struct {
	apiToken   string
	tailnet    string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Tailscale API client
func NewClient(apiToken string) (*Client, error) {
	if apiToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	return &Client{
		apiToken: apiToken,
		baseURL:  DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// SetTailnet sets the tailnet for API operations
func (c *Client) SetTailnet(tailnet string) {
	c.tailnet = tailnet
}

// GetTailnet returns the currently configured tailnet
func (c *Client) GetTailnet() string {
	return c.tailnet
}

// doRequest performs an HTTP request with proper authentication
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := fmt.Sprintf("%s/api/%s%s", c.baseURL, DefaultAPIVersion, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication using basic auth with API token
	req.SetBasicAuth(c.apiToken, "")

	// Set default headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply additional headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handleResponse processes HTTP response and returns error if not successful
func handleResponse(resp *http.Response, expectedStatus int) error {
	if resp.StatusCode == expectedStatus {
		return nil
	}

	// Read error response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d (failed to read error body)", resp.StatusCode),
		}
	}

	// Try to parse as JSON error
	var errResp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		msg := errResp.Message
		if msg == "" {
			msg = errResp.Error
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	// Fallback to raw body
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
}

// APIError represents an error returned by the Tailscale API
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Tailscale API error (%d): %s", e.StatusCode, e.Message)
}

// IsPermissionError returns true if the error is a permission/authorization error
func (e *APIError) IsPermissionError() bool {
	return e.StatusCode == 403 || e.StatusCode == 401
}

// IsConflict returns true if the error is an ETag conflict (412 Precondition Failed)
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 412
}

// IsNotFound returns true if the resource was not found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// DetectTailnet attempts to detect the tailnet from the API token
// Unfortunately, there's no simple API endpoint to auto-detect the tailnet
// This function returns an error with instructions for the user
func (c *Client) DetectTailnet(ctx context.Context) (string, error) {
	return "", fmt.Errorf(`unable to auto-detect tailnet

Please specify your tailnet name with the --tailnet flag.

Your tailnet name is either:
  - Your email-based tailnet (e.g., yourname@github)
  - Your organization's domain (e.g., example.com)

You can find it:
  - In your Tailscale admin console URL
  - By running: tailscale status

Example: tse setup --tailnet yourname@github`)
}

// GetCurrentUser retrieves the authenticated user's email
// This is used to determine the owner for tagOwners
func (c *Client) GetCurrentUser(ctx context.Context) (string, error) {
	// The /api/v2/tailnet/{tailnet}/acl endpoint doesn't give us user info
	// Instead, we can infer from the tailnet name (e.g., "user@github", "example.com")
	if c.tailnet == "" {
		if _, err := c.DetectTailnet(ctx); err != nil {
			return "", err
		}
	}

	// For personal tailnets, the format is typically "user@provider"
	// For organizational tailnets, it's a domain name
	// We'll use "autogroup:admin" as a safe default for tagOwners
	return "autogroup:admin", nil
}

// ensureTailnet checks that tailnet is set, attempts to detect if not
func (c *Client) ensureTailnet(ctx context.Context) error {
	if c.tailnet != "" {
		return nil
	}

	_, err := c.DetectTailnet(ctx)
	return err
}

// normalizeTailnet handles tailnet name variations
// Tailscale API accepts both "user@github" and "user@github.com" formats
func normalizeTailnet(tailnet string) string {
	// The API is flexible, so we don't need to do much normalization
	return strings.TrimSpace(tailnet)
}
