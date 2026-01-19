.PHONY: all clean darwin linux helper fmt lint test

all: goplaying

# Build the main binary
goplaying:
	go build -o goplaying

# Format code
fmt:
	gofmt -s -w .
	goimports -w -local goplaying .

# Run linters
lint:
	golangci-lint run

# Run tests
test:
	go test -v ./...

# Build macOS helper (only on Darwin)
helper:
	cd helpers/nowplaying && $(MAKE)

# Darwin-specific build that includes helper
darwin: helper goplaying

# Linux build (no helper needed)
linux: goplaying

clean:
	rm -f goplaying
	test -d helpers/nowplaying && cd helpers/nowplaying && $(MAKE) clean || true
