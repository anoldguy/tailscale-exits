package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/anoldguy/tse/shared/onepassword"
	"github.com/anoldguy/tse/shared/tailscale"
)

const setupUsage = `Usage: tse setup --tailnet <name> [flags]

Configure Tailscale for TSE ephemeral exit nodes

This command automates the Tailscale account configuration:
  - Adds tag:exitnode to your ACL policy with auto-approval
  - Creates a reusable, ephemeral auth key
  - Optionally stores the auth key in 1Password

Prerequisites:
  - TAILSCALE_API_TOKEN environment variable (API access token)
  - You must be an Owner or Admin on your Tailscale network
  - Create token at: https://login.tailscale.com/admin/settings/keys

Required Flags:
  --tailnet string      Your tailnet name (e.g., yourname@github or example.com)
                        Find it by running: tailscale status

Optional Flags:
  --status              Check configuration status without changes
  --show-auth-key       Display auth key instead of storing
  --show-acl-changes    Preview ACL changes without applying
  --skip-acl            Skip ACL configuration
  --skip-auth-key       Skip auth key creation

Examples:
  tse setup --tailnet yourname@github              # Full automated setup
  tse setup --tailnet example.com --status         # Check current configuration
  tse setup --tailnet yourname@github --show-acl-changes  # Preview changes
`

func runSetup(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, setupUsage)
	}

	statusOnly := fs.Bool("status", false, "Check configuration status without making changes")
	showAuthKey := fs.Bool("show-auth-key", false, "Display auth key instead of storing in 1Password")
	showACLChanges := fs.Bool("show-acl-changes", false, "Preview ACL changes without applying")
	skipACL := fs.Bool("skip-acl", false, "Skip ACL configuration")
	skipAuthKey := fs.Bool("skip-auth-key", false, "Skip auth key creation")
	tailnetOverride := fs.String("tailnet", "", "Override tailnet detection")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()

	// Check for API token
	apiToken := os.Getenv("TAILSCALE_API_TOKEN")
	if apiToken == "" {
		return fmt.Errorf(`TAILSCALE_API_TOKEN environment variable not set

To create an API token:
1. Visit: https://login.tailscale.com/admin/settings/keys
2. Click "Generate API key"
3. Give it a description (e.g., "TSE Setup")
4. Set expiration (90 days recommended)
5. Copy the token (starts with tskey-api-)
6. Run: export TAILSCALE_API_TOKEN=tskey-api-xxxxx
7. Run: tse setup again

Note: You must be an Owner or Admin on your Tailscale network.`)
	}

	fmt.Println("TSE Setup - Configuring Tailscale for ephemeral exit nodes")
	fmt.Println("============================================================")
	fmt.Println()

	// Create Tailscale client
	client, err := tailscale.NewClient(apiToken)
	if err != nil {
		return fmt.Errorf("failed to create Tailscale client: %w", err)
	}

	// Set or detect tailnet
	if *tailnetOverride != "" {
		client.SetTailnet(*tailnetOverride)
		fmt.Printf("✓ Using tailnet: %s\n", *tailnetOverride)
	} else {
		// Tailnet auto-detection isn't supported by the API
		// Prompt user for their tailnet name
		return fmt.Errorf(`tailnet name required

Please specify your tailnet with the --tailnet flag.

Your tailnet name is either:
  - Your email-based tailnet (e.g., yourname@github)
  - Your organization's domain (e.g., example.com)

Find it in your Tailscale admin console URL or run: tailscale status

Example: tse setup --tailnet yourname@github`)
	}

	// Get current user/owner for tagOwners
	owner, err := client.GetCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	fmt.Println()

	// Status check mode
	if *statusOnly {
		return runStatusCheck(ctx, client)
	}

	// ACL configuration
	if !*skipACL {
		if err := configureACL(ctx, client, owner, *showACLChanges); err != nil {
			return err
		}
	} else {
		fmt.Println("Skipping ACL configuration (--skip-acl)")
	}

	fmt.Println()

	// Auth key creation
	var authKey string
	if !*skipAuthKey {
		key, err := createAuthKey(ctx, client)
		if err != nil {
			return err
		}
		authKey = key
	} else {
		fmt.Println("Skipping auth key creation (--skip-auth-key)")
	}

	fmt.Println()

	// Auth key storage
	if authKey != "" {
		if err := storeAuthKey(ctx, authKey, *showAuthKey); err != nil {
			return err
		}
	}

	// Success summary
	fmt.Println()
	fmt.Println("Setup complete! 🎉")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Initialize OpenTofu (first time only): make tofu-init")
	fmt.Println("2. Deploy Lambda: make deploy")
	fmt.Println("3. Set Lambda URL: export TSE_LAMBDA_URL=$(cd deployments/opentofu && tofu output -raw lambda_function_url)")
	fmt.Println("4. Test: tse ohio start")
	fmt.Println()

	return nil
}

