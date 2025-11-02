# DIY Guide - Roll Your Own TSE

Don't trust compiled binaries? Prefer curl? This guide shows you how to do everything the `tse` CLI does, manually.

**What this guide covers:**
- Configuring Tailscale ACL with raw API calls
- Creating Tailscale auth keys with curl
- Deploying infrastructure with auditable Go code
- Managing exit nodes with curl (no CLI needed)

**Why you might want this:**
- Audit what the tool actually does
- Integrate into your own scripts
- Just prefer knowing exactly what's happening
- Don't trust Go binaries from random GitHub repos (smart!)

---

## Part 1: Configure Tailscale (Manual curl edition)

### Step 1: Get a Tailscale API Token

1. Visit: https://login.tailscale.com/admin/settings/keys
2. Click "Generate API key"
3. Copy the token (starts with `tskey-api-`)

```bash
export TAILSCALE_API_TOKEN="tskey-api-xxxxx"
export TAILNET="yourname@github"  # or your org domain
```

### Step 2: Fetch Current ACL Policy

```bash
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/acl \
  -H "Accept: application/json" > current-acl.json
```

This returns JSON with:
- `acl`: The actual ACL policy
- `etag`: Version identifier for collision avoidance

Save the `etag` for later:
```bash
ETAG=$(jq -r '.etag' current-acl.json)
echo "Current ETag: $ETAG"
```

### Step 3: Modify the ACL Policy

Edit `current-acl.json` and add these sections (merge with existing config):

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

**What this does:**
- `tagOwners`: Lets admins own the `tag:exitnode` tag
- `autoApprovers`: Auto-approves any device with `tag:exitnode` as an exit node

Extract just the ACL policy (without the etag wrapper):
```bash
jq '.acl' current-acl.json > updated-acl.json
```

### Step 4: Validate the Updated ACL

```bash
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/acl/validate \
  -X POST \
  -H "Content-Type: application/json" \
  -d @updated-acl.json
```

If valid, you'll get: `{"message":"OK"}`

### Step 5: Apply the ACL Changes

```bash
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/acl \
  -X POST \
  -H "Content-Type: application/json" \
  -H "If-Match: $ETAG" \
  -d @updated-acl.json
```

**Notes:**
- The `If-Match` header prevents conflicts if someone else modified the ACL
- If you get HTTP 412, someone else changed the ACL - refetch and retry
- If you get HTTP 403, your API token needs Owner/Admin permissions

### Step 6: Create a Reusable Auth Key

```bash
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/keys \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": {
      "devices": {
        "create": {
          "reusable": true,
          "ephemeral": true,
          "tags": ["tag:exitnode"],
          "preauthorized": true
        }
      }
    },
    "expirySeconds": 0,
    "description": "TSE exit node auth key (manual)"
  }'
```

This returns:
```json
{
  "id": "...",
  "key": "tskey-auth-xxxxx",
  "created": "...",
  "expires": "..."
}
```

**Save this auth key:**
```bash
AUTH_KEY="tskey-auth-xxxxx"

# Store in 1Password (optional)
echo -n "$AUTH_KEY" | op item create \
  --category=password \
  --title="Tailscale Auth Key" \
  --vault=private \
  CurrentAuthKey[password]=-

# Or just save it somewhere secure for deployment
echo "TAILSCALE_AUTH_KEY=$AUTH_KEY" >> .env
```

**Auth key settings explained:**
- `reusable: true` - Can be used multiple times (for multiple EC2 instances)
- `ephemeral: true` - Instances auto-removed from Tailscale when they stop
- `tags: ["tag:exitnode"]` - Instances get tagged (matches ACL auto-approval)
- `preauthorized: true` - Skip manual approval in admin console
- `expirySeconds: 0` - Never expires (hobby-friendly)

---

## Part 2: Deploy Lambda Infrastructure (AWS CLI)

Deploy everything using raw AWS CLI commands. No compiled tools needed.

### Step 1: Build the Lambda Function

```bash
# Compile Lambda for ARM64
cd lambda
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap .

# Create deployment zip
zip lambda.zip bootstrap
cd ..
```

### Step 2: Create CloudWatch Log Group

```bash
aws logs create-log-group \
  --log-group-name /aws/lambda/tailscale-exits \
  --tags ManagedBy=tse

aws logs put-retention-policy \
  --log-group-name /aws/lambda/tailscale-exits \
  --retention-in-days 14
```

