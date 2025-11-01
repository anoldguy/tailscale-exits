#!/bin/bash

# Exit if any of the intermediate steps fail
set -e

# Get both Tailscale auth key and CLI API key from 1Password
TS_AUTH=$(op read op://private/Tailscale/CurrentAuthKey)
API_KEY=$(op read op://private/Tailscale/Bearer)

# Output both values as JSON for Terraform external data source
jq -n --arg ts "$TS_AUTH" --arg api "$API_KEY" '{"tailscale_auth_key":$ts, "api_key":$api}'