func runStatusCheck(ctx context.Context, client *tailscale.Client) error {
	fmt.Println("Checking current configuration...")
	fmt.Println()

	// Fetch current ACL
	fmt.Print("Fetching ACL policy...")
	aclResp, err := client.GetACL(ctx)
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("failed to fetch ACL: %w", err)
	}
	fmt.Println(" done")

	// Check configuration
	if err := tailscale.ValidateExitNodeConfig(aclResp.ACL); err != nil {
		fmt.Println()
		fmt.Println("❌ ACL not configured for exit nodes")
		fmt.Println(err.Error())
		fmt.Println()
		fmt.Println("Run 'tse setup' (without --status) to configure")
		return nil
	}

	fmt.Println()
	fmt.Println("✓ ACL properly configured for exit nodes")

	// Check for tagOwners
	owners := tailscale.GetTagOwners(aclResp.ACL, "tag:exitnode")
	fmt.Printf("  - tag:exitnode owners: %s\n", strings.Join(owners, ", "))
	fmt.Println("  - Exit node auto-approval: enabled")

	// Check for auth key in 1Password
	fmt.Println()
	if onepassword.IsInstalled() {
		fmt.Print("Checking 1Password for auth key...")
		if err := onepassword.Verify(ctx, onepassword.DefaultAuthKeyPath); err != nil {
			fmt.Println(" not found")
			fmt.Printf("  Auth key not found at: %s\n", onepassword.DefaultAuthKeyPath)
		} else {
			fmt.Println(" found")
			fmt.Printf("  ✓ Auth key stored at: %s\n", onepassword.DefaultAuthKeyPath)
		}
	} else {
		fmt.Println("1Password CLI not installed (skipping auth key check)")
	}

	fmt.Println()
	return nil
}

func configureACL(ctx context.Context, client *tailscale.Client, owner string, previewOnly bool) error {
	fmt.Println("Step 1/3: Configuring ACL policy")

	// Fetch current ACL
	fmt.Print("✓ Fetching current ACL policy...")
	aclResp, err := client.GetACL(ctx)
	if err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("failed to fetch ACL: %w", err)
	}
	fmt.Println(" done")

	// Preview changes
	if previewOnly {
		fmt.Println()
		fmt.Println("ACL changes that would be applied:")
		preview := tailscale.PreviewChanges(aclResp.ACL, owner)
		for _, line := range preview {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
		fmt.Println("Run without --show-acl-changes to apply these changes")
		os.Exit(0)
	}

	// Apply changes
	changes, modified := tailscale.ConfigureForExitNodes(aclResp.ACL, owner)
	for _, change := range changes {
		if strings.HasPrefix(change, "✓") {
			fmt.Printf("  %s\n", change)
		} else {
			fmt.Printf("✓ %s\n", change)
		}
	}

	if !modified {
		fmt.Println("  ACL already configured - no changes needed")
		return nil
	}

	// Validate ACL
	fmt.Print("✓ Validating updated ACL...")
	if err := client.ValidateACL(ctx, aclResp.ACL); err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("ACL validation failed: %w", err)
	}
	fmt.Println(" passed")

	// Apply ACL
	fmt.Print("✓ Applying ACL changes...")
	if err := client.UpdateACL(ctx, aclResp.ACL, aclResp.ETag); err != nil {
		fmt.Println(" failed")

		// Check for common errors
		if apiErr, ok := err.(*tailscale.APIError); ok {
			if apiErr.IsConflict() {
				return fmt.Errorf("ACL was modified by someone else. Please run 'tse setup' again to retry")
			}
			if apiErr.IsPermissionError() {
				return fmt.Errorf(`insufficient permissions

Your API token doesn't have permission to modify ACL policies.
You must be an Owner or Admin on your Tailscale network.

Create a new token at: https://login.tailscale.com/admin/settings/keys`)
			}
		}
		return err
	}
	fmt.Println(" done")

	return nil
}

