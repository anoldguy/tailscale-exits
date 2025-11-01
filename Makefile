.PHONY: test build-lambda build-cli clean deps deploy tofu-init tofu-init-upgrade tofu-plan tofu-apply tofu-destroy

# Default target
all: test build-lambda build-cli

# Download dependencies
deps:
	go mod download
	go mod tidy

# Run tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Build Lambda function for AWS ARM64
build-lambda:
	cd lambda && GOOS=linux GOARCH=arm64 go build -o bootstrap .

# Build CLI tool for local use
build-cli:
	mkdir -p bin
	cd cmd/tse && go build -o ../../bin/tse .

# Install CLI tool to local bin
install-cli: build-cli
	cp bin/tse /usr/local/bin/tse

# Clean build artifacts
clean:
	rm -f lambda/bootstrap
	rm -f lambda/main
	rm -f cmd/tse/tse
	rm -rf bin/
	rm -f lambda/*.zip
	rm -f deployments/opentofu/*.zip
	rm -f deployments/opentofu/*.tfplan

# Package Lambda for deployment
package-lambda: build-lambda
	cd lambda && zip lambda-deployment.zip bootstrap

# Show available regions
regions:
	@echo "Available regions:"
	@go run -c 'import "github.com/anoldguy/tse/shared/regions"; fmt.Println(regions.GetAvailableRegions())'

# OpenTofu/Terraform deployment targets

# Initialize OpenTofu (run once before first deployment)
tofu-init:
	cd deployments/opentofu && tofu init

# Upgrade OpenTofu providers (manual step when you want latest providers)
tofu-init-upgrade:
	cd deployments/opentofu && tofu init -upgrade

# Plan infrastructure changes
tofu-plan: package-lambda
	cd deployments/opentofu && tofu plan

# Apply infrastructure changes
tofu-apply: package-lambda
	cd deployments/opentofu && tofu apply

# Deploy everything (clean build + apply infrastructure)
deploy: clean tofu-init tofu-apply
	@echo ""
	@echo "✓ Deployment complete!"
	@echo ""
	@echo "Set your environment variables:"
	@echo "  export TSE_LAMBDA_URL=\$$(cd deployments/opentofu && tofu output -raw lambda_function_url)"
	@echo "  export TSE_AUTH_TOKEN=\$$(cd deployments/opentofu && tofu output -raw auth_token)"

# Destroy all infrastructure
tofu-destroy:
	@echo "WARNING: This will destroy all TSE infrastructure in AWS"
	@cd deployments/opentofu && tofu destroy