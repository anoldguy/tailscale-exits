package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestInstanceInfoJSONSerialization(t *testing.T) {
	// Create a test instance
	launchTime := time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC)
	instance := InstanceInfo{
		InstanceID:        "i-1234567890abcdef0",
		Region:            "us-east-2",
		FriendlyRegion:    "ohio",
		State:             "running",
		PublicIP:          "1.2.3.4",
		PrivateIP:         "10.0.1.100",
		LaunchTime:        launchTime,
		InstanceType:      "t4g.nano",
		TailscaleHostname: "exit-ohio",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(instance)
	if err != nil {
		t.Fatalf("Failed to marshal InstanceInfo: %v", err)
	}

	// Unmarshal back
	var unmarshaled InstanceInfo
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal InstanceInfo: %v", err)
	}

	// Compare fields
	if unmarshaled.InstanceID != instance.InstanceID {
		t.Errorf("InstanceID mismatch: got %s, want %s", unmarshaled.InstanceID, instance.InstanceID)
	}
	if unmarshaled.Region != instance.Region {
		t.Errorf("Region mismatch: got %s, want %s", unmarshaled.Region, instance.Region)
	}
	if unmarshaled.FriendlyRegion != instance.FriendlyRegion {
		t.Errorf("FriendlyRegion mismatch: got %s, want %s", unmarshaled.FriendlyRegion, instance.FriendlyRegion)
	}
	if unmarshaled.State != instance.State {
		t.Errorf("State mismatch: got %s, want %s", unmarshaled.State, instance.State)
	}
	if unmarshaled.PublicIP != instance.PublicIP {
		t.Errorf("PublicIP mismatch: got %s, want %s", unmarshaled.PublicIP, instance.PublicIP)
	}
	if unmarshaled.PrivateIP != instance.PrivateIP {
		t.Errorf("PrivateIP mismatch: got %s, want %s", unmarshaled.PrivateIP, instance.PrivateIP)
	}
	if !unmarshaled.LaunchTime.Equal(instance.LaunchTime) {
		t.Errorf("LaunchTime mismatch: got %v, want %v", unmarshaled.LaunchTime, instance.LaunchTime)
	}
	if unmarshaled.InstanceType != instance.InstanceType {
		t.Errorf("InstanceType mismatch: got %s, want %s", unmarshaled.InstanceType, instance.InstanceType)
	}
	if unmarshaled.TailscaleHostname != instance.TailscaleHostname {
		t.Errorf("TailscaleHostname mismatch: got %s, want %s", unmarshaled.TailscaleHostname, instance.TailscaleHostname)
	}
}

func TestStartResponseJSONSerialization(t *testing.T) {
	instance := &InstanceInfo{
		InstanceID:     "i-1234567890abcdef0",
		Region:         "us-east-2",
		FriendlyRegion: "ohio",
		State:          "pending",
		InstanceType:   "t4g.nano",
		LaunchTime:     time.Now().UTC(),
	}

	response := StartResponse{
		Success:  true,
		Message:  "Exit node started in ohio region",
		Instance: instance,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal StartResponse: %v", err)
	}

	// Unmarshal back
	var unmarshaled StartResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal StartResponse: %v", err)
	}

	if unmarshaled.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", unmarshaled.Success, response.Success)
	}
	if unmarshaled.Message != response.Message {
		t.Errorf("Message mismatch: got %s, want %s", unmarshaled.Message, response.Message)
	}
	if unmarshaled.Instance == nil {
		t.Errorf("Instance should not be nil")
	} else if unmarshaled.Instance.InstanceID != instance.InstanceID {
		t.Errorf("Instance ID mismatch: got %s, want %s", unmarshaled.Instance.InstanceID, instance.InstanceID)
	}
}