### Step 3: Create IAM Role

```bash
# Create trust policy for Lambda
cat > /tmp/trust-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"Service": "lambda.amazonaws.com"},
    "Action": "sts:AssumeRole"
  }]
}
EOF

# Create the role
aws iam create-role \
  --role-name tailscale-exits-lambda-role \
  --assume-role-policy-document file:///tmp/trust-policy.json \
  --tags Key=ManagedBy,Value=tse

# Attach AWS Lambda basic execution policy
aws iam attach-role-policy \
  --role-name tailscale-exits-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
```

### Step 4: Create Inline Policy for EC2 Access

```bash
cat > /tmp/ec2-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances", "ec2:TerminateInstances",
        "ec2:DescribeInstances", "ec2:DescribeInstanceStatus",
        "ec2:DescribeImages", "ec2:CreateSecurityGroup",
        "ec2:DeleteSecurityGroup", "ec2:DescribeSecurityGroups",
        "ec2:AuthorizeSecurityGroupIngress", "ec2:AuthorizeSecurityGroupEgress",
        "ec2:RevokeSecurityGroupIngress", "ec2:RevokeSecurityGroupEgress",
        "ec2:DescribeVpcs", "ec2:CreateVpc", "ec2:DescribeSubnets",
        "ec2:CreateSubnet", "ec2:ModifySubnetAttribute",
        "ec2:DescribeAvailabilityZones", "ec2:DescribeRouteTables",
        "ec2:CreateRoute", "ec2:DescribeInternetGateways",
        "ec2:CreateInternetGateway", "ec2:AttachInternetGateway",
        "ec2:DetachInternetGateway", "ec2:DeleteInternetGateway",
        "ec2:DeleteSubnet", "ec2:DeleteVpc", "ec2:DeleteRoute",
        "ec2:CreateTags", "ec2:DescribeTags"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": ["ssm:GetParameter", "ssm:GetParameters"],
      "Resource": [
        "arn:aws:ssm:*:*:parameter/aws/service/ami-amazon-linux-latest/*",
        "arn:aws:ssm:*:*:parameter/aws/service/canonical/ubuntu/server/*"
      ]
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name tailscale-exits-lambda-role \
  --policy-name tailscale-exits-lambda-ec2-policy \
  --policy-document file:///tmp/ec2-policy.json
```

### Step 5: Wait for IAM Propagation

```bash
# IAM is eventually consistent - give it time to propagate
echo "Waiting 10 seconds for IAM propagation..."
sleep 10
```

### Step 6: Generate Auth Token

```bash
# Generate a random 256-bit token
TSE_AUTH_TOKEN=$(openssl rand -hex 32)
echo "Generated TSE_AUTH_TOKEN: $TSE_AUTH_TOKEN"
echo "Save this token - you'll need it for all API calls!"
```

### Step 7: Create Lambda Function

```bash
# Get the role ARN
ROLE_ARN=$(aws iam get-role \
  --role-name tailscale-exits-lambda-role \
  --query 'Role.Arn' \
  --output text)

# Create Lambda function
aws lambda create-function \
  --function-name tailscale-exits \
  --runtime provided.al2023 \
  --role "$ROLE_ARN" \
  --handler bootstrap \
  --architectures arm64 \
  --memory-size 256 \
  --timeout 60 \
  --zip-file fileb://lambda/lambda.zip \
  --environment Variables="{TAILSCALE_AUTH_KEY=$TAILSCALE_AUTH_KEY,TSE_AUTH_TOKEN=$TSE_AUTH_TOKEN}" \
  --tags ManagedBy=tse
```

### Step 8: Create Function URL

```bash
# Create Function URL with NONE auth (we use Bearer tokens)
aws lambda create-function-url-config \
  --function-name tailscale-exits \
  --auth-type NONE \
  --cors '{
    "AllowCredentials": false,
    "AllowOrigins": ["*"],
    "AllowMethods": ["GET", "POST", "DELETE"],
    "AllowHeaders": ["date", "keep-alive", "content-type", "authorization"],
    "ExposeHeaders": ["date", "keep-alive"],
    "MaxAge": 86400
  }'

# Add permission for public invocation
aws lambda add-permission \
  --function-name tailscale-exits \
  --statement-id FunctionURLAllowPublicAccess \
  --action lambda:InvokeFunctionUrl \
  --principal '*' \
  --function-url-auth-type NONE

# Get the Function URL
TSE_LAMBDA_URL=$(aws lambda get-function-url-config \
  --function-name tailscale-exits \
  --query 'FunctionUrl' \
  --output text)

echo "Lambda URL: $TSE_LAMBDA_URL"
```

