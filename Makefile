.PHONY: all clean darwin linux helper

all: goplaying

# Build the main binary
goplaying:
	go build -o goplaying

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
