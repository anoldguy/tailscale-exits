# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TSE (Tailscale Ephemeral Exit Nodes) is a tool for creating on-demand Tailscale VPN exit nodes in AWS. The system consists of:
- **Lambda Function**: HTTP endpoint for provisioning AWS infrastructure
- **CLI Tool**: User-friendly command-line interface for managing exit nodes and deploying infrastructure
- **Shared Libraries**: Common types and region mapping utilities

## Development Commands

### Building
```bash
# Build CLI tool for local use
make build-cli

# Build Lambda function (ARM64 for AWS) - optional, deploy does this
make build-lambda

# Build and test
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
# Deploy AWS infrastructure (Lambda, IAM, logs, Function URL)
./bin/tse deploy

# Check deployment status
./bin/tse status

# Remove all infrastructure
./bin/tse teardown
```

**Deployment Flow:**
- `tse deploy` compiles Lambda from source, creates all AWS resources
- Uses tag-based discovery (`ManagedBy=tse`) for state management
- Converges on re-run: missing resources are created, and an existing function
  gets both its code and its environment updated
- No local state files required

**Environment Variables Required:**
- `TAILSCALE_OAUTH_SECRET` - OAuth client secret; the Lambda passes it to `tailscale up`, which mints a single-use tagged key per node
- `TSE_AUTH_TOKEN` - Generated during deploy, used for Lambda API auth
- `TSE_LAMBDA_URL` - Function URL, output by deploy

Store these in `.env` file (see `.env.example`).

### Using the CLI
```bash
# Build and test CLI locally
make build-cli
./bin/tse status        # Check infrastructure deployment
./bin/tse health        # Check Lambda health
./bin/tse ohio start    # Start exit node
./bin/tse ohio instances
./bin/tse ohio stop
```

## What You Need to Know

### File Structure
```
cmd/tse/
  infrastructure/   # Native AWS deployment (discovery, create, delete, setup, teardown)
  *.go             # CLI commands (setup, deploy, status, teardown, region operations)
lambda/           # Lambda handler + AWS service layer
shared/
  regions/        # Friendly name ↔ AWS region mapping
  types/          # Request/response types (Lambda ↔ CLI)
  tailscale/      # Tailscale API client + ACL logic
```

### Infrastructure Management

**Native Go Deployment:**
- No external tools required (no Terraform/OpenTofu)
- Uses AWS SDK v2 (IAM, Lambda, CloudWatch Logs)
- Tag-based resource discovery
- Creates 6 resources: Log Group, IAM Role, 2 Policies, Lambda Function, Function URL

**Region Behavior:**
- TSE infrastructure (Lambda, IAM role, CloudWatch logs) deploys to the user's default AWS region
- Default region determined by (in order):
  1. `AWS_REGION` environment variable
  2. `AWS_DEFAULT_REGION` environment variable
  3. `~/.aws/config` default region
- Exit nodes can launch in **any AWS region** regardless of where control plane is deployed
- Infrastructure code uses `GetDefaultRegion()` helper to load region from AWS config

**Tagging Strategy for Infrastructure:**
All managed infrastructure resources tagged with:
- `ManagedBy=tse`

**Tagging Strategy for Exit Nodes:**
All EC2 resources (instances, VPCs, etc.) tagged with:
- `Project=tse`
- `Type=ephemeral`
- `Region=<friendly-region>`

**Why this matters:**
- Infrastructure discovery relies on `ManagedBy=tse` tag
- Cleanup for exit nodes relies on `Project=tse` and `Type=ephemeral`
- If you manually create resources without tags, cleanup won't find them

**Finding orphaned resources:** `tse <region> cleanup` force-deletes everything with exit node tags.

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

Both Lambda and CLI use the same mapping. Rebuild CLI after changes.

### Tailscale Integration

The Lambda requires `TAILSCALE_OAUTH_SECRET` environment variable (set during deployment).

This is an OAuth client secret (`tskey-client-...`), not an auth key. Auth keys
cap at 90 days and cannot be renewed, which is how the original build quietly
broke three months after it shipped. OAuth client secrets do not expire.

