# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TSE (Tailscale Ephemeral Exit Nodes) is a tool for creating on-demand Tailscale VPN exit nodes in AWS. The system consists of:
- **Lambda Function**: HTTP endpoint for provisioning AWS infrastructure
- **CLI Tool**: User-friendly command-line interface for managing exit nodes
- **Shared Libraries**: Common types and region mapping utilities

## Development Commands

### Building
```bash
# Build Lambda function (ARM64 for AWS)
make build-lambda

# Build CLI tool for local use
make build-cli

# Build both
make all
```

### Testing
```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose
```

### Deployment
```bash
# First time: Initialize OpenTofu providers (one-time)
make tofu-init

# Full deployment (clean build + infrastructure)
make deploy

# Individual OpenTofu operations
make tofu-plan           # Preview infrastructure changes
make tofu-apply          # Apply infrastructure changes
make tofu-init-upgrade   # Upgrade providers (manual, when desired)
make tofu-destroy        # Destroy all infrastructure

# Get Lambda URL for CLI usage
export TSE_LAMBDA_URL=$(cd deployments/opentofu && tofu output -raw lambda_function_url)
```

**Deployment Flow:**
- `make deploy` runs: clean → tofu-init → tofu-apply → package-lambda → build-lambda
- Ensures Lambda is always rebuilt cleanly before deployment
- Uses provider lock file for reproducible deployments
- Run `make tofu-init-upgrade` manually when you want to upgrade providers

### Using the CLI
```bash
# Build and test CLI locally
make build-cli
./bin/tse health
./bin/tse ohio start
./bin/tse ohio instances
./bin/tse ohio stop
```

## What You Need to Know

### File Structure
```
cmd/tse/          # CLI tool (setup command, region operations)
lambda/           # Lambda handler + AWS service layer
shared/
  regions/        # Friendly name ↔ AWS region mapping
  types/          # Request/response types (Lambda ↔ CLI)
  tailscale/      # Tailscale API client + ACL logic
  onepassword/    # 1Password CLI integration
```

### Tagging Strategy (Critical!)

**All AWS resources are tagged:**
- `Project=tse`
- `Type=ephemeral`
- `Region=<friendly-region>`

**Why this matters:** Cleanup relies entirely on these tags. If you manually create resources without tags, `tse stop` won't find them. If you modify tags, cleanup will break.

**Finding orphaned resources:** `tse <region> cleanup` force-deletes everything with these tags.

### VPC Lifecycle (Important!)

**One VPC per region**, created automatically on first `start` in that region.

**Cleanup behavior:**
- `StopInstances()` waits 30 seconds, then deletes VPC if no instances remain
- Async cleanup can fail silently (detached IGW, lingering ENIs, etc.)
- Use `cleanup` endpoint to force-delete everything if VPCs get stuck

**Common issue:** VPC delete fails if instances still terminating. Wait 60 seconds and retry cleanup.

### Adding New Regions

Edit `shared/regions/regions.go`:
```go
var friendlyToAWS = map[string]string{
    "ohio": "us-east-2",
    "yournewregion": "ap-south-2",  // Add here
}
```

Both Lambda and CLI use the same mapping. Rebuild both after changes.

### Tailscale Integration

The Lambda requires `TAILSCALE_AUTH_KEY` environment variable (configured in OpenTofu).

Auth key requirements (created in Tailscale admin console):
- ✅ Reusable
- ✅ Ephemeral (instances auto-removed when terminated)
- ✅ Tagged with `tag:exitnode`
- ✅ Pre-approved

Tailscale ACL must include:
```json
{
  "tagOwners": {
    "tag:exitnode": ["autogroup:admin"]
  },
  "autoApprovers": {
    "exitNode": ["tag:exitnode"]
  }
}
```

**Critical:** If ACL config doesn't match auth key tags, instances won't auto-approve. You'll see them in Tailscale admin but not as exit nodes.

### Tailscale API Integration

The `tse setup` command will automate Tailscale configuration using the Tailscale API:

**API Endpoints:**
- `GET /api/v2/tailnet/{tailnet}/acl` - Retrieve current ACL policy (includes ETag)
- `POST /api/v2/tailnet/{tailnet}/acl` - Update ACL policy (full replacement)
- `POST /api/v2/tailnet/{tailnet}/acl/validate` - Validate ACL before applying
- `POST /api/v2/tailnet/{tailnet}/keys` - Create auth keys programmatically

**Authentication:**
- Requires `TAILSCALE_API_TOKEN` environment variable (API access token)
- User must be an Owner or Admin on the Tailscale network
- API access tokens inherit full permissions from the creating user (no scope selection)
- Tokens created at https://login.tailscale.com/admin/settings/keys
- Note: OAuth clients (with scopes) are also supported but not required for hobby use

**Setup Command Behavior:**
- Reads existing ACL policy and merges in required configuration (idempotent)
- Only adds `tag:exitnode` to `tagOwners` if not already present
- Only adds exit node auto-approval if not already configured
- Creates auth key with: reusable=true, ephemeral=true, tags=["tag:exitnode"], preauthorized=true
- Stores auth key in 1Password (optional) or displays for manual storage
- Uses ETag-based collision avoidance when updating ACL (If-Match header)
- Validates ACL changes before applying

**Error Handling Patterns:**
- Detect insufficient API token permissions and guide user to check Owner/Admin role
- Validate ACL syntax before applying changes
- Handle concurrent ACL modifications gracefully (retry with updated ETag)
- Provide clear error messages with actionable next steps

## Common Gotchas

### Lambda Returns 404 for Everything
Check `TSE_LAMBDA_URL` is set and points to Function URL (not API Gateway).

### Exit Node Doesn't Appear in Tailscale
1. Check ACL has `tag:exitnode` in tagOwners and autoApprovers
2. Check auth key has `tag:exitnode` (stored in 1Password or OpenTofu var)
3. Wait 60 seconds - instance needs time to install Tailscale

### VPC Won't Delete
Instances still terminating. Wait 60 seconds and run `tse <region> cleanup`.

### "Region not found" Error
Add region to `shared/regions/regions.go` and rebuild CLI + Lambda.

## Testing

Tests are colocated with implementation:
- `shared/regions/regions_test.go`: Region mapping validation
- `shared/types/types_test.go`: Type serialization
- `lambda/aws/service_test.go`: AWS service mocking

Run specific package tests:
```bash
go test ./shared/regions -v
go test ./lambda/aws -v
```

## Dependencies

- `github.com/aws/aws-lambda-go`: Lambda runtime and event types
- `github.com/aws/aws-sdk-go-v2`: AWS SDK for EC2 operations
- Go 1.23 (specified in `go.mod`)

## OpenTofu/Terraform

Infrastructure code in `deployments/opentofu/`:
- `main.tf`: Lambda function, IAM roles, Function URL
- `variables.tf`: Configurable inputs (Tailscale auth key path)
- `outputs.tf`: Lambda Function URL output
- Uses 1Password CLI to fetch Tailscale auth key: `op://private/Tailscale/CurrentAuthKey`
