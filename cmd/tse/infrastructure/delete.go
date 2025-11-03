package infrastructure

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// deleteFunctionURL deletes the Lambda function URL.
func deleteFunctionURL(ctx context.Context, clients *AWSClients, functionName string) error {
	_, err := clients.Lambda.DeleteFunctionUrlConfig(ctx, &lambda.DeleteFunctionUrlConfigInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete function URL: %w", err)
	}

	return nil
}

// deleteLambdaFunction deletes the Lambda function.
func deleteLambdaFunction(ctx context.Context, clients *AWSClients, functionName string) error {
	_, err := clients.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete Lambda function: %w", err)
	}

	return nil
}

// deleteInlinePolicy deletes an inline IAM policy from a role.
// This MUST be done before deleting the role.
func deleteInlinePolicy(ctx context.Context, clients *AWSClients, roleName string, policyName string) error {
	_, err := clients.IAM.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
		RoleName:   aws.String(roleName),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete inline policy: %w", err)
	}

	return nil
}

// detachManagedPolicy detaches a managed policy from a role.
// This MUST be done before deleting the role.
func detachManagedPolicy(ctx context.Context, clients *AWSClients, roleName string, policyARN string) error {
	_, err := clients.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyARN),
	})
	if err != nil {
		return fmt.Errorf("failed to detach managed policy: %w", err)
	}

	return nil
}

// deleteIAMRole deletes an IAM role.
// All policies MUST be detached/deleted first.
func deleteIAMRole(ctx context.Context, clients *AWSClients, roleName string) error {
	_, err := clients.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %w", err)
	}

	return nil
}

// deleteLogGroup deletes a CloudWatch log group.
func deleteLogGroup(ctx context.Context, clients *AWSClients, logGroupName string) error {
	_, err := clients.Logs.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete log group: %w", err)
	}

	return nil
}