### Step 9: Save Your Credentials

```bash
echo "Save these environment variables:"
echo "export TSE_LAMBDA_URL=$TSE_LAMBDA_URL"
echo "export TSE_AUTH_TOKEN=$TSE_AUTH_TOKEN"
```

**What you created:**
- CloudWatch Log Group (14 day retention)
- IAM Role with EC2/VPC permissions
- Lambda Function (ARM64, ~10MB)
- Function URL (public HTTP endpoint)
- Auth token for API security

**Cost:** All free tier (Lambda free for hobby usage)

---

## Part 3: Manage Exit Nodes with curl

Now you can manage exit nodes without the CLI at all. All requests require authentication via Bearer token.

### Set Up Auth Token

```bash
# Use the token from your deploy output
export TSE_AUTH_TOKEN="<from-deploy-output>"
```

### Health Check

```bash
curl -X GET "$LAMBDA_URL/" \
  -H "Authorization: Bearer $TSE_AUTH_TOKEN"
```

Returns:
```json
{
  "status": "healthy",
  "version": "1.0",
  "timestamp": "2025-11-01T12:34:56Z"
}
```

### Start an Exit Node

```bash
curl -X POST "$LAMBDA_URL/ohio/start" \
  -H "Authorization: Bearer $TSE_AUTH_TOKEN"
```

Returns:
```json
{
  "success": true,
  "message": "Exit node started successfully",
  "instance": {
    "instance_id": "i-0abc123def456",
    "instance_type": "t4g.nano",
    "state": "running",
    "tailscale_hostname": "exit-ohio",
    "public_ip": "3.21.45.67",
    "launch_time": "2025-11-01T12:35:00Z"
  }
}
```

**Available regions:**
Replace `ohio` with: `virginia`, `oregon`, `california`, `canada`, `ireland`, `london`, `paris`, `frankfurt`, `stockholm`, `singapore`, `sydney`, `tokyo`, `seoul`, `mumbai`, `saopaulo`

### List Running Instances in a Region

```bash
curl -X GET "$LAMBDA_URL/ohio/instances" \
  -H "Authorization: Bearer $TSE_AUTH_TOKEN"
```

Returns:
```json
{
  "success": true,
  "count": 1,
  "instances": [
    {
      "instance_id": "i-0abc123def456",
      "instance_type": "t4g.nano",
      "state": "running",
      "tailscale_hostname": "exit-ohio",
      "public_ip": "3.21.45.67",
      "launch_time": "2025-11-01T12:35:00Z"
    }
  ]
}
```

### Stop Exit Nodes in a Region

```bash
curl -X POST "$LAMBDA_URL/ohio/stop" \
  -H "Authorization: Bearer $TSE_AUTH_TOKEN"
```

Returns:
```json
{
  "success": true,
  "message": "Terminated 1 instance(s)",
  "terminated_count": 1,
  "terminated_ids": ["i-0abc123def456"]
}
```

### Force Cleanup Orphaned Resources

If VPCs or security groups get stuck:

```bash
curl -X POST "$LAMBDA_URL/ohio/cleanup" \
  -H "Authorization: Bearer $TSE_AUTH_TOKEN"
```

This aggressively deletes ALL TSE-tagged resources in the region.

### Stop Exit Nodes in ALL Regions (Prevents Surprise Bills!)

```bash
# Loop through all regions
for region in ohio virginia oregon california canada \
              ireland london paris frankfurt stockholm \
              singapore sydney tokyo seoul mumbai saopaulo; do
  echo "Checking $region..."
  response=$(curl -s -X POST "$LAMBDA_URL/$region/stop" \
    -H "Authorization: Bearer $TSE_AUTH_TOKEN")
  count=$(echo "$response" | jq -r '.terminated_count // 0')
  if [ "$count" -gt 0 ]; then
    echo "  ✓ Terminated $count instance(s)"
  fi
done
```

---

## Part 4: Understanding What's Happening

### When you start an exit node, the Lambda:

1. **Creates VPC infrastructure** (first time in a region):
   - VPC: 10.0.0.0/16
   - Public subnet in first available AZ
   - Internet gateway
   - Route table (0.0.0.0/0 → IGW)
   - Security group (UDP 41641 for WireGuard, TCP 22 for SSH)

