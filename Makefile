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
	$(MAKE) test-unit & $(MAKE) test-integration & $(MAKE) test-e2e & wait

test-unit:
	go test ./pkg/...

test-integration:
	go test ./test/integration/...

test-e2e:
	go test ./test/e2e/...

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
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out ./pkg/... ./cmd/ocw/e2e/...
	@go tool cover -html=coverage.out -o coverage.html
	@echo ""
	@echo "Coverage Summary:"
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "HTML Report: coverage.html"

# Run all checks
check: fmt vet lint test
	@echo "All checks passed"

# Clean build artifacts
clean:
	rm -f ocw
