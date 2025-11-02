package infrastructure

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

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
	fmt.Println("  Compiling Lambda function for linux/arm64...")
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
	fmt.Println("  Creating deployment package...")
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

	fmt.Printf("  Deployment package created (%d bytes)\n", buf.Len())
	return buf.Bytes(), nil
}

// createLogGroup creates a CloudWatch log group with the specified retention.
func createLogGroup(ctx context.Context, clients *AWSClients, functionName string, retentionDays int) error {
	logGroupName := fmt.Sprintf("/aws/lambda/%s", functionName)
	fmt.Printf("  Creating CloudWatch log group: %s\n", logGroupName)

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

	fmt.Println("  ✓ Log group created")
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

	fmt.Printf("  Creating IAM role: %s\n", roleName)

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

	fmt.Println("  ✓ IAM role created")
	return *result.Role.Arn, nil
}

// attachManagedPolicy attaches the AWSLambdaBasicExecutionRole managed policy to the role.
func attachManagedPolicy(ctx context.Context, clients *AWSClients, roleName string) error {
	fmt.Println("  Attaching managed policy: AWSLambdaBasicExecutionRole")

	_, err := clients.IAM.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(ManagedPolicyARN),
	})
	if err != nil {
		return fmt.Errorf("failed to attach managed policy: %w", err)
	}

	fmt.Println("  ✓ Managed policy attached")
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

	fmt.Printf("  Creating inline policy: %s\n", InlinePolicyName)

	_, err := clients.IAM.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(InlinePolicyName),
		PolicyDocument: aws.String(policyDocument),
	})
	if err != nil {
		return fmt.Errorf("failed to create inline policy: %w", err)
	}

	fmt.Println("  ✓ Inline policy created")
	return nil
}

// createLambdaFunction creates the Lambda function with the provided configuration.
// Returns the function ARN.
func createLambdaFunction(ctx context.Context, clients *AWSClients, functionName string, roleARN string, zipBytes []byte, tailscaleAuthKey string, tseAuthToken string) (string, error) {
	fmt.Printf("  Creating Lambda function: %s\n", functionName)

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

	fmt.Println("  ✓ Lambda function created")
	return *result.FunctionArn, nil
}

// createFunctionURL creates a Lambda function URL with CORS configuration.
// Returns the function URL.
func createFunctionURL(ctx context.Context, clients *AWSClients, functionName string) (string, error) {
	fmt.Printf("  Creating function URL for: %s\n", functionName)

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
	fmt.Println("  Adding public invocation permission...")
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

	fmt.Println("  ✓ Function URL created")
	return *result.FunctionUrl, nil
}
