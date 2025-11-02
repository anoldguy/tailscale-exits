package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// AuthKeyRequest represents a request to create a new auth key
type AuthKeyRequest struct {
	Capabilities  AuthKeyCapabilities `json:"capabilities"`
	ExpirySeconds int                 `json:"expirySeconds"`
	Description   string              `json:"description,omitempty"`
}

// AuthKeyCapabilities defines what the auth key can do
type AuthKeyCapabilities struct {
	Devices AuthKeyDeviceCapabilities `json:"devices"`
}

// AuthKeyDeviceCapabilities defines device-related capabilities
type AuthKeyDeviceCapabilities struct {
	Create AuthKeyDeviceCreate `json:"create"`
}

// AuthKeyDeviceCreate defines settings for devices created with this key
type AuthKeyDeviceCreate struct {
	Reusable      bool     `json:"reusable"`
	Ephemeral     bool     `json:"ephemeral"`
	Tags          []string `json:"tags"`
	Preauthorized bool     `json:"preauthorized"`
}

// AuthKeyResponse represents the response when creating an auth key
type AuthKeyResponse struct {
	ID           string              `json:"id"`
	Key          string              `json:"key"`
	Created      string              `json:"created"`
	Expires      string              `json:"expires"`
	Capabilities AuthKeyCapabilities `json:"capabilities"`
	Description  string              `json:"description,omitempty"`
}

// CreateAuthKey creates a new auth key with the specified capabilities
func (c *Client) CreateAuthKey(ctx context.Context, req *AuthKeyRequest) (*AuthKeyResponse, error) {
	if err := c.ensureTailnet(ctx); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, fmt.Errorf("auth key request cannot be nil")
	}

	path := fmt.Sprintf("/tailnet/%s/keys", normalizeTailnet(c.tailnet))

	resp, err := c.doRequest(ctx, "POST", path, req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth key: %w", err)
	}
	defer resp.Body.Close()

	if err := handleResponse(resp, http.StatusOK); err != nil {
		return nil, fmt.Errorf("failed to create auth key: %w", err)
	}

	var authKey AuthKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&authKey); err != nil {
		return nil, fmt.Errorf("failed to parse auth key response: %w", err)
	}

	return &authKey, nil
}

// NewExitNodeAuthKeyRequest creates an auth key request configured for exit nodes
func NewExitNodeAuthKeyRequest() *AuthKeyRequest {
	return &AuthKeyRequest{
		Capabilities: AuthKeyCapabilities{
			Devices: AuthKeyDeviceCapabilities{
				Create: AuthKeyDeviceCreate{
					Reusable:      true,
					Ephemeral:     true,
					Tags:          []string{"tag:exitnode"},
					Preauthorized: true,
				},
			},
		},
		ExpirySeconds: 0, // Never expire
		Description:   "TSE ephemeral exit node auth key",
	}
}
