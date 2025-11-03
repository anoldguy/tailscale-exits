package infrastructure

import (
	"context"
	"fmt"

	"github.com/anoldguy/tse/cmd/tse/ui"
)

// Teardown removes all TSE infrastructure in reverse dependency order.
// Returns error only on critical failures; logs warnings for individual resource failures.
func Teardown(ctx context.Context, region string) error {
	// 1. Discover what exists
	var state *InfrastructureState
	err := ui.WithSpinner("Discovering infrastructure to teardown", func() error {
		var err error
		state, err = AutodiscoverInfrastructure(ctx, region)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to discover infrastructure: %w", err)
	}
	fmt.Println()

	if !state.Exists() {
		fmt.Println("No TSE infrastructure found")
		return nil
	}

	// 2. Check for legacy resources (missing ManagedBy tag)
	isLegacy := detectLegacyResources(state)
	if isLegacy {
		fmt.Println("⚠️  Legacy infrastructure detected!")
		fmt.Println("    Resources found without 'ManagedBy=tse' tag.")
		fmt.Println("    This appears to be from an OpenTofu/Terraform deployment.")
		fmt.Println()
	}

	// 3. Show what will be deleted
	fmt.Println("The following resources will be deleted:")
	if state.FunctionURL != "" {
		fmt.Printf("  - Function URL: %s\n", state.FunctionURL)
	}
	if state.Lambda != nil {
		fmt.Printf("  - Lambda Function: %s\n", state.Lambda.Name)
	}
	if state.Policies.InlineName != "" {
		fmt.Printf("  - Inline Policy: %s\n", state.Policies.InlineName)
	}
	if state.Policies.Managed {
		fmt.Println("  - Managed Policy Attachment: AWSLambdaBasicExecutionRole")
	}
	if state.IAMRole != nil {
		fmt.Printf("  - IAM Role: %s\n", state.IAMRole.Name)
	}
	if state.LogGroup != nil {
		fmt.Printf("  - CloudWatch Log Group: %s\n", state.LogGroup.Name)
	}
	fmt.Println()

	// 4. Create AWS clients once
	clients, err := NewAWSClients(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to create AWS clients: %w", err)
	}

	// 5. Delete in reverse dependency order
	// Order: Function URL → Lambda → Inline Policy → Managed Policy → IAM Role → Log Group

	if state.FunctionURL != "" && state.Lambda != nil {
		if err := ui.WithSpinner("Deleting function URL", func() error {
			return deleteFunctionURL(ctx, clients, state.Lambda.Name)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	if state.Lambda != nil {
		if err := ui.WithSpinner("Deleting Lambda function", func() error {
			return deleteLambdaFunction(ctx, clients, state.Lambda.Name)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	// CRITICAL: Must delete/detach policies before deleting role
	if state.Policies.InlineName != "" && state.IAMRole != nil {
		if err := ui.WithSpinner("Deleting inline policy", func() error {
			return deleteInlinePolicy(ctx, clients, state.IAMRole.Name, state.Policies.InlineName)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	if state.Policies.Managed && state.IAMRole != nil {
		if err := ui.WithSpinner("Detaching managed policy", func() error {
			return detachManagedPolicy(ctx, clients, state.IAMRole.Name, ManagedPolicyARN)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	if state.IAMRole != nil {
		if err := ui.WithSpinner("Deleting IAM role", func() error {
			return deleteIAMRole(ctx, clients, state.IAMRole.Name)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	if state.LogGroup != nil {
		if err := ui.WithSpinner("Deleting CloudWatch log group", func() error {
			return deleteLogGroup(ctx, clients, state.LogGroup.Name)
		}); err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println(ui.Success("✓ Teardown complete!"))
	if isLegacy {
		fmt.Println()
		fmt.Println("  Legacy infrastructure has been removed.")
		fmt.Println("  You can now deploy with: tse deploy")
	}

	return nil
}

// detectLegacyResources checks if resources exist but ALL are missing the ManagedBy=tse tag.
// Returns true if legacy resources detected (old OpenTofu deployment without tags).
// If even one resource has the ManagedBy=tse tag, it's considered a tse deployment.
func detectLegacyResources(state *InfrastructureState) bool {
	var hasResources bool
	var hasTaggedResource bool

	// Check log group
	if state.LogGroup != nil {
		hasResources = true
		if state.LogGroup.Tags["ManagedBy"] == TagManagedBy {
			hasTaggedResource = true
		}
	}

	// Check IAM role
	if state.IAMRole != nil {
		hasResources = true
		if state.IAMRole.Tags["ManagedBy"] == TagManagedBy {
			hasTaggedResource = true
		}
	}

	// Check Lambda
	if state.Lambda != nil {
		hasResources = true
		if state.Lambda.Tags["ManagedBy"] == TagManagedBy {
			hasTaggedResource = true
		}
	}

	// Legacy only if we found resources but NONE have the ManagedBy tag
	return hasResources && !hasTaggedResource
}
