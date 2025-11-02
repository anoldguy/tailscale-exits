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

	fmt.Printf("%s %s...\n\n", ui.Info("Discovering TSE infrastructure in"), ui.Highlight(region))

	state, err := infrastructure.AutodiscoverInfrastructure(ctx, region)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if !state.Exists() {
		fmt.Println(ui.Subtle("No TSE infrastructure found"))
		fmt.Printf("\n%s Run 'tse deploy' to create infrastructure\n", ui.Info("→"))
		return nil
	}

	// Print table header
	fmt.Println(ui.Bold("Resource                           Status    Details"))
	fmt.Println(ui.Subtle("-----------------------------------  --------  --------------------------------------------------"))

	// CloudWatch Log Group
	printResourceRow("CloudWatch Log Group", state.LogGroup != nil, func() string {
		if state.LogGroup != nil {
			return state.LogGroup.Name
		}
		return ""
	}())

	// IAM Role
	printResourceRow("IAM Role", state.IAMRole != nil, func() string {
		if state.IAMRole != nil {
			return state.IAMRole.Name
		}
		return ""
	}())

	// Managed Policy Attachment
	printResourceRow("Managed Policy Attachment", state.Policies.Managed, func() string {
		if state.Policies.Managed {
			return "AWSLambdaBasicExecutionRole"
		}
		return ""
	}())

	// Inline Policy
	printResourceRow("Inline Policy", state.Policies.InlineName != "", func() string {
		return state.Policies.InlineName
	}())

	// Lambda Function
	printResourceRow("Lambda Function", state.Lambda != nil, func() string {
		if state.Lambda != nil {
			return state.Lambda.Name
		}
		return ""
	}())

	// Function URL
	printResourceRow("Function URL", state.FunctionURL != "", state.FunctionURL)

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

// printResourceRow prints a single row in the status table.
func printResourceRow(name string, exists bool, details string) {
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

	// Pad name to 35 chars
	fmt.Printf("%-35s  %s  %s\n", name, status, details)
}
