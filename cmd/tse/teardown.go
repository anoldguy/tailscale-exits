package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anoldguy/tse/cmd/tse/infrastructure"
	"github.com/anoldguy/tse/cmd/tse/ui"
)

// runTeardown tears down all TSE infrastructure after confirmation.
func runTeardown(args []string) error {
	ctx := context.Background()
	region := "us-east-2" // TODO: make configurable

	fmt.Printf("Region: %s\n", region)
	fmt.Println()

	// Show DANGER box
	items := []string{
		"Lambda function and function URL",
		"IAM role and policies",
		"CloudWatch log groups",
		"ALL exit node instances and VPCs",
	}

	dangerBox := ui.DangerBox(
		"DANGER - PERMANENT DELETION",
		items,
		"Type 'DELETE' to confirm (anything else cancels):",
	)

	fmt.Println(dangerBox)
	fmt.Println()
	fmt.Print("→ ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.TrimSpace(response)
	if response != "DELETE" {
		fmt.Println()
		fmt.Println(ui.Success("✓ Teardown cancelled - nothing was deleted"))
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
