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
	region := "us-east-2" // TODO: make configurable

	var state *infrastructure.InfrastructureState
	err := ui.WithSpinner("Discovering infrastructure in us-east-2", func() error {
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
