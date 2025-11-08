package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
	"github.com/anoldguy/tse/cmd/tse/ui"
)

// runDeploy deploys TSE infrastructure to AWS.
func runDeploy(args []string) error {
	// Validate prerequisites
	if os.Getenv("TAILSCALE_AUTH_KEY") == "" {
		return fmt.Errorf(`TAILSCALE_AUTH_KEY environment variable not set

The Lambda function requires a Tailscale auth key to join exit nodes to your network.

To create one:
  1. Run: tse setup --tailnet <your-tailnet>
     This will configure Tailscale and create an auth key automatically.

Or create manually:
  1. Visit: https://login.tailscale.com/admin/settings/keys
  2. Generate an auth key with these settings:
     - Reusable: Yes
     - Ephemeral: Yes
     - Tags: tag:exitnode
     - Pre-authorized: Yes
  3. Set: export TAILSCALE_AUTH_KEY=<your-key>

Then run 'tse deploy' again.`)
	}

	ctx := context.Background()

	// Get default AWS region from user's configuration
	region, err := infrastructure.GetDefaultRegion(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine AWS region: %w", err)
	}

	fmt.Printf("%s %s\n", ui.Label("Region:"), ui.Highlight(region))
	fmt.Println()

	result, err := infrastructure.Setup(ctx, region)
	if err != nil {
		return err
	}

	state := result.State

	// Build success box content conditionally
	successContent := []string{"✨ Your TSE infrastructure is ready!", ""}

	if state.FunctionURL != "" {
		successContent = append(successContent, fmt.Sprintf("Function URL:  %s", state.FunctionURL))
	}

	if state.Lambda != nil {
		successContent = append(successContent, fmt.Sprintf("Lambda ARN:    %s", state.Lambda.ARN))
	}

	if state.IAMRole != nil {
		successContent = append(successContent, fmt.Sprintf("IAM Role:      %s", state.IAMRole.Name))
	}

	successContent = append(successContent, "", "Next: Start an exit node with 'tse ohio start'")

	fmt.Println(ui.SuccessBox("Deployment Complete", successContent...))
	fmt.Println()

	// Show critical export commands in highlight box (only if we have the URL)
	if state.FunctionURL != "" {
		exportTitle := "Copy These Exports"
		if result.WasGenerated {
			exportTitle = "⚠️  SAVE THIS - New Auth Token Generated!"
		}

		exportContent := []string{
			"Add these to your shell or .env file:",
			"",
			fmt.Sprintf("export TSE_LAMBDA_URL=%s", state.FunctionURL),
			fmt.Sprintf("export TSE_AUTH_TOKEN=%s", result.AuthToken),
		}
		fmt.Println(ui.HighlightBox(exportTitle, exportContent...))
	} else {
		// Deployment incomplete - just show auth token
		fmt.Println(ui.Warning("⚠️  Deployment incomplete - some resources failed to create"))
		fmt.Println()
		if result.WasGenerated {
			fmt.Printf("Generated auth token (save this): %s\n", ui.Highlight(result.AuthToken))
		}
	}
	fmt.Println()

	// Next steps
	fmt.Println(ui.Subheader("Next steps:"))
	fmt.Println(ui.Info("  1. Export the variables above"))
	fmt.Println(ui.Info("  2. Test connectivity: tse health"))
	fmt.Println(ui.Info("  3. Start an exit node: tse ohio start"))

	return nil
}
