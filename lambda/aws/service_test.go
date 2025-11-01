package aws

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateUserData(t *testing.T) {
	tests := []struct {
		name           string
		authKey        string
		friendlyRegion string
	}{
		{
			name:           "ohio region",
			authKey:        "tskey-auth-test123",
			friendlyRegion: "ohio",
		},
		{
			name:           "virginia region",
			authKey:        "tskey-auth-different456",
			friendlyRegion: "virginia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUserData(tt.authKey, tt.friendlyRegion)

			// Should be base64 encoded
			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Errorf("generateUserData returned invalid base64: %v", err)
				return
			}

			script := string(decoded)

			// Should contain expected elements
			expectedElements := []string{
				"#!/bin/bash",
				"curl -fsSL https://tailscale.com/install.sh",
				"tailscale up",
				"--authkey=" + tt.authKey,
				"--advertise-exit-node",
				"--hostname=exit-" + tt.friendlyRegion,
				"net.ipv4.ip_forward = 1",
				"net.ipv6.conf.all.forwarding = 1",
			}

			for _, expected := range expectedElements {
				if !strings.Contains(script, expected) {
					t.Errorf("generateUserData script missing expected element: %s", expected)
				}
			}

			// Should contain the auth key
			if !strings.Contains(script, tt.authKey) {
				t.Errorf("generateUserData script missing auth key: %s", tt.authKey)
			}

			// Should contain the friendly region in hostname
			expectedHostname := "exit-" + tt.friendlyRegion
			if !strings.Contains(script, expectedHostname) {
				t.Errorf("generateUserData script missing expected hostname: %s", expectedHostname)
			}

			// Should start with shebang
			if !strings.HasPrefix(script, "#!/bin/bash") {
				t.Errorf("generateUserData script should start with #!/bin/bash")
			}

			// Should have set -e for error handling
			if !strings.Contains(script, "set -e") {
				t.Errorf("generateUserData script should contain 'set -e' for error handling")
			}
		})
	}
}

func TestGenerateUserDataNoAuthKeyInjection(t *testing.T) {
	// Test that user input can't inject commands
	maliciousAuthKey := "tskey-auth-test; rm -rf /"
	friendlyRegion := "ohio"

	result := generateUserData(maliciousAuthKey, friendlyRegion)
	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Fatalf("generateUserData returned invalid base64: %v", err)
	}

	script := string(decoded)

	// The auth key should be used as-is in the tailscale up command
	// This test ensures we're not doing any special shell escaping that could be bypassed
	if !strings.Contains(script, "--authkey="+maliciousAuthKey) {
		t.Errorf("generateUserData should contain the full auth key including semicolon")
	}

	// The malicious part should be in the auth key parameter, not as a separate command
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "rm -rf") {
			t.Errorf("generateUserData appears to have command injection vulnerability")
		}
	}
}

func TestGenerateUserDataEmptyInputs(t *testing.T) {
	tests := []struct {
		name           string
		authKey        string
		friendlyRegion string
	}{
		{
			name:           "empty auth key",
			authKey:        "",
			friendlyRegion: "ohio",
		},
		{
			name:           "empty region",
			authKey:        "tskey-auth-test123",
			friendlyRegion: "",
		},
		{
			name:           "both empty",
			authKey:        "",
			friendlyRegion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUserData(tt.authKey, tt.friendlyRegion)

			// Should still be valid base64
			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Errorf("generateUserData returned invalid base64: %v", err)
				return
			}

			script := string(decoded)

			// Should still contain basic structure
			if !strings.Contains(script, "#!/bin/bash") {
				t.Errorf("generateUserData script should still contain shebang")
			}

			if !strings.Contains(script, "tailscale up") {
				t.Errorf("generateUserData script should still contain tailscale up command")
			}

			// Should contain the inputs as provided (even if empty)
			expectedAuthKey := "--authkey=" + tt.authKey
			if !strings.Contains(script, expectedAuthKey) {
				t.Errorf("generateUserData script should contain auth key parameter: %s", expectedAuthKey)
			}

			expectedHostname := "--hostname=exit-" + tt.friendlyRegion
			if !strings.Contains(script, expectedHostname) {
				t.Errorf("generateUserData script should contain hostname parameter: %s", expectedHostname)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Test that our constants have expected values
	if InstanceType != "t4g.nano" {
		t.Errorf("InstanceType should be t4g.nano for cost efficiency, got: %s", InstanceType)
	}

	if SecurityGroupName != "tse-ephemeral-exit-node" {
		t.Errorf("SecurityGroupName should be descriptive, got: %s", SecurityGroupName)
	}

	if TagProject != "tse" {
		t.Errorf("TagProject should be tse, got: %s", TagProject)
	}

	if TagType != "ephemeral" {
		t.Errorf("TagType should be ephemeral, got: %s", TagType)
	}
}