2. **Launches EC2 instance**:
   - Type: `t4g.nano` (ARM64, $0.0042/hour)
   - AMI: Latest Amazon Linux 2023 ARM64
   - Tags: `Project=tse`, `Type=ephemeral`, `Region=<name>`

3. **User data script runs on boot**:
   ```bash
   #!/bin/bash
   # Install Tailscale
   curl -fsSL https://tailscale.com/install.sh | sh

   # Enable IP forwarding
   echo 'net.ipv4.ip_forward = 1' >> /etc/sysctl.conf
   echo 'net.ipv6.conf.all.forwarding = 1' >> /etc/sysctl.conf
   sysctl -p

   # Start Tailscale with auth key and advertise as exit node
   tailscale up \
     --authkey="$TAILSCALE_AUTH_KEY" \
     --hostname="exit-ohio" \
     --advertise-exit-node \
     --ssh
   ```

4. **Instance auto-registers** with your Tailscale network:
   - Tagged with `tag:exitnode`
   - Auto-approved as exit node (thanks to ACL config)
   - Shows up as "exit-ohio" in your Tailscale admin console

### When you stop exit nodes:

1. **Terminates EC2 instances** with TSE tags
2. **Waits 30 seconds** (async goroutine)
3. **Cleans up VPC** if no instances remain:
   - Deletes security group
   - Deletes route table
   - Detaches and deletes internet gateway
   - Deletes subnet
   - Deletes VPC

### Cost breakdown:

**Running costs:**
- EC2 instance: **$0.0042/hour** ($3.02/month if running 24/7)
- Data transfer: First 100 GB/month free, then **$0.09/GB**

**Free resources:**
- Lambda (API endpoint): Free tier covers hobby usage
- VPC, subnet, IGW, route table, security group: **$0**

**Example monthly cost:**
- 4 hours/day usage: ~**$0.50/month**
- Weekend trips (16 hours/month): ~**$0.07/month**

---

## Part 5: Auditing the Code

Since you're reading this, you probably want to audit the code yourself.

**Key files to review:**

1. **Lambda handler** (`lambda/main.go`):
   - Routes HTTP requests to AWS service
   - No authentication (relies on Function URL being private)
   - ~200 lines

2. **AWS service** (`lambda/aws/service.go`):
   - All EC2 API calls
   - VPC creation, instance launching, cleanup
   - ~600 lines

3. **Tailscale API client** (`shared/tailscale/`):
   - ACL manipulation
   - Auth key creation
   - ~400 lines total

4. **CLI tool** (`cmd/tse/main.go`):
   - Just HTTP client wrapper around Lambda
   - ~350 lines

**No weird stuff:**
- No telemetry
- No phone-home
- No credential exfiltration
- Just AWS SDK and HTTP calls

**Build it yourself:**
```bash
# Verify checksums, inspect code, build from source
git clone https://github.com/anoldguy/tailscale-exits
cd tailscale-exits
make build-cli
make build-lambda

# Run tests
make test
```

---

## Part 6: Cleanup

### Remove All AWS Infrastructure

Delete everything in reverse order of creation:

```bash
# 1. Delete Function URL
aws lambda delete-function-url-config \
  --function-name tailscale-exits

# 2. Delete Lambda function
aws lambda delete-function \
  --function-name tailscale-exits

# 3. Delete inline policy
aws iam delete-role-policy \
  --role-name tailscale-exits-lambda-role \
  --policy-name tailscale-exits-lambda-ec2-policy

# 4. Detach managed policy
aws iam detach-role-policy \
  --role-name tailscale-exits-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# 5. Delete IAM role
aws iam delete-role \
  --role-name tailscale-exits-lambda-role

# 6. Delete CloudWatch log group
aws logs delete-log-group \
  --log-group-name /aws/lambda/tailscale-exits
```

**Note:** If you have running exit nodes, stop them first using the curl commands from Part 3, or manually terminate instances via AWS console.

### Revoke Tailscale Auth Key (Optional)

```bash
# List all auth keys
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/keys

# Delete a specific key
curl -u "$TAILSCALE_API_TOKEN:" \
  https://api.tailscale.com/api/v2/tailnet/$TAILNET/keys/<key-id> \
  -X DELETE
```

---

## FAQ

