package infrastructure

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const (
	// Resource names
	FunctionName     = "tailscale-exits"
	RoleName         = "tailscale-exits-lambda-role"
	InlinePolicyName = "tailscale-exits-lambda-ec2-policy"
	LogGroupName     = "/aws/lambda/tailscale-exits"

	// Standard tag for all TSE resources
	TagManagedBy = "tse"

	// AWS managed policy ARN
	ManagedPolicyARN = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
)

// AWSClients holds AWS service clients for infrastructure operations.
// Creating clients once and reusing them is more efficient than repeatedly loading config.
type AWSClients struct {
	IAM    *iam.Client
	Lambda *lambda.Client
	Logs   *cloudwatchlogs.Client
}

// NewAWSClients creates AWS service clients for the given region.
// IAM client uses the region but IAM is a global service.
func NewAWSClients(ctx context.Context, region string) (*AWSClients, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSClients{
		IAM:    iam.NewFromConfig(cfg),
		Lambda: lambda.NewFromConfig(cfg),
		Logs:   cloudwatchlogs.NewFromConfig(cfg),
	}, nil
}

// AutodiscoverInfrastructure discovers all TSE infrastructure in the given region using tag-based discovery.
// Returns a complete InfrastructureState.
func AutodiscoverInfrastructure(ctx context.Context, region string) (*InfrastructureState, error) {
	clients, err := NewAWSClients(ctx, region)
	if err != nil {
		return nil, err
	}

	state := &InfrastructureState{}

	// Discover IAM resources (global, but we still check)
	if err := discoverIAMResources(ctx, clients, state); err != nil {
		return nil, fmt.Errorf("IAM discovery failed: %w", err)
	}

	// Discover Lambda resources
	if err := discoverLambdaResources(ctx, clients, state); err != nil {
		return nil, fmt.Errorf("Lambda discovery failed: %w", err)
	}

	// Discover CloudWatch Logs resources
	if err := discoverLogsResources(ctx, clients, state); err != nil {
		return nil, fmt.Errorf("CloudWatch Logs discovery failed: %w", err)
	}

	return state, nil
}

// discoverIAMResources discovers IAM role, policies, and attachments.
// Populates the IAMRole and Policies fields of the state.
func discoverIAMResources(ctx context.Context, clients *AWSClients, state *InfrastructureState) error {
	// Try to get the IAM role
	roleOutput, err := clients.IAM.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(RoleName),
	})
	if err != nil {
		// Role doesn't exist - this is fine for discovery
		return nil
	}

	// Get role tags
	tagsOutput, err := clients.IAM.ListRoleTags(ctx, &iam.ListRoleTagsInput{
		RoleName: aws.String(RoleName),
	})
	if err != nil {
		return fmt.Errorf("failed to list role tags: %w", err)
	}

	tags := make(map[string]string)
	for _, tag := range tagsOutput.Tags {
		tags[*tag.Key] = *tag.Value
	}

	// Store role info
	// Note: Tag validation is lenient for backward compatibility with resources
	// created before ManagedBy tagging was standardized
	state.IAMRole = &Resource{
		Name: *roleOutput.Role.RoleName,
		ARN:  *roleOutput.Role.Arn,
		Tags: tags,
	}

	// Check for managed policy attachment
	attachedPolicies, err := clients.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(RoleName),
	})
	if err != nil {
		return fmt.Errorf("failed to list attached policies: %w", err)
	}

	for _, policy := range attachedPolicies.AttachedPolicies {
		if *policy.PolicyArn == ManagedPolicyARN {
			state.Policies.Managed = true
			break
		}
	}

	// Check for inline policy
	inlinePolicy, err := clients.IAM.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
		RoleName:   aws.String(RoleName),
		PolicyName: aws.String(InlinePolicyName),
	})
	if err == nil {
		state.Policies.InlineName = *inlinePolicy.PolicyName
		state.Policies.InlineDocument = *inlinePolicy.PolicyDocument
	}
	// Ignore error if policy doesn't exist

	return nil
}

// discoverLambdaResources discovers Lambda function and function URL.
// Populates the Lambda and FunctionURL fields of the state.
func discoverLambdaResources(ctx context.Context, clients *AWSClients, state *InfrastructureState) error {
	// Try to get the Lambda function
	functionOutput, err := clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(FunctionName),
	})
	if err != nil {
		// Function doesn't exist - fine for discovery
		return nil
	}

	// Get function tags
	tagsOutput, err := clients.Lambda.ListTags(ctx, &lambda.ListTagsInput{
		Resource: functionOutput.Configuration.FunctionArn,
	})
	if err != nil {
		return fmt.Errorf("failed to list function tags: %w", err)
	}

	// Store function info
	// Note: Tag validation is lenient for backward compatibility with resources
	// created before ManagedBy tagging was standardized
	state.Lambda = &Resource{
		Name: *functionOutput.Configuration.FunctionName,
		ARN:  *functionOutput.Configuration.FunctionArn,
		Tags: tagsOutput.Tags,
	}

	// Try to get function URL config
	urlConfig, err := clients.Lambda.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{
		FunctionName: aws.String(FunctionName),
	})
	if err == nil {
		state.FunctionURL = *urlConfig.FunctionUrl
	}
	// Ignore error if URL doesn't exist

	return nil
}

// discoverLogsResources discovers CloudWatch log groups.
// Populates the LogGroup field of the state.
func discoverLogsResources(ctx context.Context, clients *AWSClients, state *InfrastructureState) error {
	// Try to find the log group
	logGroups, err := clients.Logs.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(LogGroupName),
		Limit:              aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("failed to describe log groups: %w", err)
	}

	// Check if we found the exact log group
	if len(logGroups.LogGroups) > 0 && *logGroups.LogGroups[0].LogGroupName == LogGroupName {
		logGroup := logGroups.LogGroups[0]

		tags := make(map[string]string)

		// Try to get tags if ARN is available
		// Note: Tag validation is lenient for backward compatibility with resources
		// created before ManagedBy tagging was standardized
		if logGroup.Arn != nil && *logGroup.Arn != "" {
			tagsOutput, err := clients.Logs.ListTagsForResource(ctx, &cloudwatchlogs.ListTagsForResourceInput{
				ResourceArn: logGroup.Arn,
			})
			if err == nil {
				tags = tagsOutput.Tags
			}
			// Ignore tag fetch errors for now - we'll just have empty tags
		}

		arn := ""
		if logGroup.Arn != nil {
			arn = *logGroup.Arn
		}

		// Store log group info
		state.LogGroup = &Resource{
			Name: *logGroup.LogGroupName,
			ARN:  arn,
			Tags: tags,
		}
	}

	return nil
}
