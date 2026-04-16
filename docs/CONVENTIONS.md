# Conventions for building ocw
Conventions to guide development of ocw.

## Testing
- Unit tests: `*_test.go` alongside sources (Go testing + testify)
- Integration tests: `test/integration/*_test.go` (Go testing)
- E2E tests: `test/e2e/*_test.go` (Ginko/Gomega)

`/testdata` directories inside `test/e2e` and `test/integration` can be used to provides fixtures.