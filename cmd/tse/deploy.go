package main

import (
	"context"
	"fmt"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
	"github.com/anoldguy/tse/cmd/tse/ui"
)

// runDeploy deploys TSE infrastructure to AWS.
func runDeploy(args []string) error {
	// Prerequisites already validated in main.go
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
