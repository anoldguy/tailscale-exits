package infrastructure

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anoldguy/tse/cmd/tse/ui"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// iamPropagationMessages are the rotating snarky messages shown during IAM propagation wait.
var iamPropagationMessages = []string{
	"Waiting for IAM to propagate (retrying Lambda creation)",
	"AWS is eventually consistent... eventually",
	"Waiting for IAM to propagate across all AWS regions and dimensions",
	"This saves you $10/month vs a commercial VPN",
	"IAM propagation: like waiting for DNS, but for permissions",
	"Fun fact: IAM consistency is why Terraform has trust issues",
	"Distributed systems are great, they said. It'll be fun, they said",
	"Somewhere, an AWS engineer is muttering 'it's fine, it's eventual'",
	"Still cheaper than NordVPN though",
	"This is the part where we pretend 10 seconds is science, not vibes",
	"IAM propagation: the buffering icon of cloud infrastructure",
}

// standardTags returns the standard tag for TSE resources.
func standardTags() map[string]string {
	return map[string]string{
		"ManagedBy": TagManagedBy,
	}
}

// buildLambdaZip compiles the Lambda function for linux/arm64 and creates a deployment zip.
// Returns the zip file bytes.
// Assumes current working directory is the project root.
func buildLambdaZip() ([]byte, error) {
	// Lambda directory relative to current working directory (project root)
	lambdaDir := "lambda"

	// Create a temporary directory for the build
	tmpDir, err := os.MkdirTemp("", "tse-lambda-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	bootstrapPath := filepath.Join(tmpDir, "bootstrap")

	// Compile the Lambda function for linux/arm64
	cmd := exec.Command("go", "build", "-o", bootstrapPath, ".")
	cmd.Dir = lambdaDir
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=arm64",
		"CGO_ENABLED=0",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to compile Lambda: %w\nOutput: %s", err, string(output))
	}

	// Create zip file in memory
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add bootstrap binary to zip
	bootstrapFile, err := os.ReadFile(bootstrapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bootstrap binary: %w", err)
	}

	zipFile, err := zipWriter.Create("bootstrap")
	if err != nil {
		return nil, fmt.Errorf("failed to create zip entry: %w", err)
	}

	_, err = zipFile.Write(bootstrapFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write to zip: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// createLogGroup creates a CloudWatch log group with the specified retention.
func createLogGroup(ctx context.Context, clients *AWSClients, functionName string, retentionDays int) error {
	logGroupName := fmt.Sprintf("/aws/lambda/%s", functionName)

	// Create log group
	_, err := clients.Logs.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroupName),
		Tags:         standardTags(),
	})
	if err != nil {
		return fmt.Errorf("failed to create log group: %w", err)
	}

	// Set retention policy
	_, err = clients.Logs.PutRetentionPolicy(ctx, &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    aws.String(logGroupName),
		RetentionInDays: aws.Int32(int32(retentionDays)),
	})
	if err != nil {
		return fmt.Errorf("failed to set log retention: %w", err)
	}

	return nil
}

// createIAMRole creates the IAM role for Lambda execution.
// Returns the role ARN.
func createIAMRole(ctx context.Context, clients *AWSClients, roleName string) (string, error) {
	// Lambda assume role policy
	assumeRolePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "lambda.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	// Convert tags to IAM tag format
	iamTags := []iamtypes.Tag{}
	for k, v := range standardTags() {
		iamTags = append(iamTags, iamtypes.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	result, err := clients.IAM.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicy),
		Tags:                     iamTags,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %w", err)
	}

	return *result.Role.Arn, nil
}

// attachManagedPolicy attaches the AWSLambdaBasicExecutionRole managed policy to the role.
func attachManagedPolicy(ctx context.Context, clients *AWSClients, roleName string) error {
	_, err := clients.IAM.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(ManagedPolicyARN),
	})
	if err != nil {
		return fmt.Errorf("failed to attach managed policy: %w", err)
	}

	return nil
}