func createAuthKey(ctx context.Context, client *tailscale.Client) (string, error) {
	fmt.Println("Step 2/3: Creating reusable auth key")

	// Create auth key request
	req := tailscale.NewExitNodeAuthKeyRequest()

	fmt.Print("✓ Creating ephemeral auth key with tag:exitnode...")
	authKeyResp, err := client.CreateAuthKey(ctx, req)
	if err != nil {
		fmt.Println(" failed")

		// Check for permission errors
		if apiErr, ok := err.(*tailscale.APIError); ok && apiErr.IsPermissionError() {
			return "", fmt.Errorf(`insufficient permissions

Your API token doesn't have permission to create auth keys.
You must be an Owner or Admin on your Tailscale network.

Create a new token at: https://login.tailscale.com/admin/settings/keys`)
		}
		return "", err
	}
	fmt.Println(" done")

	fmt.Println("✓ Auth key created (never expires)")

	return authKeyResp.Key, nil
}

func storeAuthKey(ctx context.Context, authKey string, showOnly bool) error {
	fmt.Println("Step 3/3: Storing auth key securely")

	if showOnly {
		fmt.Println()
		fmt.Println("Your Tailscale auth key:")
		fmt.Println(authKey)
		fmt.Println()
		fmt.Println("Store this key securely and configure it in your OpenTofu variables.")
		return nil
	}

	// Check if 1Password is available
	if !onepassword.IsInstalled() {
		fmt.Println()
		fmt.Println("1Password CLI not found.")
		fmt.Println()
		fmt.Println("Your Tailscale auth key:")
		fmt.Println(authKey)
		fmt.Println()
		fmt.Println("To install 1Password CLI: https://developer.1password.com/docs/cli/get-started/")
		fmt.Println("Or store the key manually in your OpenTofu variables.")
		return nil
	}

	// Prompt for storage method
	fmt.Println()
	fmt.Println("How should we store the auth key?")
	fmt.Println("  1. 1Password (recommended)")
	fmt.Println("  2. Display for manual copy")
	fmt.Println("  3. Skip (I'll handle this myself)")
	fmt.Println()
	fmt.Print("Choice [1-3]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1", "":
		// Store in 1Password
		fmt.Printf("✓ Storing in 1Password at: %s\n", onepassword.DefaultAuthKeyPath)
		if err := onepassword.Store(ctx, onepassword.DefaultAuthKeyPath, authKey); err != nil {
			return fmt.Errorf("failed to store in 1Password: %w", err)
		}

		fmt.Print("✓ Verifying storage...")
		if err := onepassword.Verify(ctx, onepassword.DefaultAuthKeyPath); err != nil {
			fmt.Println(" failed")
			return fmt.Errorf("failed to verify storage: %w", err)
		}
		fmt.Println(" done")

	case "2":
		// Display key
		fmt.Println()
		fmt.Println("Your Tailscale auth key:")
		fmt.Println(authKey)
		fmt.Println()
		fmt.Println("Store this key securely and configure it in your OpenTofu variables.")

	case "3":
		// Skip
		fmt.Println("Skipping auth key storage")
		fmt.Println()
		fmt.Println("Note: You'll need to configure the auth key manually in OpenTofu")

	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}

	return nil
}
