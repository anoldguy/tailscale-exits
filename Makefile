.PHONY: test build-lambda build-cli clean deps install-cli regions

# Default target
all: test build-cli

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
	go clean -testcache -cache

# Show available regions
regions:
	@echo "Available regions:"
	@echo "ohio, virginia, oregon, california, canada, ireland, london, paris, frankfurt, stockholm, singapore, sydney, tokyo, seoul, mumbai, saopaulo"