// createInlinePolicy creates the inline policy for EC2/VPC permissions.
func createInlinePolicy(ctx context.Context, clients *AWSClients, roleName string) error {
	// EC2/VPC policy document
	policyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ec2:RunInstances",
					"ec2:TerminateInstances",
					"ec2:DescribeInstances",
					"ec2:DescribeInstanceStatus",
					"ec2:DescribeImages",
					"ec2:CreateSecurityGroup",
					"ec2:DeleteSecurityGroup",
					"ec2:DescribeSecurityGroups",
					"ec2:AuthorizeSecurityGroupIngress",
					"ec2:AuthorizeSecurityGroupEgress",
					"ec2:RevokeSecurityGroupIngress",
					"ec2:RevokeSecurityGroupEgress",
					"ec2:DescribeVpcs",
					"ec2:CreateVpc",
					"ec2:DescribeSubnets",
					"ec2:CreateSubnet",
					"ec2:ModifySubnetAttribute",
					"ec2:DescribeAvailabilityZones",
					"ec2:DescribeRouteTables",
					"ec2:CreateRoute",
					"ec2:DescribeInternetGateways",
					"ec2:CreateInternetGateway",
					"ec2:AttachInternetGateway",
					"ec2:DetachInternetGateway",
					"ec2:DeleteInternetGateway",
					"ec2:DeleteSubnet",
					"ec2:DeleteVpc",
					"ec2:DeleteRoute",
					"ec2:CreateTags",
					"ec2:DescribeTags"
				],
				"Resource": "*"
			},
			{
				"Effect": "Allow",
				"Action": [
					"ssm:GetParameter",
					"ssm:GetParameters"
				],
				"Resource": [
					"arn:aws:ssm:*:*:parameter/aws/service/ami-amazon-linux-latest/*",
					"arn:aws:ssm:*:*:parameter/aws/service/canonical/ubuntu/server/*"
				]
			}
		]
	}`

	_, err := clients.IAM.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(InlinePolicyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		return fmt.Errorf("failed to create inline policy: %w", err)
	}

	return nil
}

// createLambdaFunction creates the Lambda function with the provided configuration.
// Returns the function ARN.
func createLambdaFunction(ctx context.Context, clients *AWSClients, functionName string, roleARN string, zipBytes []byte, tailscaleAuthKey string, tseAuthToken string) (string, error) {
	// Convert tags to Lambda tag format
	lambdaTags := standardTags()

	result, err := clients.Lambda.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: aws.String(functionName),
		Runtime:      lambdatypes.RuntimeProvidedal2023,
		Role:         aws.String(roleARN),
		Handler:      aws.String("bootstrap"),
		Code: &lambdatypes.FunctionCode{
			ZipFile: zipBytes,
		},
		Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureArm64},
		MemorySize:    aws.Int32(256),
		Timeout:       aws.Int32(60),
		Environment: &lambdatypes.Environment{
			Variables: map[string]string{
				"TAILSCALE_AUTH_KEY": tailscaleAuthKey,
				"TSE_AUTH_TOKEN":     tseAuthToken,
			},
		},
		Tags: lambdaTags,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Lambda function: %w", err)
	}

	return *result.FunctionArn, nil
}

// isIAMPropagationError checks if an error is due to IAM eventual consistency.
// Returns true if the error indicates the role cannot be assumed yet.
func isIAMPropagationError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// Check for InvalidParameterValueException with "cannot be assumed" message
	return strings.Contains(errMsg, "InvalidParameterValueException") &&
		strings.Contains(errMsg, "cannot be assumed")
}

// createLambdaFunctionWithRetry creates the Lambda function, retrying on IAM propagation errors.
// Shows rotating snarky messages if we hit propagation delays.
// Handles its own UI - starts with regular spinner, switches to rotating messages if needed.
// Returns the function ARN.
func createLambdaFunctionWithRetry(ctx context.Context, clients *AWSClients, functionName string, roleARN string, zipBytes []byte, tailscaleAuthKey string, tseAuthToken string) (string, error) {
	// Try immediately with a regular spinner
	var arn string
	err := ui.WithSpinner("Creating Lambda function", func() error {
		var err error
		arn, err = createLambdaFunction(ctx, clients, functionName, roleARN, zipBytes, tailscaleAuthKey, tseAuthToken)
		return err
	})

	if err == nil {
		// Success on first try!
		return arn, nil
	}

	// Check if it's an IAM propagation error
	if !isIAMPropagationError(err) {
		// Real error, fail immediately (spinner already showed X)
		return "", err
	}

	// IAM propagation error - show rotating messages and retry
	var finalARN string
	var finalErr error

	retryErr := ui.WithRotatingMessages(iamPropagationMessages, func() error {
		arn, err := createLambdaFunction(ctx, clients, functionName, roleARN, zipBytes, tailscaleAuthKey, tseAuthToken)
		if err == nil {
			finalARN = arn
			return nil
		}

		// Still failing - check if it's still propagation or a different error
		if isIAMPropagationError(err) {
			// Keep retrying
			return fmt.Errorf("still waiting")
		}

		// Different error, stop retrying
		finalErr = err
		return nil
	})

	if retryErr != nil {
		return "", retryErr // Timeout
	}

	if finalErr != nil {
		return "", finalErr // Real error encountered during retry
	}

	return finalARN, nil
}

// createFunctionURL creates a Lambda function URL with CORS configuration.
// Returns the function URL.
func createFunctionURL(ctx context.Context, clients *AWSClients, functionName string) (string, error) {
	result, err := clients.Lambda.CreateFunctionUrlConfig(ctx, &lambda.CreateFunctionUrlConfigInput{
		FunctionName: aws.String(functionName),
		AuthType:     lambdatypes.FunctionUrlAuthTypeNone,
		Cors: &lambdatypes.Cors{
			AllowCredentials: aws.Bool(false),
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "DELETE"},
			AllowHeaders:     []string{"date", "keep-alive", "content-type", "authorization"},
			ExposeHeaders:    []string{"date", "keep-alive"},
			MaxAge:           aws.Int32(86400),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create function URL: %w", err)
	}

	// Add resource-based policy to allow public invocation via Function URL
	// This is required when AuthType is NONE
	_, err = clients.Lambda.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName:        aws.String(functionName),
		StatementId:         aws.String("FunctionURLAllowPublicAccess"),
		Action:              aws.String("lambda:InvokeFunctionUrl"),
		Principal:           aws.String("*"),
		FunctionUrlAuthType: lambdatypes.FunctionUrlAuthTypeNone,
	})
	if err != nil {
		return "", fmt.Errorf("failed to add function URL permission: %w", err)
	}

	return *result.FunctionUrl, nil
}
