# main.tf - Core infrastructure for Tailscale ephemeral exit node Lambda service

# Data source to get secrets from 1Password
data "external" "one_password" {
  program = ["bash", "${path.module}/one_password.sh"]
}

# Generate random auth token for Lambda authentication
# This token persists across deployments (stored in Terraform state)
# To rotate: tofu taint random_id.lambda_auth_token && tofu apply
resource "random_id" "lambda_auth_token" {
  byte_length = 32 # 256 bits, outputs as 64-char hex
}

# Local values to mark sensitive data
locals {
  tailscale_auth_key = sensitive(data.external.one_password.result.tailscale_auth_key)
  lambda_auth_token  = sensitive(random_id.lambda_auth_token.hex)
}

# Data source for Lambda deployment package
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.root}/../../lambda/bootstrap"
  output_path = "${path.root}/lambda-deployment.zip"
}

# IAM role for Lambda function
resource "aws_iam_role" "lambda_role" {
  name = "${var.lambda_function_name}-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Project = "tailscale-exits"
    Purpose = "ephemeral-vpn-nodes"
  }
}

# IAM policy for Lambda execution basics
resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  role       = aws_iam_role.lambda_role.name
}

# Custom IAM policy for EC2 and VPC operations
resource "aws_iam_role_policy" "lambda_ec2_policy" {
  name = "${var.lambda_function_name}-lambda-ec2-policy"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          # EC2 instance management
          "ec2:RunInstances",
          "ec2:TerminateInstances",
          "ec2:DescribeInstances",
          "ec2:DescribeInstanceStatus",
          "ec2:DescribeImages",

          # Security group management
          "ec2:CreateSecurityGroup",
          "ec2:DeleteSecurityGroup",
          "ec2:DescribeSecurityGroups",
          "ec2:AuthorizeSecurityGroupIngress",
          "ec2:AuthorizeSecurityGroupEgress",
          "ec2:RevokeSecurityGroupIngress",
          "ec2:RevokeSecurityGroupEgress",

          # VPC and networking
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

          # Resource tagging
          "ec2:CreateTags",
          "ec2:DescribeTags"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          # SSM for getting latest AMI IDs
          "ssm:GetParameter",
          "ssm:GetParameters"
        ]
        Resource = [
          "arn:aws:ssm:*:*:parameter/aws/service/ami-amazon-linux-latest/*",
          "arn:aws:ssm:*:*:parameter/aws/service/canonical/ubuntu/server/*"
        ]
      }
    ]
  })
}

# Lambda function
resource "aws_lambda_function" "tailscale_exits" {
  filename         = data.archive_file.lambda_zip.output_path
  function_name    = var.lambda_function_name
  role            = aws_iam_role.lambda_role.arn
  handler         = "bootstrap"
  runtime         = "provided.al2023"
  architectures   = ["arm64"]

  memory_size = var.lambda_memory_size
  timeout     = var.lambda_timeout

  source_code_hash = data.archive_file.lambda_zip.output_base64sha256

  environment {
    variables = {
      TAILSCALE_AUTH_KEY = local.tailscale_auth_key
      TSE_AUTH_TOKEN     = local.lambda_auth_token
    }
  }

  tags = {
    Project = "tailscale-exits"
    Purpose = "ephemeral-vpn-nodes"
  }

  depends_on = [
    aws_iam_role_policy_attachment.lambda_basic_execution,
    aws_iam_role_policy.lambda_ec2_policy,
  ]
}

# Lambda function URL for direct HTTP access (no API Gateway needed)
resource "aws_lambda_function_url" "tailscale_exits_url" {
  function_name      = aws_lambda_function.tailscale_exits.function_name
  authorization_type = "NONE"

  cors {
    allow_credentials = false
    allow_origins     = ["*"]
    allow_methods     = ["GET", "POST", "DELETE"]
    allow_headers     = ["date", "keep-alive", "content-type", "authorization"]
    expose_headers    = ["date", "keep-alive"]
    max_age          = 86400
  }
}

# CloudWatch Log Group for Lambda function
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${aws_lambda_function.tailscale_exits.function_name}"
  retention_in_days = var.log_retention_days

  tags = {
    Project = "tailscale-exits"
    Purpose = "ephemeral-vpn-nodes"
  }
}