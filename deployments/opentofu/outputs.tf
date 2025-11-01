# outputs.tf - Output values from Tailscale exit node infrastructure

output "lambda_function_url" {
  description = "The URL endpoint for the Lambda function (used by CLI)"
  value       = aws_lambda_function_url.tailscale_exits_url.function_url
  sensitive   = false
}

output "auth_token" {
  description = "Authentication token for Lambda requests (set as TSE_AUTH_TOKEN env var)"
  value       = random_id.lambda_auth_token.hex
  sensitive   = true
}

output "lambda_function_arn" {
  description = "ARN of the Lambda function"
  value       = aws_lambda_function.tailscale_exits.arn
  sensitive   = false
}

output "lambda_function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.tailscale_exits.function_name
  sensitive   = false
}

output "lambda_role_arn" {
  description = "ARN of the IAM role used by the Lambda function"
  value       = aws_iam_role.lambda_role.arn
  sensitive   = false
}

output "cloudwatch_log_group_name" {
  description = "Name of the CloudWatch log group for Lambda function logs"
  value       = aws_cloudwatch_log_group.lambda_logs.name
  sensitive   = false
}

output "deployment_info" {
  description = "Summary information about the deployment"
  value = {
    function_url    = aws_lambda_function_url.tailscale_exits_url.function_url
    function_name   = aws_lambda_function.tailscale_exits.function_name
    runtime         = aws_lambda_function.tailscale_exits.runtime
    architecture    = aws_lambda_function.tailscale_exits.architectures[0]
    memory_size     = aws_lambda_function.tailscale_exits.memory_size
    timeout         = aws_lambda_function.tailscale_exits.timeout
    log_group       = aws_cloudwatch_log_group.lambda_logs.name
  }
  sensitive = false
}