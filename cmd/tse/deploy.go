package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
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
	region := "us-east-2" // TODO: make configurable via flag or env var

	fmt.Printf("Region: %s\n", region)
	fmt.Println()

	result, err := infrastructure.Setup(ctx, region)
	if err != nil {
		return err
	}

	state := result.State

	// Show deployment summary
	fmt.Println("Deployment Summary:")
	fmt.Println("-------------------")

	if state.FunctionURL != "" {
		fmt.Printf("Function URL: %s\n", state.FunctionURL)
	}

	if state.IAMRole != nil {
		fmt.Printf("IAM Role ARN: %s\n", state.IAMRole.ARN)
	}

	if state.Lambda != nil {
		fmt.Printf("Lambda ARN: %s\n", state.Lambda.ARN)
	}

	// Show auth token (always, whether generated or existing)
	fmt.Println()
	if result.WasGenerated {
		fmt.Println("⚠️  IMPORTANT: New auth token generated!")
		fmt.Println("Save this token - you won't see it again:")
	} else {
		fmt.Println("Auth token (from environment):")
	}
	fmt.Printf("  export TSE_AUTH_TOKEN=%s\n", result.AuthToken)

	fmt.Println()
	fmt.Println("Export these to use the CLI:")
	if state.FunctionURL != "" {
		fmt.Printf("  export TSE_LAMBDA_URL=%s\n", state.FunctionURL)
	}
	fmt.Printf("  export TSE_AUTH_TOKEN=%s\n", result.AuthToken)

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Add the exports above to your .env file")
	fmt.Println("  2. Run: tse health")
	fmt.Println("  3. Start an exit node: tse ohio start")

	return nil
}
