package main

import (
	"bytes"
	"encoding/base64"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// TestDecodeArtworkData tests the decodeArtworkData function
func TestDecodeArtworkData(t *testing.T) {
	// Create a small test image
	testImg := generateTestImage(10, 10, color.RGBA{255, 0, 0, 255})

	// Encode it as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImg); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
	rawData := buf.Bytes()

	t.Run("raw bytes", func(t *testing.T) {
		img, err := decodeArtworkData(rawData)
		assertNoError(t, err)
		if img == nil {
			t.Error("Expected non-nil image")
		}
	})

	t.Run("base64 encoded", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString(rawData)
		img, err := decodeArtworkData([]byte(encoded))
		assertNoError(t, err)
		if img == nil {
			t.Error("Expected non-nil image")
		}
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := decodeArtworkData([]byte{})
		if err == nil {
			t.Error("Expected error for empty data")
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		_, err := decodeArtworkData([]byte("not an image"))
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})
}

// TestExtractDominantColor tests the extractDominantColor function
func TestExtractDominantColor(t *testing.T) {
	t.Run("solid color image", func(t *testing.T) {
		// Red image
		img := generateTestImage(100, 100, color.RGBA{255, 0, 0, 255})
		color, err := extractDominantColor(img)
		assertNoError(t, err)

		if !isValidHexColor(color) {
			t.Errorf("Invalid hex color format: %s", color)
		}
	})

	t.Run("gradient image", func(t *testing.T) {
		// Gradient from blue to green
		img := generateGradientImage(100, 100,
			color.RGBA{0, 0, 255, 255},
			color.RGBA{0, 255, 0, 255})

		color, err := extractDominantColor(img)
		assertNoError(t, err)

		if !isValidHexColor(color) {
			t.Errorf("Invalid hex color format: %s", color)
		}
	})

	t.Run("small image", func(t *testing.T) {
		// Very small image (edge case)
		img := generateTestImage(5, 5, color.RGBA{128, 128, 255, 255})
		color, err := extractDominantColor(img)
		assertNoError(t, err)

		if !isValidHexColor(color) {
			t.Errorf("Invalid hex color format: %s", color)
		}
	})

	t.Run("nil image", func(t *testing.T) {
		_, err := extractDominantColor(nil)
		if err == nil {
			t.Error("Expected error for nil image")
		}
	})

	t.Run("transparent image", func(t *testing.T) {
		// Fully transparent image
		img := generateTestImage(50, 50, color.RGBA{255, 0, 0, 0})
		_, err := extractDominantColor(img)
		// Should handle transparent images gracefully
		// (might return error or fallback color)
		if err != nil {
			t.Logf("Transparent image returned error (expected): %v", err)
		}
	})
}

// TestEncodeArtworkForKitty tests the encodeArtworkForKitty function
func TestEncodeArtworkForKitty(t *testing.T) {
	// Create a small test config
	testConfig := Config{}
	testConfig.Artwork.WidthPixels = 100
	testConfig.Artwork.WidthColumns = 10
	config.Set(testConfig)

	t.Run("valid image", func(t *testing.T) {
		img := generateTestImage(50, 50, color.RGBA{100, 150, 200, 255})
		encoded, err := encodeArtworkForKitty(img)
		assertNoError(t, err)

		if encoded == "" {
			t.Error("Expected non-empty encoded string")
		}

		// Should contain Kitty protocol escape sequences
		if !bytes.Contains([]byte(encoded), []byte("\033_G")) {
			t.Error("Encoded string doesn't contain Kitty protocol escape sequence")
		}
	})

	t.Run("nil image", func(t *testing.T) {
		_, err := encodeArtworkForKitty(nil)
		if err == nil {
			t.Error("Expected error for nil image")
		}
	})

	t.Run("large image chunks", func(t *testing.T) {
		// Large image that should trigger chunking
		img := generateTestImage(800, 800, color.RGBA{100, 150, 200, 255})
		encoded, err := encodeArtworkForKitty(img)
		assertNoError(t, err)

		if encoded == "" {
			t.Error("Expected non-empty encoded string")
		}

		// Should contain chunking markers (m=1 or m=0)
		hasChunking := bytes.Contains([]byte(encoded), []byte("m=1")) ||
			bytes.Contains([]byte(encoded), []byte("m=0"))
		if !hasChunking {
			t.Log("Large image might not have triggered chunking (depends on PNG compression)")
		}
	})
}

// TestProcessArtwork tests the combined processArtwork function
func TestProcessArtwork(t *testing.T) {
	// Setup test config
	testConfig := Config{}
	testConfig.Artwork.WidthPixels = 100
	testConfig.Artwork.WidthColumns = 10
	config.Set(testConfig)

	// Create test image and encode as PNG
	testImg := generateTestImage(50, 50, color.RGBA{100, 150, 200, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImg); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
	imageData := buf.Bytes()

	t.Run("with color extraction", func(t *testing.T) {
		color, encoded, err := processArtwork(imageData, true)
		assertNoError(t, err)

		if !isValidHexColor(color) {
			t.Errorf("Invalid hex color: %s", color)
		}

		if encoded == "" {
			t.Error("Expected non-empty encoded string")
		}
	})

	t.Run("without color extraction", func(t *testing.T) {
		color, encoded, err := processArtwork(imageData, false)
		assertNoError(t, err)

		if color != "" {
			t.Error("Expected empty color when extractColor=false")
		}

		if encoded == "" {
			t.Error("Expected non-empty encoded string")
		}
	})

	t.Run("base64 input", func(t *testing.T) {
		base64Data := base64.StdEncoding.EncodeToString(imageData)
		color, encoded, err := processArtwork([]byte(base64Data), true)
		assertNoError(t, err)

		if !isValidHexColor(color) {
			t.Errorf("Invalid hex color: %s", color)
		}

		if encoded == "" {
			t.Error("Expected non-empty encoded string")
		}
	})

	t.Run("invalid data", func(t *testing.T) {
		_, _, err := processArtwork([]byte("not an image"), true)
		if err == nil {
			t.Error("Expected error for invalid data")
		}
	})

	t.Run("empty data", func(t *testing.T) {
		_, _, err := processArtwork([]byte{}, true)
		if err == nil {
			t.Error("Expected error for empty data")
		}
	})
}

// TestSupportsKittyGraphics tests terminal detection
func TestSupportsKittyGraphics(t *testing.T) {
	// Save original env vars
	origTerm := os.Getenv("TERM")
	origTermProgram := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("TERM", origTerm)
		os.Setenv("TERM_PROGRAM", origTermProgram)
	}()

	tests := []struct {
		name          string
		term          string
		termProgram   string
		shouldSupport bool
	}{
		{"kitty terminal", "xterm-kitty", "", true},
		{"kitty in name", "kitty", "", true},
		{"konsole", "konsole", "", true},
		{"ghostty", "", "ghostty", true},
		{"wezterm", "", "WezTerm", true},
		{"xterm", "xterm-256color", "", false},
		{"tmux", "tmux-256color", "", false},
		{"unknown", "unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TERM", tt.term)
			os.Setenv("TERM_PROGRAM", tt.termProgram)

			result := supportsKittyGraphics()
			if result != tt.shouldSupport {
				t.Errorf("Expected %v, got %v for TERM=%s, TERM_PROGRAM=%s",
					tt.shouldSupport, result, tt.term, tt.termProgram)
			}
		})
	}
}

