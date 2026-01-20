# Testing Workflow Skill

This skill guides you through testing goplaying, including running tests, writing new tests, benchmarks, and maintaining test coverage for critical functions.

## When to Use This Skill

Use this skill when you want to:
- Run the test suite with race detection
- Write tests for new features or bug fixes
- Create benchmark tests for performance-critical code
- Test platform-specific code (macOS/Linux)
- Ensure test coverage for critical functions

## Test Infrastructure

### Existing Tests
- `config_test.go`: SafeConfig thread-safety tests
- Tests run with: `go test ./...` or `make test`
- Race detector: `go test -race ./...`

### Priority Test Targets (from TODO.md)
1. **High Priority:**
   - `text.go`: `scrollText()`, `formatTime()`
   - `artwork.go`: `decodeArtworkData()`, `extractDominantColor()`, `encodeArtworkForKitty()`, `processArtwork()`
   - `model.go`: `getCurrentPosition()`

2. **Medium Priority:**
   - Color extraction algorithms
   - Kitty protocol encoding
   - Config validation

## Workflow Steps

### 1. Running Existing Tests

**Run all tests:**
```bash
go test ./...
```

**Run with race detector:**
```bash
go test -race ./...
```

**Run with coverage:**
```bash
go test -cover ./...
```

**Run with verbose output:**
```bash
go test -v ./...
```

**Run specific test:**
```bash
go test -run TestName
```

**Run specific test file:**
```bash
go test -v ./config_test.go config.go
```

### 2. Writing New Tests

**File naming convention:**
- Test file: `<source>_test.go`
- Example: `text_test.go` for `text.go`

**Test function naming:**
- Format: `Test<FunctionName>` or `Test<FunctionName><Scenario>`
- Example: `TestScrollText`, `TestFormatTimeZeroDuration`

