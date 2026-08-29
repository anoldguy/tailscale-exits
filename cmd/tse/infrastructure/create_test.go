package infrastructure

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

// TestBuildLambdaZip guards the artifact that both createLambdaFunction and
// updateLambdaCode ship to AWS. The provided.al2023 runtime only looks for an
// entrypoint named "bootstrap" at the root of the zip, and a rename or a
// nested path would deploy cleanly and then fail at every invocation.
func TestBuildLambdaZip(t *testing.T) {
	// buildLambdaZip compiles ./lambda relative to the working directory,
	// so it only works from the project root.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir("../../.."); err != nil {
		t.Fatalf("failed to change to project root: %v", err)
	}
	t.Cleanup(func() { os.Chdir(cwd) })

	zipBytes, err := buildLambdaZip()
	if err != nil {
		t.Fatalf("buildLambdaZip failed: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("buildLambdaZip did not produce a readable zip: %v", err)
	}

	var bootstrap *zip.File
	for _, f := range reader.File {
		if f.Name == "bootstrap" {
			bootstrap = f
		}
	}

	if bootstrap == nil {
		var names []string
		for _, f := range reader.File {
			names = append(names, f.Name)
		}
		t.Fatalf("zip must contain a 'bootstrap' entry at the root, got: %v", names)
	}

	if bootstrap.UncompressedSize64 == 0 {
		t.Errorf("bootstrap entry is empty")
	}
}

// TestMergeLambdaEnvironment covers the update path's environment handling.
// UpdateFunctionConfiguration replaces the whole variable map, so a naive
// write would blank out TSE_AUTH_TOKEN whenever the operator's shell didn't
// happen to have it exported, locking them out of their own Lambda.
func TestMergeLambdaEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		current      map[string]string
		oauthSecret  string
		tseAuthToken string
		want         map[string]string
	}{
		{
			name:         "sets the secret on a fresh function",
			current:      nil,
			oauthSecret:  "tskey-client-new",
			tseAuthToken: "token-abc",
			want: map[string]string{
				"TAILSCALE_OAUTH_SECRET": "tskey-client-new",
				"TSE_AUTH_TOKEN":         "token-abc",
			},
		},
		{
			name: "drops the pre-rename auth key",
			current: map[string]string{
				"TAILSCALE_AUTH_KEY": "tskey-auth-expired",
				"TSE_AUTH_TOKEN":     "token-abc",
			},
			oauthSecret:  "tskey-client-new",
			tseAuthToken: "",
			want: map[string]string{
				"TAILSCALE_OAUTH_SECRET": "tskey-client-new",
				"TSE_AUTH_TOKEN":         "token-abc",
			},
		},
		{
			name: "keeps the deployed token when none is supplied",
			current: map[string]string{
				"TSE_AUTH_TOKEN": "token-deployed",
			},
			oauthSecret:  "tskey-client-new",
			tseAuthToken: "",
			want: map[string]string{
				"TAILSCALE_OAUTH_SECRET": "tskey-client-new",
				"TSE_AUTH_TOKEN":         "token-deployed",
			},
		},
		{
			name: "preserves variables we do not manage",
			current: map[string]string{
				"TSE_AUTH_TOKEN": "token-abc",
				"LOG_LEVEL":      "debug",
			},
			oauthSecret:  "tskey-client-new",
			tseAuthToken: "",
			want: map[string]string{
				"TAILSCALE_OAUTH_SECRET": "tskey-client-new",
				"TSE_AUTH_TOKEN":         "token-abc",
				"LOG_LEVEL":              "debug",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeLambdaEnvironment(tt.current, tt.oauthSecret, tt.tseAuthToken)

			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("key %s: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
