package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/anoldguy/tse/cmd/tse/ui"
	"github.com/anoldguy/tse/shared/tailscale"
)

const setupUsage = `Usage: tse setup --tailnet <name> [flags]

Configure Tailscale for TSE ephemeral exit nodes

This command automates the Tailscale account configuration:
  - Adds tag:exitnode to your ACL policy with auto-approval
  - Tells you how to create the OAuth client the Lambda needs

Exit nodes authenticate with an OAuth client secret rather than a stored auth
key. Auth keys cap at 90 days and cannot be renewed; OAuth client secrets do
not expire, and Tailscale mints a fresh single-use key per node from them.

Prerequisites:
  - TAILSCALE_API_TOKEN environment variable (API access token)
  - You must be an Owner or Admin on your Tailscale network
  - Create token at: https://login.tailscale.com/admin/settings/keys

Required Flags:
  --tailnet string      Your tailnet name (e.g., yourname@github or example.com)
                        Find it by running: tailscale status

Optional Flags:
  --status              Check configuration status without changes
  --show-acl-changes    Preview ACL changes without applying
  --skip-acl            Skip ACL configuration

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
	showACLChanges := fs.Bool("show-acl-changes", false, "Preview ACL changes without applying")
	skipACL := fs.Bool("skip-acl", false, "Skip ACL configuration")
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

	fmt.Println(ui.Title("TSE Setup - Configuring Tailscale for ephemeral exit nodes"))
	fmt.Println(ui.Subtle("============================================================"))
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

	// The OAuth client has to be created by hand; there is no API for it.
	displayOAuthInstructions()

	// Success summary
	fmt.Println()
	fmt.Println(ui.Success("Setup complete! 🎉"))
	fmt.Println()
	fmt.Println(ui.Bold("Next steps:"))
	fmt.Println(ui.Info("1. Add TAILSCALE_OAUTH_SECRET to your .env file"))
	fmt.Println(ui.Info("2. Deploy Lambda: ./bin/tse deploy"))
	fmt.Println(ui.Info("3. Save TSE_AUTH_TOKEN and TSE_LAMBDA_URL from deploy output to .env"))
	fmt.Println(ui.Info("4. Test: ./bin/tse ohio start"))
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

	// Check for auth key in environment
	fmt.Println()
	if secret := os.Getenv("TAILSCALE_OAUTH_SECRET"); secret != "" {
		fmt.Println("✓ TAILSCALE_OAUTH_SECRET found in environment")
		fmt.Println("  You're ready to deploy with: tse deploy")
	} else {
		fmt.Println("⚠️  TAILSCALE_OAUTH_SECRET not set in environment")
		fmt.Println("  After creating an OAuth client, add its secret to your .env file")
	}

	fmt.Println()
	return nil
}

func configureACL(ctx context.Context, client *tailscale.Client, owner string, previewOnly bool) error {
	fmt.Println("Step 1/2: Configuring ACL policy")

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

func displayOAuthInstructions() {
	fmt.Println(ui.Bold("Step 2/2: Create an OAuth client"))
	fmt.Println()

	content := []string{
		"OAuth clients can only be created in the admin console;",
		"there is no API for it. This is a one-time step.",
		"",
		"1. Open https://login.tailscale.com/admin/settings/oauth",
		"2. Generate a client with the auth_keys scope (write)",
		"3. Assign the tag:exitnode tag to that scope",
		"4. Copy the secret; it is shown only once",
		"",
		"Add to your .env file:",
		"",
		"TAILSCALE_OAUTH_SECRET=tskey-client-...",
		"",
		"Then deploy with: tse deploy",
	}
	fmt.Println(ui.HighlightBox("Your Tailscale OAuth Client", content...))
}
