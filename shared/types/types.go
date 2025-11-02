package types

import "time"

// InstanceInfo represents information about a running exit node instance
type InstanceInfo struct {
	InstanceID        string    `json:"instance_id"`
	Region            string    `json:"region"`
	FriendlyRegion    string    `json:"friendly_region"`
	State             string    `json:"state"`
	PublicIP          string    `json:"public_ip,omitempty"`
	PrivateIP         string    `json:"private_ip,omitempty"`
	LaunchTime        time.Time `json:"launch_time"`
	InstanceType      string    `json:"instance_type"`
	TailscaleHostname string    `json:"tailscale_hostname,omitempty"`
}

// StartRequest represents a request to start an exit node
type StartRequest struct {
	Region string `json:"region"`
}

// StartResponse represents the response from starting an exit node
type StartResponse struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message"`
	Instance *InstanceInfo `json:"instance,omitempty"`
}

// StopRequest represents a request to stop exit nodes in a region
type StopRequest struct {
	Region string `json:"region"`
}

// StopResponse represents the response from stopping exit nodes
type StopResponse struct {
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
	TerminatedCount int      `json:"terminated_count"`
	TerminatedIDs   []string `json:"terminated_ids,omitempty"`
}

// InstancesRequest represents a request to list instances in a region
type InstancesRequest struct {
	Region string `json:"region"`
}

// InstancesResponse represents the response with instance listings
type InstancesResponse struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	Instances []*InstanceInfo `json:"instances"`
	Count     int             `json:"count"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
}
