package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetACL fetches the current ACL policy with ETag for collision avoidance
func (c *Client) GetACL(ctx context.Context) (*ACLResponse, error) {
	if err := c.ensureTailnet(ctx); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/tailnet/%s/acl", normalizeTailnet(c.tailnet))

	// Request JSON format for easier parsing
	headers := map[string]string{
		"Accept": "application/json",
	}

	resp, err := c.doRequest(ctx, "GET", path, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACL: %w", err)
	}
	defer resp.Body.Close()

	if err := handleResponse(resp, http.StatusOK); err != nil {
		return nil, fmt.Errorf("failed to get ACL: %w", err)
	}

	// Extract ETag header
	etag := resp.Header.Get("ETag")

	// Parse ACL policy from response
	var policy ACLPolicy
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse ACL policy: %w", err)
	}

	return &ACLResponse{
		ACL:  &policy,
		ETag: etag,
	}, nil
}

// UpdateACL updates the ACL policy using ETag for collision avoidance
// Returns error if the ACL was modified since the ETag was retrieved (412 Precondition Failed)
func (c *Client) UpdateACL(ctx context.Context, policy *ACLPolicy, etag string) error {
	if err := c.ensureTailnet(ctx); err != nil {
		return err
	}

	if policy == nil {
		return fmt.Errorf("ACL policy cannot be nil")
	}

	path := fmt.Sprintf("/tailnet/%s/acl", normalizeTailnet(c.tailnet))

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Include If-Match header for ETag-based collision avoidance
	if etag != "" {
		headers["If-Match"] = etag
	}

	resp, err := c.doRequest(ctx, "POST", path, policy, headers)
	if err != nil {
		return fmt.Errorf("failed to update ACL: %w", err)
	}
	defer resp.Body.Close()

	if err := handleResponse(resp, http.StatusOK); err != nil {
		return fmt.Errorf("failed to update ACL: %w", err)
	}

	return nil
}

// ValidateACL validates an ACL policy without applying it
// Returns nil if the ACL is valid, error otherwise
func (c *Client) ValidateACL(ctx context.Context, policy *ACLPolicy) error {
	if err := c.ensureTailnet(ctx); err != nil {
		return err
	}

	if policy == nil {
		return fmt.Errorf("ACL policy cannot be nil")
	}

	path := fmt.Sprintf("/tailnet/%s/acl/validate", normalizeTailnet(c.tailnet))

	resp, err := c.doRequest(ctx, "POST", path, policy, nil)
	if err != nil {
		return fmt.Errorf("failed to validate ACL: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for validation errors
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read validation response: %w", err)
	}

	// A valid ACL returns {} (empty JSON object)
	// An invalid ACL returns error details
	if resp.StatusCode != http.StatusOK {
		var errMsg struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(body, &errMsg); err == nil {
			msg := errMsg.Message
			if msg == "" {
				msg = errMsg.Error
			}
			return fmt.Errorf("ACL validation failed: %s", msg)
		}
		return fmt.Errorf("ACL validation failed: %s", string(body))
	}

	// Check if response is empty object (valid) or has error details
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse validation response: %w", err)
	}

	if len(result) > 0 {
		// Response contains validation errors
		return fmt.Errorf("ACL validation failed: %v", result)
	}

	return nil
}
