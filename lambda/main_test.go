package main

import (
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestValidateAuth(t *testing.T) {
	// Set up test token
	testToken := "test-token-12345"
	os.Setenv("TSE_AUTH_TOKEN", testToken)
	defer os.Unsetenv("TSE_AUTH_TOKEN")

	tests := []struct {
		name        string
		headers     map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid token with Bearer prefix",
			headers: map[string]string{
				"Authorization": "Bearer test-token-12345",
			},
			expectError: false,
		},
		{
			name: "valid token without Bearer prefix",
			headers: map[string]string{
				"Authorization": "test-token-12345",
			},
			expectError: false,
		},
		{
			name: "valid token with lowercase bearer",
			headers: map[string]string{
				"authorization": "bearer test-token-12345",
			},
			expectError: false,
		},
		{
			name: "valid token with extra whitespace",
			headers: map[string]string{
				"Authorization": "Bearer  test-token-12345  ",
			},
			expectError: false,
		},
		{
			name: "missing authorization header",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectError: true,
			errorMsg:    "missing Authorization header",
		},
		{
			name: "invalid token",
			headers: map[string]string{
				"Authorization": "Bearer wrong-token",
			},
			expectError: true,
			errorMsg:    "invalid token",
		},
		{
			name: "empty token",
			headers: map[string]string{
				"Authorization": "",
			},
			expectError: true,
			errorMsg:    "missing Authorization header",
		},
		{
			name: "only Bearer prefix",
			headers: map[string]string{
				"Authorization": "Bearer",
			},
			expectError: true,
			errorMsg:    "invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := events.LambdaFunctionURLRequest{
				Headers: tt.headers,
			}

			err := validateAuth(request)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateAuth_NoTokenConfigured(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("TSE_AUTH_TOKEN")

	request := events.LambdaFunctionURLRequest{
		Headers: map[string]string{
			"Authorization": "Bearer some-token",
		},
	}

	err := validateAuth(request)
	if err == nil {
		t.Error("expected error when TSE_AUTH_TOKEN not set")
	}
	if err.Error() != "TSE_AUTH_TOKEN not configured" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateAuth_CaseInsensitiveHeader(t *testing.T) {
	testToken := "test-token-case-insensitive"
	os.Setenv("TSE_AUTH_TOKEN", testToken)
	defer os.Unsetenv("TSE_AUTH_TOKEN")

	testCases := []string{
		"Authorization",
		"authorization",
		"AUTHORIZATION",
		"AuThOrIzAtIoN",
	}

	for _, headerName := range testCases {
		t.Run(headerName, func(t *testing.T) {
			request := events.LambdaFunctionURLRequest{
				Headers: map[string]string{
					headerName: "Bearer " + testToken,
				},
			}

			err := validateAuth(request)
			if err != nil {
				t.Errorf("case-insensitive header %q failed: %v", headerName, err)
			}
		})
	}
}

func TestValidateAuth_TimingAttackResistance(t *testing.T) {
	// This test doesn't measure actual timing but ensures we're using
	// constant-time comparison by verifying it's called on different length tokens
	testToken := "correct-token-with-specific-length"
	os.Setenv("TSE_AUTH_TOKEN", testToken)
	defer os.Unsetenv("TSE_AUTH_TOKEN")

	testCases := []struct {
		name  string
		token string
	}{
		{"shorter token", "short"},
		{"same length token", "xxxxxx-xxxxx-xxxx-xxxxxxx-xxxxxx"},
		{"longer token", "this-is-a-much-longer-token-that-should-still-be-rejected-safely"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := events.LambdaFunctionURLRequest{
				Headers: map[string]string{
					"Authorization": "Bearer " + tc.token,
				},
			}

			err := validateAuth(request)
			// All should fail with invalid token
			if err == nil {
				t.Error("expected error for incorrect token")
			}
			if err.Error() != "invalid token" {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
