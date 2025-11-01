# tse - Tailscale Ephemeral Exit Nodes

A hobby tool to create on-demand Tailscale VPN exit nodes in AWS. Spin up exit nodes in any region with a single command, only costs money while exit nodes are running.

🤖 Built with Claude

## Should You Use This?

**Good fit:**
- You travel and want VPN exit nodes in different countries
- You want to avoid monthly VPN subscription fees ($5-10/month)
- You're comfortable with AWS and command-line tools
- You already use Tailscale

**Not a good fit:**
- You want a GUI (this is CLI-only)
- You need 24/7 exit nodes (just get a VPN subscription)
- You're new to AWS (learning curve isn't worth the savings)
- You need enterprise support or SLAs
- You don't trust compiled binaries (see [DIY.md](DIY.md) for the curl-only approach)
- You don't trust AI code (fair, but at least it's auditable)

**vs. Mullvad/ProtonVPN:**
- ✅ Cheaper for occasional use (pay per hour vs monthly fee)
- ✅ More regions available (any AWS region)
- ❌ More setup work (you manage the infrastructure)
- ❌ No mobile app (Tailscale app only)

## What It Does

- Creates temporary Tailscale exit nodes across AWS regions
- Uses cheap ARM instances (t4g.nano, $0.0042/hour)
- Automatically configures and approves exit nodes
- Cleanup stops instances so you don't get surprise bills

**Don't trust compiled tools?** Check out [DIY.md](DIY.md) for the curl-only approach (no CLI needed).

## Quick Start

