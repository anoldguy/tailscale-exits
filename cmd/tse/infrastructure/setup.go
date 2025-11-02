package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// SetupResult contains the deployment result including secrets.
type SetupResult struct {
	State        *InfrastructureState
	AuthToken    string // TSE_AUTH_TOKEN used for this deployment
	WasGenerated bool   // True if auth token was newly generated
}

// Setup orchestrates the idempotent deployment of TSE infrastructure.
// Creates only missing resources and returns the final state.
func Setup(ctx context.Context, region string) (*SetupResult, error) {
	fmt.Println("Deploying TSE infrastructure...")
	fmt.Println()

	// 1. Discover existing state
	state, err := AutodiscoverInfrastructure(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to discover infrastructure: %w", err)
	}

	if state.IsComplete() {
		fmt.Println("✓ Infrastructure already deployed")
		fmt.Println()
		// Still need to return auth token even if already deployed
		tseAuthToken := os.Getenv("TSE_AUTH_TOKEN")
		return &SetupResult{
			State:        state,
			AuthToken:    tseAuthToken,
			WasGenerated: false,
		}, nil
	}

	missing := state.Missing()
	fmt.Printf("Found %d missing resources, creating...\n", len(missing))
	fmt.Println()

	// 2. Get secrets from environment
	tailscaleAuthKey := os.Getenv("TAILSCALE_AUTH_KEY")
	if tailscaleAuthKey == "" {
		return nil, fmt.Errorf("TAILSCALE_AUTH_KEY environment variable not set\n\nHint: Export your Tailscale auth key:\n  export TAILSCALE_AUTH_KEY=tskey-auth-...")
	}

	// Generate or reuse auth token
	tseAuthToken := os.Getenv("TSE_AUTH_TOKEN")
	wasGenerated := false
	if tseAuthToken == "" {
		tseAuthToken = generateAuthToken()
		wasGenerated = true
		fmt.Println("Generated new TSE_AUTH_TOKEN (save this!):")
		fmt.Printf("  export TSE_AUTH_TOKEN=%s\n", tseAuthToken)
		fmt.Println()
	}

	// 3. Create AWS clients once
	clients, err := NewAWSClients(ctx, region)
	if err != nil {
		return nil, err
	}

	// 4. Create CloudWatch Log Group (if missing)
	if state.LogGroup == nil {
		if err := createLogGroup(ctx, clients, FunctionName, 14); err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// 5. Create IAM Role (if missing)
	var roleARN string
	if state.IAMRole == nil {
		roleARN, err = createIAMRole(ctx, clients, RoleName)
		if err != nil {
			return nil, err
		}
		fmt.Println()
	} else {
		roleARN = state.IAMRole.ARN
	}

	// 6. Attach policies (if missing)
	if !state.Policies.Managed {
		if err := attachManagedPolicy(ctx, clients, RoleName); err != nil {
			return nil, err
		}
		fmt.Println()
	}

	if state.Policies.InlineName == "" {
		if err := createInlinePolicy(ctx, clients, RoleName); err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// 7. Wait for IAM eventual consistency if we created role or policies
	if state.IAMRole == nil || !state.Policies.Managed || state.Policies.InlineName == "" {
		fmt.Println("Waiting 10 seconds for IAM propagation...")
		time.Sleep(10 * time.Second)
		fmt.Println()
	}

	// 8. Create Lambda Function (if missing)
	if state.Lambda == nil {
		// Build Lambda
		zipBytes, err := buildLambdaZip()
		if err != nil {
			return nil, err
		}
		fmt.Println()

		// Create function
		_, err = createLambdaFunction(ctx, clients, FunctionName, roleARN, zipBytes, tailscaleAuthKey, tseAuthToken)
		if err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// 9. Create Function URL (if missing)
	if state.FunctionURL == "" {
		_, err := createFunctionURL(ctx, clients, FunctionName)
		if err != nil {
			return nil, err
		}
		fmt.Println()
	}

	// 10. Re-discover to get final state
	finalState, err := AutodiscoverInfrastructure(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("failed to verify deployment: %w", err)
	}

	fmt.Println("✓ Infrastructure deployment complete!")
	fmt.Println()

	return &SetupResult{
		State:        finalState,
		AuthToken:    tseAuthToken,
		WasGenerated: wasGenerated,
	}, nil
}

// generateAuthToken creates a cryptographically secure random token.
func generateAuthToken() string {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		// Fallback to less secure but still reasonable token
		return fmt.Sprintf("tse-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