func TestStopResponseJSONSerialization(t *testing.T) {
	response := StopResponse{
		Success:         true,
		Message:         "Terminated 2 instances in ohio region",
		TerminatedCount: 2,
		TerminatedIDs:   []string{"i-1234567890abcdef0", "i-0987654321fedcba0"},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal StopResponse: %v", err)
	}

	// Unmarshal back
	var unmarshaled StopResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal StopResponse: %v", err)
	}

	if unmarshaled.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", unmarshaled.Success, response.Success)
	}
	if unmarshaled.Message != response.Message {
		t.Errorf("Message mismatch: got %s, want %s", unmarshaled.Message, response.Message)
	}
	if unmarshaled.TerminatedCount != response.TerminatedCount {
		t.Errorf("TerminatedCount mismatch: got %d, want %d", unmarshaled.TerminatedCount, response.TerminatedCount)
	}
	if len(unmarshaled.TerminatedIDs) != len(response.TerminatedIDs) {
		t.Errorf("TerminatedIDs length mismatch: got %d, want %d", len(unmarshaled.TerminatedIDs), len(response.TerminatedIDs))
	} else {
		for i, id := range response.TerminatedIDs {
			if unmarshaled.TerminatedIDs[i] != id {
				t.Errorf("TerminatedIDs[%d] mismatch: got %s, want %s", i, unmarshaled.TerminatedIDs[i], id)
			}
		}
	}
}

func TestInstancesResponseJSONSerialization(t *testing.T) {
	instances := []*InstanceInfo{
		{
			InstanceID:     "i-1234567890abcdef0",
			Region:         "us-east-2",
			FriendlyRegion: "ohio",
			State:          "running",
			InstanceType:   "t4g.nano",
			LaunchTime:     time.Now().UTC(),
		},
		{
			InstanceID:     "i-0987654321fedcba0",
			Region:         "us-east-2",
			FriendlyRegion: "ohio",
			State:          "pending",
			InstanceType:   "t4g.nano",
			LaunchTime:     time.Now().UTC(),
		},
	}

	response := InstancesResponse{
		Success:   true,
		Message:   "Found 2 instances in ohio",
		Instances: instances,
		Count:     2,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal InstancesResponse: %v", err)
	}

	// Unmarshal back
	var unmarshaled InstancesResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal InstancesResponse: %v", err)
	}

	if unmarshaled.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", unmarshaled.Success, response.Success)
	}
	if unmarshaled.Message != response.Message {
		t.Errorf("Message mismatch: got %s, want %s", unmarshaled.Message, response.Message)
	}
	if unmarshaled.Count != response.Count {
		t.Errorf("Count mismatch: got %d, want %d", unmarshaled.Count, response.Count)
	}
	if len(unmarshaled.Instances) != len(response.Instances) {
		t.Errorf("Instances length mismatch: got %d, want %d", len(unmarshaled.Instances), len(response.Instances))
	}
}

func TestErrorResponseJSONSerialization(t *testing.T) {
	response := ErrorResponse{
		Success: false,
		Error:   "Instance not found",
		Code:    404,
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", err)
	}

	// Unmarshal back
	var unmarshaled ErrorResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", err)
	}

	if unmarshaled.Success != response.Success {
		t.Errorf("Success mismatch: got %v, want %v", unmarshaled.Success, response.Success)
	}
	if unmarshaled.Error != response.Error {
		t.Errorf("Error mismatch: got %s, want %s", unmarshaled.Error, response.Error)
	}
	if unmarshaled.Code != response.Code {
		t.Errorf("Code mismatch: got %d, want %d", unmarshaled.Code, response.Code)
	}
}

func TestHealthResponseJSONSerialization(t *testing.T) {
	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: "2023-12-01T12:00:00Z",
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal HealthResponse: %v", err)
	}

	// Unmarshal back
	var unmarshaled HealthResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal HealthResponse: %v", err)
	}

	if unmarshaled.Status != response.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshaled.Status, response.Status)
	}
	if unmarshaled.Version != response.Version {
		t.Errorf("Version mismatch: got %s, want %s", unmarshaled.Version, response.Version)
	}
	if unmarshaled.Timestamp != response.Timestamp {
		t.Errorf("Timestamp mismatch: got %s, want %s", unmarshaled.Timestamp, response.Timestamp)
	}
}