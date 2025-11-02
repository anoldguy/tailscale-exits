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
	fmt.Printf("  Deleting function URL for: %s\n", functionName)

	_, err := clients.Lambda.DeleteFunctionUrlConfig(ctx, &lambda.DeleteFunctionUrlConfigInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete function URL: %w", err)
	}

	fmt.Println("  ✓ Function URL deleted")
	return nil
}

// deleteLambdaFunction deletes the Lambda function.
func deleteLambdaFunction(ctx context.Context, clients *AWSClients, functionName string) error {
	fmt.Printf("  Deleting Lambda function: %s\n", functionName)

	_, err := clients.Lambda.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete Lambda function: %w", err)
	}

	fmt.Println("  ✓ Lambda function deleted")
	return nil
}

// deleteInlinePolicy deletes an inline IAM policy from a role.
// This MUST be done before deleting the role.
func deleteInlinePolicy(ctx context.Context, clients *AWSClients, roleName string, policyName string) error {
	fmt.Printf("  Deleting inline policy: %s\n", policyName)

	_, err := clients.IAM.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{
		RoleName:   aws.String(roleName),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete inline policy: %w", err)
	}

	fmt.Println("  ✓ Inline policy deleted")
	return nil
}

// detachManagedPolicy detaches a managed policy from a role.
// This MUST be done before deleting the role.
func detachManagedPolicy(ctx context.Context, clients *AWSClients, roleName string, policyARN string) error {
	fmt.Printf("  Detaching managed policy: %s\n", policyARN)

	_, err := clients.IAM.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(policyARN),
	})
	if err != nil {
		return fmt.Errorf("failed to detach managed policy: %w", err)
	}

	fmt.Println("  ✓ Managed policy detached")
	return nil
}

// deleteIAMRole deletes an IAM role.
// All policies MUST be detached/deleted first.
func deleteIAMRole(ctx context.Context, clients *AWSClients, roleName string) error {
	fmt.Printf("  Deleting IAM role: %s\n", roleName)

	_, err := clients.IAM.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete IAM role: %w", err)
	}

	fmt.Println("  ✓ IAM role deleted")
	return nil
}

// deleteLogGroup deletes a CloudWatch log group.
func deleteLogGroup(ctx context.Context, clients *AWSClients, logGroupName string) error {
	fmt.Printf("  Deleting CloudWatch log group: %s\n", logGroupName)

	_, err := clients.Logs.DeleteLogGroup(ctx, &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete log group: %w", err)
	}

	fmt.Println("  ✓ Log group deleted")
	return nil
}