**Basic test structure:**
```go
package main

import (
    "testing"
)

func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := functionName(input)
    
    // Assert
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

**Table-driven tests (preferred for multiple cases):**
```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case 1", "input1", "output1"},
        {"case 2", "input2", "output2"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := functionName(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### 3. Test Categories

**Unit Tests:**
- Test individual functions in isolation
- Mock external dependencies
- Focus on edge cases and error handling
- Examples: `formatTime()`, `scrollText()`, `extractDominantColor()`

**Integration Tests:**
- Test multiple components together
- May require test fixtures (images, config files)
- Examples: `processArtwork()` with real image data

**Concurrency Tests:**
- Test thread-safe access patterns
- Use race detector: `go test -race`
- Examples: `SafeConfig` tests in `config_test.go`

**Benchmark Tests:**
- Measure performance of critical functions
- Use for performance-sensitive code
- Format: `func BenchmarkFunctionName(b *testing.B)`

### 4. Testing Best Practices

**Arrange-Act-Assert Pattern:**
1. **Arrange**: Set up test data and conditions
2. **Act**: Execute the function being tested
3. **Assert**: Verify the result matches expectations

**Edge Cases to Test:**
- Empty inputs
- Nil values
- Boundary values (0, max, negative)
- Invalid inputs
- Unicode/special characters
- Large inputs (performance)

**Error Handling:**
- Test both success and failure paths
- Verify error messages are meaningful
- Test error wrapping with `errors.Is()` / `errors.As()`

**Test Independence:**
- Each test should run independently
- Don't rely on test execution order
- Clean up resources (files, connections)

### 5. Creating Test Helpers

**Helper file location:**
- `testutil.go` or `test_helpers.go`
- Build tag to exclude from main build: `//go:build test`

**Common helpers:**
```go
// Generate test image
func generateTestImage(width, height int) image.Image

// Create temporary config file
func createTempConfig(t *testing.T, config Config) string

// Assert error types
func assertError(t *testing.T, err error, expectedMsg string)
```

### 6. Benchmark Tests

**Creating benchmarks:**
```go
func BenchmarkFunctionName(b *testing.B) {
    // Setup (not timed)
    input := "test input"
    
    // Reset timer after setup
    b.ResetTimer()
    
    // Run function b.N times
    for i := 0; i < b.N; i++ {
        functionName(input)
    }
}
```

**Running benchmarks:**
```bash
# Run all benchmarks
go test -bench=.

# Run specific benchmark
go test -bench=BenchmarkExtractColor

# With memory stats
go test -bench=. -benchmem

# Compare before/after
go test -bench=. -benchmem > before.txt
# Make changes
go test -bench=. -benchmem > after.txt
benchcmp before.txt after.txt
```

### 7. Platform-Specific Tests

**Build tags for platform tests:**
```go
//go:build darwin

package main

import "testing"

func TestMacOSSpecific(t *testing.T) {
    // macOS-only test
}
```

**Running platform-specific tests:**
```bash
# Run only Linux tests
GOOS=linux go test ./...

# Run only macOS tests
GOOS=darwin go test ./...
```

## Test Coverage Goals

### Critical Functions (Must Have Tests)
- [x] `SafeConfig.Get()` / `SafeConfig.Set()` (config_test.go)
- [ ] `scrollText()` - Text scrolling logic
- [ ] `formatTime()` - Time formatting
- [ ] `decodeArtworkData()` - Image decoding
- [ ] `extractDominantColor()` - Color extraction
- [ ] `encodeArtworkForKitty()` - Kitty protocol encoding
- [ ] `processArtwork()` - Combined artwork processing
- [ ] `getCurrentPosition()` - Position interpolation

### Nice to Have Tests
- [ ] Config validation
- [ ] Error wrapping/unwrapping
- [ ] Platform-specific media controller methods
- [ ] View rendering (harder to test)

## Quick Reference Commands

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage report
go test -cover ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run benchmarks
go test -bench=. -benchmem

# Run specific test pattern
go test -run TestScroll

# Verbose output
go test -v ./...

# Generate test template
cat > new_test.go << 'EOF'
package main

import "testing"

func TestNewFunction(t *testing.T) {
    t.Run("basic case", func(t *testing.T) {
        // Test implementation
    })
}
EOF
```

## Troubleshooting

### Tests Fail with Race Detector
- Indicates concurrent access to shared memory
- Fix with proper synchronization (mutex, channels)
- See `SafeConfig` in config.go for example

### Tests Depend on External State
- Use dependency injection
- Mock external dependencies
- Create test fixtures in testdata/

### Flaky Tests (Pass Sometimes)
- Usually timing-related
- Avoid sleep() in tests
- Use synchronization primitives
- Make tests deterministic

### Platform-Specific Test Failures
- Use build tags to separate platform tests
- Test on both macOS and Linux
- Use CI/CD for cross-platform testing

## Test File Structure Examples

### text_test.go (Simple Unit Tests)
```go
package main

import "testing"

func TestFormatTime(t *testing.T) {
    tests := []struct {
        name     string
        seconds  int
        expected string
    }{
        {"zero", 0, "0:00"},
        {"under minute", 45, "0:45"},
        {"exact minute", 60, "1:00"},
        {"over hour", 3661, "61:01"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := formatTime(tt.seconds)
            if result != tt.expected {
                t.Errorf("formatTime(%d) = %s; want %s", 
                    tt.seconds, result, tt.expected)
            }
        })
    }
}
```

### artwork_test.go (With Test Fixtures)
```go
package main

import (
    "image"
    "os"
    "testing"
)

func TestExtractDominantColor(t *testing.T) {
    // Load test image
    img := loadTestImage(t, "testdata/album.png")
    
    color, err := extractDominantColor(img)
    if err != nil {
        t.Fatalf("extractDominantColor failed: %v", err)
    }
    
    // Verify color format
    if !isValidHexColor(color) {
        t.Errorf("Invalid color format: %s", color)
    }
}

func loadTestImage(t *testing.T, path string) image.Image {
    t.Helper()
    f, err := os.Open(path)
    if err != nil {
        t.Fatalf("Failed to open test image: %v", err)
    }
    defer f.Close()
    
    img, _, err := image.Decode(f)
    if err != nil {
        t.Fatalf("Failed to decode test image: %v", err)
    }
    return img
}
```

## Testing Checklist

**Before Committing:**
- [ ] All tests pass (`go test ./...`)
- [ ] No race conditions (`go test -race ./...`)
- [ ] New functions have tests
- [ ] Edge cases covered
- [ ] Error paths tested

**For New Features:**
- [ ] Unit tests for pure functions
- [ ] Integration tests for workflows
- [ ] Benchmark if performance-critical
- [ ] Platform-specific tests if needed

**Test Quality:**
- [ ] Tests are independent
- [ ] Test names are descriptive
- [ ] Table-driven for multiple cases
- [ ] Arrange-Act-Assert pattern
- [ ] Edge cases included

## Notes

- **Test Coverage**: Aim for >70% for critical paths, 100% not required
- **Performance**: Benchmarks help track regressions
- **CI/CD**: GitHub Actions runs tests automatically
- **Test Data**: Store test fixtures in `testdata/` directory
- **Mocking**: Use interfaces for mockable dependencies

For detailed architecture and testing patterns, see [CLAUDE.md](../../CLAUDE.md).
