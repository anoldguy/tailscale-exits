package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/anoldguy/tse/cmd/tse/ui"
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
	fmt.Println(ui.Title("Deploying TSE infrastructure"))
	fmt.Println()

	// 1. Discover existing state
	var state *InfrastructureState
	err := ui.WithSpinner("Discovering existing infrastructure", func() error {
		var err error
		state, err = AutodiscoverInfrastructure(ctx, region)
		return err
	})
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
		if err := ui.WithSpinner("Creating CloudWatch log group", func() error {
			return createLogGroup(ctx, clients, FunctionName, 14)
		}); err != nil {
			return nil, err
		}
	}

	// 5. Create IAM Role (if missing)
	var roleARN string
	if state.IAMRole == nil {
		if err := ui.WithSpinner("Creating IAM execution role", func() error {
			var err error
			roleARN, err = createIAMRole(ctx, clients, RoleName)
			return err
		}); err != nil {
			return nil, err
		}
	} else {
		roleARN = state.IAMRole.ARN
	}

	// 6. Attach policies (if missing)
	if !state.Policies.Managed {
		if err := ui.WithSpinner("Attaching managed execution policy", func() error {
			return attachManagedPolicy(ctx, clients, RoleName)
		}); err != nil {
			return nil, err
		}
	}

	if state.Policies.InlineName == "" {
		if err := ui.WithSpinner("Creating inline EC2/VPC policy", func() error {
			return createInlinePolicy(ctx, clients, RoleName)
		}); err != nil {
			return nil, err
		}
	}

	// 7. Create Lambda Function (if missing)
	// Note: This will automatically retry with snarky messages if we hit IAM propagation delays
	if state.Lambda == nil {
		// Build Lambda
		var zipBytes []byte
		if err := ui.WithSpinner("Building Lambda function (linux/arm64)", func() error {
			var err error
			zipBytes, err = buildLambdaZip()
			return err
		}); err != nil {
			return nil, err
		}

		// Create function (handles its own UI - spinner for normal case, rotating messages for IAM delays)
		if _, err := createLambdaFunctionWithRetry(ctx, clients, FunctionName, roleARN, zipBytes, tailscaleAuthKey, tseAuthToken); err != nil {
			return nil, err
		}
	}

	// 8. Create Function URL (if missing)
	if state.FunctionURL == "" {
		if err := ui.WithSpinner("Creating public function URL", func() error {
			_, err := createFunctionURL(ctx, clients, FunctionName)
			return err
		}); err != nil {
			return nil, err
		}
	}

	// 9. Re-discover to get final state
	var finalState *InfrastructureState
	if err := ui.WithSpinner("Verifying deployment", func() error {
		var err error
		finalState, err = AutodiscoverInfrastructure(ctx, region)
		return err
	}); err != nil {
		return nil, fmt.Errorf("failed to verify deployment: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.Success("✓ Infrastructure deployment complete!"))
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
