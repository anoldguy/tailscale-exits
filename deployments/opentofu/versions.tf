# versions.tf - Provider version constraints and requirements

terraform {
  required_version = ">= 1.5"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 2.3"
    }
  }
}

# AWS Provider configuration
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = merge({
      Project     = "tailscale-exits"
      Purpose     = "ephemeral-vpn-nodes"
      ManagedBy   = "opentofu"
      Environment = "production"
    }, var.tags)
  }
}