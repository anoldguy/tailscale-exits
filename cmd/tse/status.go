package main

import (
	"context"
	"fmt"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
	"github.com/anoldguy/tse/cmd/tse/ui"
)

// runStatus displays the current state of TSE infrastructure.
func runStatus(args []string) error {
	ctx := context.Background()

	// Get default AWS region from user's configuration
	region, err := infrastructure.GetDefaultRegion(ctx)
	if err != nil {
		return fmt.Errorf("failed to determine AWS region: %w", err)
	}

	var state *infrastructure.InfrastructureState
	err = ui.WithSpinner(fmt.Sprintf("Discovering infrastructure in %s", region), func() error {
		var err error
		state, err = infrastructure.AutodiscoverInfrastructure(ctx, region)
		return err
	})
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}
	fmt.Println()

	if !state.Exists() {
		fmt.Println(ui.Subtle("No TSE infrastructure found"))
		fmt.Printf("\n%s Run 'tse deploy' to create infrastructure\n", ui.Info("→"))
		return nil
	}

	// Build table
	table := ui.NewTable("Resource", "Status", "Details")

	// CloudWatch Log Group
	addResourceRow(table, "CloudWatch Log Group", state.LogGroup != nil, func() string {
		if state.LogGroup != nil {
			return state.LogGroup.Name
		}
		return ""
	}())

	// IAM Role
	addResourceRow(table, "IAM Role", state.IAMRole != nil, func() string {
		if state.IAMRole != nil {
			return state.IAMRole.Name
		}
		return ""
	}())

	// Managed Policy Attachment
	addResourceRow(table, "Managed Policy Attachment", state.Policies.Managed, func() string {
		if state.Policies.Managed {
			return "AWSLambdaBasicExecutionRole"
		}
		return ""
	}())

	// Inline Policy
	addResourceRow(table, "Inline Policy", state.Policies.InlineName != "", state.Policies.InlineName)

	// Lambda Function
	addResourceRow(table, "Lambda Function", state.Lambda != nil, func() string {
		if state.Lambda != nil {
			return state.Lambda.Name
		}
		return ""
	}())

	// Function URL
	addResourceRow(table, "Function URL", state.FunctionURL != "", state.FunctionURL)

	// Render table
	fmt.Println(table.Render())

	// Print summary
	fmt.Println()
	if state.IsComplete() {
		fmt.Println(ui.Success("✓ Infrastructure is complete"))
	} else {
		missing := state.Missing()
		fmt.Printf("%s Infrastructure is incomplete (%s missing)\n",
			ui.Error("✗"),
			ui.Bold(fmt.Sprintf("%d", len(missing))))
		fmt.Println(ui.Bold("\nMissing resources:"))
		for _, res := range missing {
			fmt.Printf("  %s %s\n", ui.Error("-"), res)
		}

		// Check for IAM-only resources (wrong region indicator)
		if state.HasOnlyIAMResources() {
			fmt.Println()
			fmt.Println(ui.Warning("⚠️  Found IAM resources but no Lambda/logs"))
			fmt.Println(ui.Subtle("   IAM is global, but Lambda and CloudWatch are regional."))
			fmt.Println(ui.Subtle("   You might have infrastructure in a different region."))
			fmt.Println()
			fmt.Printf("%s Check your AWS region: %s\n", ui.Info("→"), ui.Highlight(region))
			fmt.Printf("%s Change region: %s or %s\n",
				ui.Info("→"),
				ui.Subtle("aws configure"),
				ui.Subtle("export AWS_REGION=us-east-2"))
		}

		fmt.Printf("\n%s Run 'tse deploy' to create missing resources\n", ui.Info("→"))
	}

	return nil
}

// addResourceRow adds a resource row to the table with proper styling
func addResourceRow(table *ui.Table, name string, exists bool, details string) {
	var status string
	if exists {
		status = ui.Success("✓ Found")
	} else {
		status = ui.Error("✗ Missing")
		details = ""
	}

	// Highlight important details (URLs, resource names)
	if details != "" {
		details = ui.Subtle(details)
	}

	table.AddRow(name, status, details)
}
