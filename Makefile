.PHONY: build install schema test clean lint

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
	go test ./...

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f ocw
