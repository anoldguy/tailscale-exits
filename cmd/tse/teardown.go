package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
)

// runTeardown tears down all TSE infrastructure after confirmation.
func runTeardown(args []string) error {
	ctx := context.Background()
	region := "us-east-2" // TODO: make configurable

	fmt.Printf("Region: %s\n", region)
	fmt.Println()

	// Show warning
	fmt.Println("⚠️  WARNING: This will permanently delete all TSE infrastructure!")
	fmt.Println()
	fmt.Println("This includes:")
	fmt.Println("  - Lambda function and function URL")
	fmt.Println("  - IAM role and policies")
	fmt.Println("  - CloudWatch log groups")
	fmt.Println()

	// Require explicit confirmation
	fmt.Print("Type 'yes' to confirm deletion: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(response)
	if response != "yes" {
		fmt.Println()
		fmt.Println("Teardown cancelled")
		return nil
	}

	fmt.Println()

	// Execute teardown
	err = infrastructure.Teardown(ctx, region)
	if err != nil {
		return err
	}

	return nil
}
