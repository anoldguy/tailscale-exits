package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anoldguy/tse/shared/regions"
	"github.com/anoldguy/tse/shared/types"
)

const Usage = `Tailscale Ephemeral Exit Node Service CLI

Usage:
  tse setup [flags]             - Configure Tailscale for exit nodes (one-time)
  tse health                    - Check Lambda health
  tse shutdown                  - Stop exit nodes in ALL regions
  tse <region> instances        - List instances in region
  tse <region> start            - Start exit node in region
  tse <region> stop             - Stop exit nodes in region
  tse <region> cleanup          - Clean up orphaned TSE resources in region

Available regions: %s

Environment Variables:
  TSE_LAMBDA_URL        - Lambda Function URL (required for exit node operations)
  TAILSCALE_API_TOKEN   - Tailscale API token (required for setup command)

Examples:
  tse setup                      # Configure Tailscale (first time)
  tse setup --status             # Check configuration status
  tse health
  tse shutdown                   # Stop exit nodes everywhere
  tse ohio instances
  tse ohio start
  tse ohio stop
`

func main() {
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Handle setup command (doesn't require TSE_LAMBDA_URL)
	if command == "setup" {
		err := runSetup(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// All other commands require TSE_LAMBDA_URL
	lambdaURL := os.Getenv("TSE_LAMBDA_URL")
	if lambdaURL == "" {
		fmt.Fprintf(os.Stderr, "Error: TSE_LAMBDA_URL environment variable not set\n")
		fmt.Fprintf(os.Stderr, "\nHint: First run 'tse setup' to configure Tailscale, then deploy the Lambda.\n")
		os.Exit(1)
	}

	// Remove trailing slash if present
	lambdaURL = strings.TrimSuffix(lambdaURL, "/")

	// Handle health check (special case)
	if command == "health" {
		if len(os.Args) != 2 {
			showUsage()
			os.Exit(1)
		}
		err := handleHealth(lambdaURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle shutdown (stop all regions)
	if command == "shutdown" {
		if len(os.Args) != 2 {
			showUsage()
			os.Exit(1)
		}
		err := handleShutdown(lambdaURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// All other commands require region + action
	if len(os.Args) != 3 {
		showUsage()
		os.Exit(1)
	}

	region := command
	action := os.Args[2]

	// Validate region
	if !regions.IsValidFriendlyName(region) {
		fmt.Fprintf(os.Stderr, "Error: Invalid region '%s'\n", region)
		fmt.Fprintf(os.Stderr, "Available regions: %s\n", regions.GetAvailableRegions())
		os.Exit(1)
	}

	// Handle actions
	switch action {
	case "instances":
		err := handleInstances(lambdaURL, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "start":
		err := handleStart(lambdaURL, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "stop":
		err := handleStop(lambdaURL, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "cleanup":
		err := handleCleanup(lambdaURL, region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid action '%s'\n", action)
		fmt.Fprintf(os.Stderr, "Valid actions: instances, start, stop, cleanup\n")
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Printf(Usage, regions.GetAvailableRegions())
}

func getAuthToken() string {
	return os.Getenv("TSE_AUTH_TOKEN")
}

func makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Add Authorization header if token is set
	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	return client.Do(req)
}

func handleHealth(lambdaURL string) error {
	resp, err := makeAuthenticatedRequest("GET", lambdaURL, nil)
	if err != nil {
		return fmt.Errorf("failed to contact Lambda: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", string(body))
	}

	var health types.HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("failed to parse health response: %w", err)
	}

	fmt.Printf("Status: %s\n", health.Status)
	fmt.Printf("Version: %s\n", health.Version)
	fmt.Printf("Timestamp: %s\n", health.Timestamp)

	return nil
}

func handleInstances(lambdaURL, region string) error {
	url := fmt.Sprintf("%s/%s/instances", lambdaURL, region)
	resp, err := makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to contact Lambda: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp types.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return fmt.Errorf(errorResp.Error)
		}
		return fmt.Errorf("request failed: %s", string(body))
	}

	var instancesResp types.InstancesResponse
	if err := json.Unmarshal(body, &instancesResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("Instances in %s region: %d\n", region, instancesResp.Count)
	if instancesResp.Count == 0 {
		fmt.Println("No instances found.")
		return nil
	}

	fmt.Println()
	for _, instance := range instancesResp.Instances {
		fmt.Printf("Instance ID: %s\n", instance.InstanceID)
		fmt.Printf("  State: %s\n", instance.State)
		fmt.Printf("  Type: %s\n", instance.InstanceType)
		fmt.Printf("  Launch Time: %s\n", instance.LaunchTime.Format(time.RFC3339))
		if instance.PublicIP != "" {
			fmt.Printf("  Public IP: %s\n", instance.PublicIP)
		}
		if instance.TailscaleHostname != "" {
			fmt.Printf("  Tailscale Hostname: %s\n", instance.TailscaleHostname)
		}
		fmt.Println()
	}

	return nil
}

func handleStart(lambdaURL, region string) error {
	url := fmt.Sprintf("%s/%s/start", lambdaURL, region)
	resp, err := makeAuthenticatedRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to contact Lambda: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		var errorResp types.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			fmt.Printf("Info: %s\n", errorResp.Error)
			return nil
		}
	}

	if resp.StatusCode != http.StatusCreated {
		var errorResp types.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return fmt.Errorf(errorResp.Error)
		}
		return fmt.Errorf("request failed: %s", string(body))
	}

	var startResp types.StartResponse
	if err := json.Unmarshal(body, &startResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("✓ %s\n", startResp.Message)
	if startResp.Instance != nil {
		fmt.Printf("Instance ID: %s\n", startResp.Instance.InstanceID)
		fmt.Printf("Instance Type: %s\n", startResp.Instance.InstanceType)
		fmt.Printf("Tailscale Hostname: %s\n", startResp.Instance.TailscaleHostname)
		fmt.Printf("State: %s\n", startResp.Instance.State)
		fmt.Println("\nNote: It may take 1-2 minutes for the exit node to become available in Tailscale.")
	}

	return nil
}

func handleStop(lambdaURL, region string) error {
	url := fmt.Sprintf("%s/%s/stop", lambdaURL, region)
	resp, err := makeAuthenticatedRequest("POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to contact Lambda: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp types.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return fmt.Errorf(errorResp.Error)
		}
		return fmt.Errorf("request failed: %s", string(body))
	}

	var stopResp types.StopResponse
	if err := json.Unmarshal(body, &stopResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("✓ %s\n", stopResp.Message)
	if stopResp.TerminatedCount > 0 {
		fmt.Printf("Terminated instances: %v\n", stopResp.TerminatedIDs)
	}

	return nil
}

func handleCleanup(lambdaURL, region string) error {
	url := fmt.Sprintf("%s/%s/cleanup", lambdaURL, region)
	resp, err := makeAuthenticatedRequest("POST", url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("failed to contact Lambda: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp types.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return fmt.Errorf(errorResp.Error)
		}
		return fmt.Errorf("cleanup failed: %s", string(body))
	}

	var cleanupResp types.StopResponse // Reuse stop response structure
	if err := json.Unmarshal(body, &cleanupResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Printf("✓ %s\n", cleanupResp.Message)
	if cleanupResp.TerminatedCount > 0 {
		fmt.Printf("Cleaned up resources: %v\n", cleanupResp.TerminatedIDs)
	} else {
		fmt.Println("No orphaned TSE resources found.")
	}

	return nil
}

func handleShutdown(lambdaURL string) error {
	fmt.Println("Stopping exit nodes in all regions...")
	fmt.Println()

	allRegions := regions.GetAllFriendlyNames()
	totalTerminated := 0
	regionsWithInstances := []string{}

	for _, region := range allRegions {
		url := fmt.Sprintf("%s/%s/stop", lambdaURL, region)
		resp, err := makeAuthenticatedRequest("POST", url, bytes.NewReader([]byte("{}")))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to contact Lambda for %s: %v\n", region, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read response for %s: %v\n", region, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// Silently skip regions with no instances or errors
			continue
		}

		var stopResp types.StopResponse
		if err := json.Unmarshal(body, &stopResp); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse response for %s: %v\n", region, err)
			continue
		}

		if stopResp.TerminatedCount > 0 {
			fmt.Printf("✓ %s: terminated %d instance(s)\n", region, stopResp.TerminatedCount)
			totalTerminated += stopResp.TerminatedCount
			regionsWithInstances = append(regionsWithInstances, region)
		}
	}

	fmt.Println()
	if totalTerminated == 0 {
		fmt.Println("No running exit nodes found in any region.")
	} else {
		fmt.Printf("✓ Shutdown complete: terminated %d instance(s) across %d region(s)\n",
			totalTerminated, len(regionsWithInstances))
	}

	return nil
}