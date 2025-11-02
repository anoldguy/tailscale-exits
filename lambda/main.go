package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/anoldguy/tse/lambda/aws"
	"github.com/anoldguy/tse/shared/regions"
	"github.com/anoldguy/tse/shared/types"
)

const Version = "1.0.0"

// validateAuth checks the Authorization header against the expected token
func validateAuth(request events.LambdaFunctionURLRequest) error {
	expectedToken := os.Getenv("TSE_AUTH_TOKEN")
	if expectedToken == "" {
		return fmt.Errorf("TSE_AUTH_TOKEN not configured")
	}

	// Get Authorization header (case-insensitive lookup)
	authHeader := ""
	for key, value := range request.Headers {
		if strings.ToLower(key) == "authorization" {
			authHeader = value
			break
		}
	}

	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	// Support "Bearer <token>" or just "<token>"
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimPrefix(token, "bearer ")
	token = strings.TrimSpace(token)

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return fmt.Errorf("invalid token")
	}

	return nil
}

// handler processes Lambda Function URL requests
func handler(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Printf("Request: %s %s", request.RequestContext.HTTP.Method, request.RawPath)

	// Validate authentication
	if err := validateAuth(request); err != nil {
		log.Printf("Authentication failed: %v", err)
		return errorResponse(http.StatusUnauthorized, fmt.Sprintf("Unauthorized: %v", err)), nil
	}

	// Parse the path
	path := strings.TrimPrefix(request.RawPath, "/")
	parts := strings.Split(path, "/")

	method := request.RequestContext.HTTP.Method

	// Route the request
	switch {
	case method == "GET" && path == "":
		return handleHealth(ctx)

	case method == "GET" && len(parts) == 2 && parts[1] == "instances":
		return handleListInstances(ctx, parts[0])

	case method == "POST" && len(parts) == 2 && parts[1] == "start":
		return handleStartInstance(ctx, parts[0])

	case method == "POST" && len(parts) == 2 && parts[1] == "stop":
		return handleStopInstances(ctx, parts[0])

	case method == "POST" && len(parts) == 2 && parts[1] == "cleanup":
		return handleCleanupResources(ctx, parts[0])

	default:
		return errorResponse(http.StatusNotFound, "Not found"), nil
	}
}

// handleHealth returns a simple health check response
func handleHealth(ctx context.Context) (events.LambdaFunctionURLResponse, error) {
	response := types.HealthResponse{
		Status:    "healthy",
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	return jsonResponse(http.StatusOK, response), nil
}

// handleListInstances lists all exit node instances in a region
func handleListInstances(ctx context.Context, friendlyRegion string) (events.LambdaFunctionURLResponse, error) {
	// Validate region
	awsRegion, err := regions.GetAWSRegion(friendlyRegion)
	if err != nil {
		return errorResponse(http.StatusBadRequest, err.Error()), nil
	}

	// Create AWS service for the region
	service, err := aws.New(ctx, awsRegion)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize AWS service: %v", err)), nil
	}

	// List instances
	instances, err := service.ListInstances(ctx)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to list instances: %v", err)), nil
	}

	response := types.InstancesResponse{
		Success:   true,
		Message:   fmt.Sprintf("Found %d instances in %s", len(instances), friendlyRegion),
		Instances: instances,
		Count:     len(instances),
	}

	return jsonResponse(http.StatusOK, response), nil
}

