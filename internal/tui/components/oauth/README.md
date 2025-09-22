# Qwen3 Coder OAuth Test Suite

This directory contains a comprehensive test suite for the Qwen3 Coder OAuth functionality in the Crush application.

## Test Organization

The tests are organized into multiple files based on functionality:

1. **qwen3_test.go** - Unit tests for the Qwen3OAuthComponent
2. **dialog_test.go** - Unit tests for the generic OAuth dialog component
3. **integration_test.go** - Integration tests for complete OAuth flows
4. **pkce_test.go** - Security tests for PKCE (Proof Key for Code Exchange) implementation
5. **ui_test.go** - UI component and message passing tests
6. **config_test.go** - Configuration and token storage tests
7. **error_test.go** - Error handling and edge case tests
8. **oauth_test.go** - Test suite placeholder

## Coverage Areas

### 1. OAuth Flow Initiation (Device Code Flow with PKCE)
- Code verifier generation
- Code challenge generation
- Device authorization request
- Browser opening functionality

### 2. Token Storage and Management
- OAuth token storage in config
- Refresh token handling
- Token expiration management
- Credential persistence

### 3. Model Fetching After Authentication
- Models API integration
- Model list parsing and storage
- Model configuration updates

### 4. UI Integration and Message Passing
- State management across OAuth flow
- Spinner animations and UI updates
- Error message display
- Success state handling

### 5. Error Handling and Edge Cases
- Network error handling
- Timeout scenarios
- Invalid response handling
- HTTP error status codes
- OAuth-specific error conditions
- Rate limiting and slow down handling

### 6. Security Considerations (PKCE Implementation)
- Cryptographically secure code verifier generation
- Proper code challenge computation
- Verifier length and character set validation
- Challenge verification

## Running the Tests

To run all OAuth tests:

```bash
go test ./internal/tui/components/oauth/...
```

To run with coverage:

```bash
go test -cover ./internal/tui/components/oauth/...
```

To run with coverage and generate a coverage profile:

```bash
go test -coverprofile=coverage.out ./internal/tui/components/oauth/...
go tool cover -html=coverage.out
```

## Test Dependencies

The tests use:
- `httptest` for mocking HTTP servers
- `testify` for assertions
- Mock implementations for external dependencies

## Mocking Strategy

1. **HTTP Clients**: Using `httptest.Server` to create mock OAuth servers
2. **File I/O**: Using `t.TempDir()` for temporary configuration directories
3. **Time-dependent operations**: Using controlled time values in tests
4. **External libraries**: Using interface-based mocking where possible

## Parallel Execution

All tests are designed to run in parallel without conflicts:
- Each test uses its own temporary directories
- Mock servers are created per test
- No shared global state between tests
- Time-based tests use controlled values rather than real time

## Code Coverage Goals

The test suite targets >90% code coverage across all OAuth functionality:
- Unit tests cover individual functions and methods
- Integration tests cover complete workflows
- Edge case tests cover error conditions
- Security tests verify PKCE implementation correctness