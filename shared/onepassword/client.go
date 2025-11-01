package onepassword

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// DefaultAuthKeyPath is the default 1Password reference for TSE auth key
	DefaultAuthKeyPath = "op://private/Tailscale/CurrentAuthKey"
)

// IsInstalled checks if the 1Password CLI is available
func IsInstalled() bool {
	_, err := exec.LookPath("op")
	return err == nil
}

// Store stores a value at a 1Password reference path
// The path format is: op://vault/item/field
func Store(ctx context.Context, path, value string) error {
	if !IsInstalled() {
		return fmt.Errorf("1Password CLI (op) is not installed")
	}

	// Parse the path to extract vault, item, and field
	// Format: op://vault/item/field
	parts := strings.TrimPrefix(path, "op://")
	pathParts := strings.Split(parts, "/")
	if len(pathParts) != 3 {
		return fmt.Errorf("invalid 1Password path format: %s (expected op://vault/item/field)", path)
	}

	vault := pathParts[0]
	item := pathParts[1]
	field := pathParts[2]

	// Use `op item edit` to store the value
	// First, check if the item exists
	checkCmd := exec.CommandContext(ctx, "op", "item", "get", item, "--vault", vault)
	if err := checkCmd.Run(); err != nil {
		// Item doesn't exist, create it
		createCmd := exec.CommandContext(ctx, "op", "item", "create",
			"--category", "password",
			"--title", item,
			"--vault", vault,
			fmt.Sprintf("%s=%s", field, value))

		output, err := createCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create 1Password item: %w\nOutput: %s", err, string(output))
		}
		return nil
	}

	// Item exists, edit it
	editCmd := exec.CommandContext(ctx, "op", "item", "edit", item,
		"--vault", vault,
		fmt.Sprintf("%s=%s", field, value))

	output, err := editCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update 1Password item: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Retrieve retrieves a value from a 1Password reference path
func Retrieve(ctx context.Context, path string) (string, error) {
	if !IsInstalled() {
		return "", fmt.Errorf("1Password CLI (op) is not installed")
	}

	// Use `op read` to retrieve the value
	cmd := exec.CommandContext(ctx, "op", "read", path)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve from 1Password: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Verify checks if a value can be retrieved from 1Password
func Verify(ctx context.Context, path string) error {
	_, err := Retrieve(ctx, path)
	return err
}
