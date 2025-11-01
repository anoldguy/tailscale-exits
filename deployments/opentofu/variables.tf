# variables.tf - Input variables for Tailscale exit node infrastructure

variable "aws_region" {
  description = "AWS region to deploy the Lambda function in"
  type        = string
  default     = "us-east-2"
}

variable "lambda_function_name" {
  description = "Name of the Lambda function"
  type        = string
  default     = "tailscale-exits"

  validation {
    condition     = can(regex("^[a-zA-Z0-9-_]+$", var.lambda_function_name))
    error_message = "Lambda function name must contain only alphanumeric characters, hyphens, and underscores."
  }
}

variable "lambda_memory_size" {
  description = "Memory size for the Lambda function in MB"
  type        = number
  default     = 256

  validation {
    condition     = var.lambda_memory_size >= 128 && var.lambda_memory_size <= 10240
    error_message = "Lambda memory size must be between 128 MB and 10,240 MB."
  }
}

variable "lambda_timeout" {
  description = "Timeout for the Lambda function in seconds"
  type        = number
  default     = 60

  validation {
    condition     = var.lambda_timeout >= 1 && var.lambda_timeout <= 900
    error_message = "Lambda timeout must be between 1 and 900 seconds."
  }
}

variable "log_retention_days" {
  description = "Number of days to retain CloudWatch logs"
  type        = number
  default     = 14

  validation {
    condition = contains([
      1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653
    ], var.log_retention_days)
    error_message = "Log retention days must be a valid CloudWatch retention period."
  }
}

variable "tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}