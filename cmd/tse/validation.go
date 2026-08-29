package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anoldguy/tse/shared/regions"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// validateCommand checks that all prerequisites for a command are met
// Returns error immediately if any required configuration is missing or invalid
func validateCommand(ctx context.Context, command string) error {
	switch command {
	case "deploy":
		if err := validateAWS(ctx); err != nil {
			return err
		}
		return validateTailscaleAuth()

	case "status", "teardown":
		return validateAWS(ctx)

	case "health", "shutdown":
		return validateLambda()

	case "setup":
		return validateTailscaleAPI()

	default:
		// Check if this is a region command (e.g., "ohio")
		if regions.IsValidFriendlyName(command) {
			return validateLambda()
		}
		// Unknown command, let normal dispatch handle it
		return nil
	}
}

// validateAWS checks that AWS credentials are configured and actually work
// Makes a quick STS call to verify credentials before expensive operations
func validateAWS(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("AWS credentials not configured: %w\n\nRun 'aws configure' to set up credentials, or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY", err)
	}

	// Quick validation that credentials actually work
	stsClient := sts.NewFromConfig(cfg)
	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("AWS credentials invalid or expired: %w\n\nRun 'aws configure' to update your credentials", err)
	}

	return nil
}

// validateLambda checks that Lambda URL and auth token are configured
func validateLambda() error {
	if os.Getenv("TSE_LAMBDA_URL") == "" {
		return fmt.Errorf("TSE_LAMBDA_URL not set\n\nRun 'tse deploy' to get the Lambda URL, then export it:\n  export TSE_LAMBDA_URL=<url-from-deploy>")
	}

	if os.Getenv("TSE_AUTH_TOKEN") == "" {
		return fmt.Errorf("TSE_AUTH_TOKEN not set\n\nRun 'tse deploy' to get the auth token, then export it:\n  export TSE_AUTH_TOKEN=<token-from-deploy>")
	}

	return nil
}

// validateTailscaleAPI checks that Tailscale API token is configured (for setup command)
func validateTailscaleAPI() error {
	if os.Getenv("TAILSCALE_API_TOKEN") == "" {
		return fmt.Errorf("TAILSCALE_API_TOKEN not set\n\nCreate an API token at https://login.tailscale.com/admin/settings/keys, then export it:\n  export TAILSCALE_API_TOKEN=<your-token>")
	}

	return nil
}

// validateTailscaleAuth checks that Tailscale auth key is configured (for deploy command)
func validateTailscaleAuth() error {
	if os.Getenv("TAILSCALE_OAUTH_SECRET") == "" {
		return fmt.Errorf("TAILSCALE_OAUTH_SECRET not set\n\nCreate an OAuth client with the auth_keys scope and the tag:exitnode tag at\nhttps://login.tailscale.com/admin/settings/oauth, then export the secret:\n  export TAILSCALE_OAUTH_SECRET=tskey-client-...")
	}

	return nil
}