**Already have TSE configured?** Jump to [Usage](#usage)

**First time?** Follow the [Complete Setup](#complete-setup-15-minutes) below (takes about 15 minutes).

## Complete Setup (15 minutes)

### Prerequisites

You need:
- **Tailscale** account and client installed (free tier works)
- **AWS** account with CLI configured (`aws configure`)
- **1Password** CLI (optional - [install guide](https://developer.1password.com/docs/cli/get-started/))

Not sure if you have these? Run:
```bash
tailscale status  # Should show your devices
aws sts get-caller-identity  # Should show your AWS account
```

### Step 1: Configure Tailscale (5 minutes)

**1.1 Create a Tailscale API token:**
1. Visit https://login.tailscale.com/admin/settings/keys
2. Click "Generate API key"
3. Give it a name (e.g., "TSE Setup")
4. Set expiration (90 days is fine)
5. Copy the token (starts with `tskey-api-`)

**1.2 Run the setup command:**
```bash
git clone https://github.com/anoldguy/tailscale-exits
cd tailscale-exits
export TAILSCALE_API_TOKEN=tskey-api-xxxxx
make build-cli
./bin/tse setup --tailnet yourname@github  # Use your tailnet name
```

Find your tailnet name by running `tailscale status` or checking your admin console URL.

This command will:
- Configure your Tailscale ACL for exit node auto-approval
- Create an auth key for exit nodes
- Optionally store it in 1Password

### Step 2: Deploy to AWS (8 minutes)

```bash
# Initialize OpenTofu (first time only)
make tofu-init

# Deploy everything
make deploy

# Save the Lambda URL and auth token
export TSE_LAMBDA_URL=$(cd deployments/opentofu && tofu output -raw lambda_function_url)
export TSE_AUTH_TOKEN=$(cd deployments/opentofu && tofu output -raw auth_token)

# Add to your shell profile for persistence
echo "export TSE_LAMBDA_URL=$TSE_LAMBDA_URL" >> ~/.bashrc
echo "export TSE_AUTH_TOKEN=$TSE_AUTH_TOKEN" >> ~/.bashrc
```

OpenTofu will show you what it's creating. Type `yes` to proceed.

### Step 3: Test It (2 minutes)

```bash
# Start an exit node in Ohio
./bin/tse ohio start

# Verify it's running
./bin/tse ohio instances

# In your Tailscale app, you should see "exit-ohio" as an available exit node
# Select it and your traffic routes through Ohio!

# When done
./bin/tse ohio stop
```

**That's it!** You now have on-demand exit nodes in any AWS region.

## Usage

```bash
# Health check
./bin/tse health

# Start exit node in any region
./bin/tse <region> start

# List running instances in a region
./bin/tse <region> instances

# Stop all instances in a region
./bin/tse <region> stop

# Stop exit nodes in ALL regions (prevents surprise bills!)
./bin/tse shutdown

# Check setup status
./bin/tse setup --status
```

## Available Regions

Use friendly names instead of AWS region codes:

- `ohio` (us-east-2)
- `virginia` (us-east-1)
- `oregon` (us-west-2)
- `california` (us-west-1)
- `ireland` (eu-west-1)
- `london` (eu-west-2)
- `frankfurt` (eu-central-1)
- `tokyo` (ap-northeast-1)
- `sydney` (ap-southeast-2)
- `singapore` (ap-southeast-1)

## How It Works

1. CLI calls Lambda Function URL
2. Lambda provisions EC2 instance with security group
3. Instance auto-installs Tailscale and registers as exit node
4. Exit node appears in your Tailscale admin console
5. Route traffic through it from any device

## What This Costs You

**TL;DR: Most hobby users spend $1-5/month**

### When Exit Nodes Are Running

**EC2 Instance (t4g.nano):**
- **$0.0042/hour** per exit node
- $3.02/month if left running 24/7
- Example: 4 hours/day = **~$0.50/month**
- Example: Weekend use (16 hours/month) = **~$0.07/month**

**Data Transfer:**
- First 100 GB/month: **Free**
- After that: **$0.09/GB** (AWS data transfer out)
- Typical streaming: ~3 GB/hour = 33 hours free

**Lambda (API endpoint):**
- First 1M requests/month: **Free**
- First 400,000 GB-seconds compute: **Free**
- Hobby usage stays within free tier: **$0**

### AWS Free Tier (First 12 Months)

- ✅ Lambda: Covered (you won't hit limits)
- ❌ t4g.nano instances: **Not free tier** (t2.micro is, but Intel not ARM)
- ✅ Data transfer: 100 GB/month free
- ⚠️  After 12 months: Same pricing, just lose the 100 GB data transfer buffer

**Want to use free-tier t2.micro instances?** See [DIY.md FAQ](DIY.md#faq) for instructions on switching to Intel instances (free for first 12 months, but more expensive after).

### Cost Calculator

**Your usage:** Running exit nodes `___` hours/month
- **EC2:** `___` hours × $0.0042 = $`___`
- **Data:** (`___` GB - 100 GB) × $0.09 = $`___`
- **Lambda:** $0 (free tier)
- **Total:** ~$`___`/month

**Links:**
- [AWS EC2 Pricing (t4g.nano)](https://aws.amazon.com/ec2/pricing/on-demand/)
- [AWS Lambda Pricing](https://aws.amazon.com/lambda/pricing/)
- [AWS Data Transfer Pricing](https://aws.amazon.com/ec2/pricing/on-demand/#Data_Transfer)

### What Gets Created in AWS

Each time you deploy:
- **Lambda Function** (ephemeral exit node API) - Free tier
- **IAM Role** (Lambda permissions) - Free
- **Function URL** (HTTP endpoint) - Free

Each time you start an exit node in a region (first time):
- **VPC** (10.0.0.0/16) - Free
- **Subnet** (10.0.1.0/24) - Free
- **Internet Gateway** - Free
- **Route Table** - Free
- **Security Group** - Free

Each time you start an exit node:
- **EC2 Instance** (t4g.nano ARM) - **$0.0042/hour**

Everything except running EC2 instances is free. VPCs and networking components cost $0.

## Cleanup

```bash
# Stop all exit nodes in ALL regions (recommended)
./bin/tse shutdown

# Or stop exit nodes in a specific region
./bin/tse <region> stop

# Remove all AWS infrastructure (Lambda, IAM roles, etc.)
make tofu-destroy
```

## Security

### Authentication

TSE uses token-based authentication to protect the Lambda Function URL. A random 256-bit token is generated during deployment:

```bash
# View your current auth token
cd deployments/opentofu && tofu output -raw auth_token
```

The CLI automatically includes this token in all requests via the `TSE_AUTH_TOKEN` environment variable.

### Token Rotation

If your token is compromised, rotate it:

```bash
cd deployments/opentofu
tofu taint random_id.lambda_auth_token
tofu apply

# Update your env var
export TSE_AUTH_TOKEN=$(tofu output -raw auth_token)
```

The old token is immediately invalidated when the new Lambda deploys.

### What's Protected

- ✅ Lambda Function URL requires valid token
- ✅ Token stored securely in OpenTofu state
- ✅ 256-bit entropy (same as good API keys)
- ✅ Constant-time comparison prevents timing attacks

### What's NOT Protected

- ⚠️ Lambda still creates resources in YOUR AWS account
- ⚠️ Anyone with your token can spin up instances (costing you money)
- ⚠️ Set up AWS billing alerts to catch unexpected charges

## Advanced Setup

### Manual Tailscale Configuration

If you prefer to configure Tailscale manually instead of using `tse setup`:

#### 1. Create Auth Key
In Tailscale Admin Console → Settings → Keys, create a key with:
- ✅ Reusable
- ✅ Ephemeral
- ✅ Tagged with `tag:exitnode`
- ✅ Pre-approved

Store it in 1Password at `op://private/Tailscale/CurrentAuthKey` or set it directly in your OpenTofu variables.

#### 2. Update ACL Policy
Add this to your Tailscale ACL at https://login.tailscale.com/admin/acls:

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

This configuration:
- Grants admin users ownership of the `tag:exitnode` tag
- Automatically approves devices tagged with `tag:exitnode` as exit nodes
- Eliminates manual approval steps when instances start

### Direct API Access

You can also call the Lambda endpoints directly with curl:

```bash
# Get the Lambda URL
LAMBDA_URL=$(cd deployments/opentofu && tofu output -raw lambda_function_url)

# Health check
curl -X GET "$LAMBDA_URL/"

# List instances in a region
curl -X GET "$LAMBDA_URL/{region}/instances"

# Start an exit node
curl -X POST "$LAMBDA_URL/{region}/start"

# Stop all instances in a region
curl -X POST "$LAMBDA_URL/{region}/stop"

# Force cleanup all resources in a region
curl -X POST "$LAMBDA_URL/{region}/cleanup"
```

Replace `{region}` with any friendly region name (ohio, virginia, etc.).

### Setup Command Options

```bash
# Check current configuration without making changes
./bin/tse setup --status

# Display auth key instead of storing in 1Password
./bin/tse setup --show-auth-key

# Preview ACL changes without applying
./bin/tse setup --show-acl-changes

# Skip ACL configuration (only create auth key)
./bin/tse setup --skip-acl

# Skip auth key creation (only configure ACL)
./bin/tse setup --skip-auth-key
```

---

This is a hobby project - simple, functional, and cost-effective for personal VPN needs.