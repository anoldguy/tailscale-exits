package aws

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateUserData(t *testing.T) {
	tests := []struct {
		name           string
		oauthSecret    string
		friendlyRegion string
	}{
		{
			name:           "ohio region",
			oauthSecret:    "tskey-client-test123",
			friendlyRegion: "ohio",
		},
		{
			name:           "virginia region",
			oauthSecret:    "tskey-client-different456",
			friendlyRegion: "virginia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUserData(tt.oauthSecret, tt.friendlyRegion)

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
				"pkgs.tailscale.com/stable/",
				"systemctl enable --now tailscaled",
				"tailscale up",
				"--auth-key='" + tt.oauthSecret + "?ephemeral=true&preauthorized=true'",
				"--advertise-tags=tag:exitnode",
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

			// The package-manager install path OOM-kills dnf on a 512MB t4g.nano,
			// so the static tarball is the only install method that fits.
			forbidden := []string{
				"tailscale.com/install.sh",
				"yum install",
				"dnf install",
			}

			for _, banned := range forbidden {
				if strings.Contains(script, banned) {
					t.Errorf("generateUserData script must not use a package manager: %s", banned)
				}
			}

			// IP forwarding must be enabled before we advertise as an exit node,
			// otherwise tailscaled warns and the node comes up unable to route.
			if strings.Index(script, "net.ipv4.ip_forward = 1") > strings.Index(script, "tailscale up") {
				t.Errorf("generateUserData script should enable IP forwarding before 'tailscale up'")
			}

			// Should start with shebang
			if !strings.HasPrefix(script, "#!/bin/bash") {
				t.Errorf("generateUserData script should start with #!/bin/bash")
			}

			// Should have set -e for error handling
			if !strings.Contains(script, "set -e") {
				t.Errorf("generateUserData script should contain 'set -e' for error handling")
			}

			// A silent failure is what made the last outage hard to find; the
			// script must name the line that died in cloud-init-output.log.
			if !strings.Contains(script, "trap") {
				t.Errorf("generateUserData script should trap errors and log the failing line")
			}
		})
	}
}

func TestGenerateUserDataTemplateSubstitution(t *testing.T) {
	// Test that template substitution works correctly
	oauthSecret := "tskey-client-test123"
	friendlyRegion := "ohio"

	result := generateUserData(oauthSecret, friendlyRegion)
	decoded, err := base64.StdEncoding.DecodeString(result)
	if err != nil {
		t.Fatalf("generateUserData returned invalid base64: %v", err)
	}

	script := string(decoded)

	// The auth key should be inserted directly by the template
	expectedSecret := "--auth-key='" + oauthSecret + "?ephemeral=true&preauthorized=true'"
	if !strings.Contains(script, expectedSecret) {
		t.Errorf("generateUserData should contain OAuth secret: %s", expectedSecret)
	}

	// The query string separator must stay inside single quotes. Unquoted, bash
	// reads the & as "background this command" and tailscale up never sees the
	// preauthorized flag.
	if strings.Contains(script, "&preauthorized") && !strings.Contains(script, "'"+oauthSecret+"?ephemeral=true&preauthorized=true'") {
		t.Errorf("OAuth secret query string must be single-quoted so bash does not background on &")
	}

	// The hostname should be inserted directly by the template
	expectedHostname := "--hostname=exit-" + friendlyRegion
	if !strings.Contains(script, expectedHostname) {
		t.Errorf("generateUserData should contain hostname: %s", expectedHostname)
	}

	// Should contain the region in the logger message
	expectedLog := "Tailscale exit node setup complete for region: " + friendlyRegion
	if !strings.Contains(script, expectedLog) {
		t.Errorf("generateUserData should contain log message with region: %s", expectedLog)
	}
}

func TestGenerateUserDataEmptyInputs(t *testing.T) {
	tests := []struct {
		name           string
		oauthSecret    string
		friendlyRegion string
	}{
		{
			name:           "empty auth key",
			oauthSecret:    "",
			friendlyRegion: "ohio",
		},
		{
			name:           "empty region",
			oauthSecret:    "tskey-client-test123",
			friendlyRegion: "",
		},
		{
			name:           "both empty",
			oauthSecret:    "",
			friendlyRegion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateUserData(tt.oauthSecret, tt.friendlyRegion)

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
			expectedSecret := "--auth-key='" + tt.oauthSecret + "?ephemeral=true&preauthorized=true'"
			if !strings.Contains(script, expectedSecret) {
				t.Errorf("generateUserData script should contain OAuth secret parameter: %s", expectedSecret)
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