// handleStartInstance creates a new exit node instance
func handleStartInstance(ctx context.Context, friendlyRegion string) (events.LambdaFunctionURLResponse, error) {
	// Validate region
	awsRegion, err := regions.GetAWSRegion(friendlyRegion)
	if err != nil {
		return errorResponse(http.StatusBadRequest, err.Error()), nil
	}

	// Get Tailscale auth key from environment
	authKey := os.Getenv("TAILSCALE_AUTH_KEY")
	if authKey == "" {
		return errorResponse(http.StatusInternalServerError, "TAILSCALE_AUTH_KEY environment variable not set"), nil
	}

	// Create AWS service for the region
	service, err := aws.New(ctx, awsRegion)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize AWS service: %v", err)), nil
	}

	// Check if instance already exists
	existingInstances, err := service.ListInstances(ctx)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to check existing instances: %v", err)), nil
	}

	// Count running/pending instances
	runningCount := 0
	for _, instance := range existingInstances {
		if instance.State == "running" || instance.State == "pending" {
			runningCount++
		}
	}

	if runningCount > 0 {
		return errorResponse(http.StatusConflict, fmt.Sprintf("Exit node already running in %s region", friendlyRegion)), nil
	}

	// Start new instance
	instance, err := service.StartInstance(ctx, friendlyRegion, authKey)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to start instance: %v", err)), nil
	}

	response := types.StartResponse{
		Success:  true,
		Message:  fmt.Sprintf("Exit node started in %s region", friendlyRegion),
		Instance: instance,
	}

	return jsonResponse(http.StatusCreated, response), nil
}

// handleStopInstances terminates all exit node instances in a region
func handleStopInstances(ctx context.Context, friendlyRegion string) (events.LambdaFunctionURLResponse, error) {
	// Validate region
	awsRegion, err := regions.GetAWSRegion(friendlyRegion)
	if err != nil {
		return errorResponse(http.StatusBadRequest, err.Error()), nil
	}

	// Create AWS service for the region
	service, err := aws.New(ctx, awsRegion)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to initialize AWS service: %v", err)), nil
	}

	// Stop instances
	terminatedIDs, err := service.StopInstances(ctx)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Failed to stop instances: %v", err)), nil
	}

	response := types.StopResponse{
		Success:         true,
		Message:         fmt.Sprintf("Terminated %d instances in %s region", len(terminatedIDs), friendlyRegion),
		TerminatedCount: len(terminatedIDs),
		TerminatedIDs:   terminatedIDs,
	}

	return jsonResponse(http.StatusOK, response), nil
}

// jsonResponse creates a JSON response
func jsonResponse(statusCode int, data interface{}) events.LambdaFunctionURLResponse {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return errorResponse(http.StatusInternalServerError, "Internal server error")
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}

// errorResponse creates an error JSON response
func errorResponse(statusCode int, message string) events.LambdaFunctionURLResponse {
	response := types.ErrorResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	}

	body, _ := json.Marshal(response)
	return events.LambdaFunctionURLResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}

// handleCleanupResources force cleans up all TSE resources in a region
func handleCleanupResources(ctx context.Context, friendlyRegion string) (events.LambdaFunctionURLResponse, error) {
	log.Printf("Starting cleanup of all TSE resources in region %s", friendlyRegion)

	awsRegion, err := regions.GetAWSRegion(friendlyRegion)
	if err != nil {
		return errorResponse(http.StatusBadRequest, fmt.Sprintf("Invalid region: %s", friendlyRegion)), nil
	}

	service, err := aws.New(ctx, awsRegion)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Failed to initialize AWS service"), nil
	}

	// Force cleanup all TSE resources
	cleanedResources, err := service.ForceCleanupAllResources(ctx, friendlyRegion)
	if err != nil {
		log.Printf("Cleanup failed: %v", err)
		return errorResponse(http.StatusInternalServerError, fmt.Sprintf("Cleanup failed: %v", err)), nil
	}

	response := types.StopResponse{
		Message:         fmt.Sprintf("Cleaned up all TSE resources in %s", friendlyRegion),
		TerminatedIDs:   cleanedResources,
		TerminatedCount: len(cleanedResources),
	}

	log.Printf("Cleanup completed in region %s: %v", friendlyRegion, cleanedResources)
	return jsonResponse(http.StatusOK, response), nil
}

func main() {
	lambda.Start(handler)
}
