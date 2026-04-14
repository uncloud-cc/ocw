.PHONY: build install schema test clean lint fmt vet coverage check integration-test

# Build the CLI binary locally
build:
	go build -o ocw ./cmd/ocw
	@echo "Built ./ocw"

# Install the CLI to $GOPATH/bin (available globally)
install:
	go install ./cmd/ocw
	@echo "Installed ocw to $$(go env GOPATH)/bin"

# Generate JSON Schema from Go structs
schema:
	go run ./cmd/schema-gen > ../schema.json
	@echo "Generated schema.json"

# Run tests
test:
	go test -race ./...

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	goimports -w .
	gofmt -w .

# Run go vet
vet:
	go vet ./...

# Generate coverage report
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run all checks
check: fmt vet lint test
	@echo "All checks passed"

# Run integration tests
integration-test:
	go test -race -tags=integration ./...

# Clean build artifacts
clean:
	rm -f ocw