OAuth client requirements (created manually at
https://login.tailscale.com/admin/settings/oauth; there is no API for it):
- ✅ `auth_keys` scope with write access
- ✅ Assigned the `tag:exitnode` tag

The node's user data passes the secret as
`--auth-key='<secret>?ephemeral=true&preauthorized=true'` along with
`--advertise-tags=tag:exitnode`. Tailscale mints a single-use tagged key at
registration. The single quotes are load-bearing: unquoted, bash reads the `&`
as "background this command" and `preauthorized` never arrives.

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

**Critical:** The ACL is load-bearing in two separate ways, and the OAuth
client's settings do not substitute for either. `tagOwners` is what makes
`tag:exitnode` exist at all, and `autoApprovers.exitNode` is what approves the
advertised exit node route. The `preauthorized` parameter only covers device
approval, which is a different question. Get the second one wrong and instances
appear in Tailscale admin but not as exit nodes, waiting on a manual click.

### Tailscale API Integration

The `tse setup` command automates Tailscale configuration using the Tailscale API:

**API Endpoints:**
- `GET /api/v2/tailnet/{tailnet}/acl` - Retrieve current ACL policy (includes ETag)
- `POST /api/v2/tailnet/{tailnet}/acl` - Update ACL policy (full replacement)
- `POST /api/v2/tailnet/{tailnet}/acl/validate` - Validate ACL before applying

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
- Prints instructions for creating the OAuth client; it no longer mints auth
  keys, because a key it minted would expire in 90 days and cannot be renewed
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
2. Check the OAuth client has the `tag:exitnode` tag (secret stored in `.env` as `TAILSCALE_OAUTH_SECRET`)
3. Wait 60 seconds - instance needs time to install Tailscale

### Exit Node Boots But Never Joins The Tailnet
Read the console output first: `aws ec2 get-console-output --region <aws-region>
--instance-id i-xxxx --latest --output text`. AL2023 tees cloud-init to the
console, so this gives you `/var/log/cloud-init-output.log` without needing to
reach the box. The user data script traps errors and names the failing line.

Do not switch the install back to `tailscale.com/install.sh`. On AL2023 it takes
the yum path, and refreshing the repo metadata needs more memory than a
`t4g.nano` has, so dnf gets OOM-killed and Tailscale never lands. The static
tarball is a download and four copies, which fits in 512MB. This failure looks
regional because the metadata has grown over time and older instances survived
it; it is not.

### VPC Won't Delete
Instances still terminating. Wait 60 seconds and run `tse <region> cleanup`.

### "Region not found" Error
Add region to `shared/regions/regions.go` and rebuild CLI + Lambda.

### "No AWS region configured" Error
User hasn't set up AWS CLI properly. They need to either:
- Run `aws configure` and set a default region
- Set `AWS_REGION` environment variable
- Ensure `~/.aws/config` has a region configured

### Deployment Fails with IAM Permission Errors
The deploying IAM user needs specific permissions. See README.md "Required IAM permissions" section for the minimal policy. Quick fix: use `AdministratorAccess` managed policy (if account owner).

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
- `github.com/aws/aws-sdk-go-v2`: AWS SDK for EC2, IAM, Lambda, CloudWatch Logs
- Go 1.23 (specified in `go.mod`)

## Native Deployment Architecture

Infrastructure deployment is pure Go using AWS SDK v2:

**Discovery** (`cmd/tse/infrastructure/discovery.go`):
- Tag-based resource discovery (no local state)
- Queries AWS for resources by name and validates tags
- Returns InfrastructureState with what exists

**Creation** (`cmd/tse/infrastructure/create.go`):
- buildLambdaZip() - compiles Lambda for linux/arm64 in-memory
- Creates: Log Group, IAM Role, Policies, Lambda Function, Function URL
- Adds resource-based policy for Function URL public access
- Also holds the update path: code and configuration are separate API calls, so
  updateLambdaCode() and updateLambdaConfiguration() both run on a redeploy with
  a waiter between them. mergeLambdaEnvironment() reads the deployed variables
  first, because UpdateFunctionConfiguration replaces the whole map and a naive
  write would blank out TSE_AUTH_TOKEN.

**Deletion** (`cmd/tse/infrastructure/delete.go`):
- Deletes resources in reverse dependency order
- Policies must be removed before IAM role deletion

**Setup** (`cmd/tse/infrastructure/setup.go`):
- Orchestrates idempotent deployment
- Handles IAM eventual consistency (10s wait)
- Generates TSE_AUTH_TOKEN if not provided on first deploy; on a redeploy an
  unset token means "keep what is live" rather than "generate a new one"

**Teardown** (`cmd/tse/infrastructure/teardown.go`):
- Discovers and deletes all resources
- Detects legacy resources (without ManagedBy tag)
- Requires confirmation before deletion
