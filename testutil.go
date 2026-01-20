package main

import (
	"image"
	"image/color"
	"testing"
)

// generateTestImage creates a simple test image with specified dimensions and colors
// Useful for testing artwork processing functions
func generateTestImage(width, height int, fillColor color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill image with the specified color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fillColor)
		}
	}

	return img
}

// generateGradientImage creates a gradient test image for color extraction testing
func generateGradientImage(width, height int, startColor, endColor color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		ratio := float64(y) / float64(height)
		r := uint8(float64(startColor.R)*(1-ratio) + float64(endColor.R)*ratio)
		g := uint8(float64(startColor.G)*(1-ratio) + float64(endColor.G)*ratio)
		b := uint8(float64(startColor.B)*(1-ratio) + float64(endColor.B)*ratio)

		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	return img
}

// assertError is a test helper that checks if an error occurred and fails the test if not
func assertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected error: %s, got nil", msg)
	}
}

// assertNoError is a test helper that fails the test if an error occurred
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// assertEqual is a generic test helper for comparing values
func assertEqual(t *testing.T, got, want interface{}, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// isValidHexColor checks if a string is a valid hex color (e.g., "#RRGGBB")
func isValidHexColor(color string) bool {
	if len(color) != 7 {
		return false
	}
	if color[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := color[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
