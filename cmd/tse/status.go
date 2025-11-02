package main

import (
	"context"
	"fmt"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
)

// runStatus displays the current state of TSE infrastructure.
func runStatus(args []string) error {
	ctx := context.Background()
	region := "us-east-2" // TODO: make configurable

	fmt.Printf("Discovering TSE infrastructure in %s...\n\n", region)

	state, err := infrastructure.AutodiscoverInfrastructure(ctx, region)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	if !state.Exists() {
		fmt.Println("No TSE infrastructure found")
		fmt.Println("\nRun 'tse deploy' to create infrastructure")
		return nil
	}

	// Print table header
	fmt.Println("Resource                           Status    Details")
	fmt.Println("-----------------------------------  --------  --------------------------------------------------")

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
		fmt.Println("✓ Infrastructure is complete")
	} else {
		missing := state.Missing()
		fmt.Printf("✗ Infrastructure is incomplete (%d missing)\n", len(missing))
		fmt.Println("\nMissing resources:")
		for _, res := range missing {
			fmt.Printf("  - %s\n", res)
		}
		fmt.Println("\nRun 'tse deploy' to create missing resources")
	}

	return nil
}

// printResourceRow prints a single row in the status table.
func printResourceRow(name string, exists bool, details string) {
	status := "✓ Found"
	if !exists {
		status = "✗ Missing"
		details = ""
	}

	// Pad name to 35 chars, status to 9 chars
	fmt.Printf("%-35s  %-9s %s\n", name, status, details)
}
