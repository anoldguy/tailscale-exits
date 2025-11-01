package regions

import (
	"strings"
	"testing"
)

func TestGetAWSRegion(t *testing.T) {
	tests := []struct {
		name           string
		friendlyName   string
		expectedRegion string
		expectError    bool
	}{
		{
			name:           "valid region ohio",
			friendlyName:   "ohio",
			expectedRegion: "us-east-2",
			expectError:    false,
		},
		{
			name:           "valid region virginia",
			friendlyName:   "virginia",
			expectedRegion: "us-east-1",
			expectError:    false,
		},
		{
			name:           "valid region with spaces",
			friendlyName:   " ohio ",
			expectedRegion: "us-east-2",
			expectError:    false,
		},
		{
			name:           "valid region mixed case",
			friendlyName:   "OHIO",
			expectedRegion: "us-east-2",
			expectError:    false,
		},
		{
			name:         "invalid region",
			friendlyName: "nonexistent",
			expectError:  true,
		},
		{
			name:         "empty region",
			friendlyName: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetAWSRegion(tt.friendlyName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expectedRegion {
				t.Errorf("expected %s, got %s", tt.expectedRegion, result)
			}
		})
	}
}

func TestGetFriendlyName(t *testing.T) {
	tests := []struct {
		name             string
		awsRegion        string
		expectedFriendly string
		expectError      bool
	}{
		{
			name:             "valid AWS region us-east-2",
			awsRegion:        "us-east-2",
			expectedFriendly: "ohio",
			expectError:      false,
		},
		{
			name:             "valid AWS region us-east-1",
			awsRegion:        "us-east-1",
			expectedFriendly: "virginia",
			expectError:      false,
		},
		{
			name:        "invalid AWS region",
			awsRegion:   "invalid-region",
			expectError: true,
		},
		{
			name:        "empty AWS region",
			awsRegion:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetFriendlyName(tt.awsRegion)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expectedFriendly {
				t.Errorf("expected %s, got %s", tt.expectedFriendly, result)
			}
		})
	}
}

func TestIsValidFriendlyName(t *testing.T) {
	tests := []struct {
		name         string
		friendlyName string
		expected     bool
	}{
		{"valid ohio", "ohio", true},
		{"valid virginia", "virginia", true},
		{"valid mixed case", "OHIO", true},
		{"valid with spaces", " ohio ", true},
		{"invalid region", "nonexistent", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidFriendlyName(tt.friendlyName)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsValidAWSRegion(t *testing.T) {
	tests := []struct {
		name      string
		awsRegion string
		expected  bool
	}{
		{"valid us-east-2", "us-east-2", true},
		{"valid us-east-1", "us-east-1", true},
		{"invalid region", "invalid-region", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAWSRegion(tt.awsRegion)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetAvailableRegions(t *testing.T) {
	regions := GetAvailableRegions()

	// Should contain expected regions
	expectedRegions := []string{"ohio", "virginia", "oregon", "california"}
	for _, expected := range expectedRegions {
		if !strings.Contains(regions, expected) {
			t.Errorf("expected regions to contain %s, but got: %s", expected, regions)
		}
	}

	// Should be comma-separated
	if !strings.Contains(regions, ",") {
		t.Errorf("expected comma-separated list, got: %s", regions)
	}
}

func TestBidirectionalMapping(t *testing.T) {
	// Test that every friendly name has a corresponding AWS region and vice versa
	for friendly, aws := range friendlyToAWS {
		// Forward mapping
		result, err := GetAWSRegion(friendly)
		if err != nil {
			t.Errorf("GetAWSRegion failed for %s: %v", friendly, err)
			continue
		}
		if result != aws {
			t.Errorf("GetAWSRegion(%s) = %s, expected %s", friendly, result, aws)
		}

		// Reverse mapping
		reverseFriendly, err := GetFriendlyName(aws)
		if err != nil {
			t.Errorf("GetFriendlyName failed for %s: %v", aws, err)
			continue
		}
		if reverseFriendly != friendly {
			t.Errorf("GetFriendlyName(%s) = %s, expected %s", aws, reverseFriendly, friendly)
		}
	}
}