// BenchmarkExtractDominantColor benchmarks color extraction
func BenchmarkExtractDominantColor(b *testing.B) {
	img := generateTestImage(300, 300, color.RGBA{100, 150, 200, 255})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractDominantColor(img)
	}
}

// BenchmarkEncodeArtworkForKitty benchmarks Kitty encoding
func BenchmarkEncodeArtworkForKitty(b *testing.B) {
	// Setup config
	testConfig := Config{}
	testConfig.Artwork.WidthPixels = 300
	testConfig.Artwork.WidthColumns = 13
	config.Set(testConfig)

	img := generateTestImage(300, 300, color.RGBA{100, 150, 200, 255})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeArtworkForKitty(img)
	}
}

// BenchmarkProcessArtwork benchmarks the combined operation
func BenchmarkProcessArtwork(b *testing.B) {
	// Setup config
	testConfig := Config{}
	testConfig.Artwork.WidthPixels = 300
	testConfig.Artwork.WidthColumns = 13
	config.Set(testConfig)

	// Create test image data
	testImg := generateTestImage(300, 300, color.RGBA{100, 150, 200, 255})
	var buf bytes.Buffer
	png.Encode(&buf, testImg)
	imageData := buf.Bytes()

	b.Run("with color extraction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processArtwork(imageData, true)
		}
	})

	b.Run("without color extraction", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processArtwork(imageData, false)
		}
	})
}

// BenchmarkDecodeArtworkData benchmarks image decoding
func BenchmarkDecodeArtworkData(b *testing.B) {
	// Create test image data
	testImg := generateTestImage(300, 300, color.RGBA{100, 150, 200, 255})
	var buf bytes.Buffer
	png.Encode(&buf, testImg)
	imageData := buf.Bytes()

	b.Run("raw bytes", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decodeArtworkData(imageData)
		}
	})

	b.Run("base64 encoded", func(b *testing.B) {
		encoded := base64.StdEncoding.EncodeToString(imageData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			decodeArtworkData([]byte(encoded))
		}
	})
}