**Q: Is the Lambda Function URL publicly accessible?**
A: Yes, but it only creates resources in YOUR AWS account. An attacker could spin up instances for you (costing you money), but that's it. If you're paranoid, add AWS WAF or IP allowlisting.

**Q: Can I use this without the Go CLI tool at all?**
A: Absolutely! That's the point of this guide. Use curl, integrate into your shell scripts, whatever.

**Q: How do I add more regions?**
A: Edit `shared/regions/regions.go`, add your region mapping, rebuild Lambda and CLI. No other changes needed.

**Q: Can I use free-tier Intel instances instead of ARM?**
A: Yes! If you're within your first 12 months of AWS and want to use the free tier, switch to `t2.micro` (Intel). Here's what changes:

1. **Edit `lambda/aws/service.go`** (around line 150):
   ```go
   // Change from:
   InstanceType: types.InstanceTypeT4gNano,

   // To:
   InstanceType: types.InstanceTypeT2Micro,
   ```

2. **Rebuild Lambda for x86_64**:
   ```bash
   cd lambda
   GOOS=linux GOARCH=amd64 go build -o bootstrap .  # Note: amd64, not arm64
   zip lambda.zip bootstrap
   cd ..
   ```

3. **Update Lambda function**:
   ```bash
   aws lambda update-function-code \
     --function-name tailscale-exits \
     --zip-file fileb://lambda/lambda.zip \
     --architectures x86_64
   ```

**Cost comparison:**
- **t4g.nano (ARM)**: $0.0042/hour = $3.02/month (no free tier)
- **t2.micro (Intel)**: $0.0116/hour = $8.35/month (750 hours/month free for first 12 months)

**Recommendation:**
- First 12 months of AWS: Use t2.micro (free)
- After 12 months: Switch to t4g.nano (63% cheaper)

**Q: Can I use other instance types?**
A: Sure! Edit `lambda/aws/service.go` to any instance type. Options:
- **t3.micro** (Intel, $0.0104/hour) - slightly cheaper than t2.micro, not free tier
- **t4g.small** (ARM, $0.0084/hour) - more memory if you need it
- **t3a.nano** (AMD, $0.0047/hour) - cheapest Intel option

Just remember to rebuild Lambda with the right architecture (arm64 for t4g, amd64 for t2/t3).

**Q: This seems overly complex for a VPN.**
A: It is! Just use Mullvad or ProtonVPN if you want simple. This is for tinkerers who want AWS everywhere.

---

## Security Considerations

**What you're trusting:**
- ✅ Tailscale (already using it)
- ✅ AWS (already using it)
- ⚠️  This Go code (small, auditable, but still code from the internet)

**What you're NOT trusting:**
- ❌ Commercial VPN providers (Mullvad, ProtonVPN, etc.)
- ❌ Pre-built binaries (if you build from source)

**Attack surface:**
- Lambda Function URL protected by Bearer token auth (256-bit random token)
- EC2 instances have public IPs (WireGuard UDP 41641 exposed)
- SSH exposed on TCP 22 (use AWS security groups to restrict if needed)

**What auth protects:**
- ✅ Prevents unauthorized Lambda invocations
- ✅ 256-bit entropy (same as good API keys)
- ✅ Constant-time comparison (prevents timing attacks)
- ✅ Token rotation supported (generate new token and update Lambda)

**What auth does NOT protect:**
- ⚠️ Lambda still creates resources in YOUR AWS account
- ⚠️ Anyone with your token can spin up instances (costing you money)
- ⚠️ Keep your token secret (don't commit to git, don't share publicly)

**Mitigations:**
- Set up AWS billing alerts for unexpected charges
- Rotate token if compromised:
  ```bash
  # Generate new token
  NEW_TOKEN=$(openssl rand -hex 32)

  # Update Lambda environment variables
  aws lambda update-function-configuration \
    --function-name tailscale-exits \
    --environment Variables="{TAILSCALE_AUTH_KEY=$TAILSCALE_AUTH_KEY,TSE_AUTH_TOKEN=$NEW_TOKEN}"

  # Update your local .env file
  echo "TSE_AUTH_TOKEN=$NEW_TOKEN" >> .env
  ```
- Review CloudTrail logs periodically
- Restrict Lambda Function URL to your IP via AWS WAF (if extra paranoid)

---

**Bottom line:** If you're comfortable with curl and don't trust the CLI tool, this guide shows you everything you need. The `tse` CLI is just a thin wrapper for convenience - all the real work happens via HTTP APIs you can call yourself.
