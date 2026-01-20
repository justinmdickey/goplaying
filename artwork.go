package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/webp"
)

// decodeArtworkData decodes base64-encoded or raw image data into an image.Image
// This handles both base64 (from MediaRemote/playerctl) and raw bytes (from AppleScript)
func decodeArtworkData(imgData []byte) (image.Image, error) {
	// Try base64 decode first (from MediaRemote/playerctl)
	var imageData []byte
	if decoded, err := base64.StdEncoding.DecodeString(string(imgData)); err == nil {
		imageData = decoded
	} else {
		// Already raw data (from AppleScript)
		imageData = imgData
	}

	if len(imageData) == 0 {
		return nil, fmt.Errorf("empty image data")
	}

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

// Extract dominant color from image and convert to hex
// Uses a sampling approach to find vibrant, light colors suitable for dark backgrounds
func extractDominantColor(img image.Image) (string, error) {
	if img == nil {
		return "", fmt.Errorf("nil image")
	}

	bounds := img.Bounds()

	// Sample colors from the image by taking every Nth pixel
	// This is much faster than analyzing every pixel
	colorMap := make(map[uint32]int)
	sampleRate := 5 // Sample every 5th pixel

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleRate {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleRate {
			r, g, b, a := img.At(x, y).RGBA()

			// Skip transparent pixels
			if a < 32768 {
				continue
			}

			// Convert from 16-bit to 8-bit color
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			// Pack RGB into a single uint32 for counting
			rgb := (uint32(r8) << 16) | (uint32(g8) << 8) | uint32(b8)
			colorMap[rgb]++
		}
	}

	// Find colors that are light and saturated enough for readability
	type colorScore struct {
		rgb   uint32
		count int
		score float64
	}

	var candidates []colorScore

	for rgb, count := range colorMap {
		r := uint8(rgb >> 16)
		g := uint8(rgb >> 8)
		b := uint8(rgb)

		// Calculate lightness and saturation
		rf := float64(r) / 255.0
		gf := float64(g) / 255.0
		bf := float64(b) / 255.0

		max := rf
		if gf > max {
			max = gf
		}
		if bf > max {
			max = bf
		}

		min := rf
		if gf < min {
			min = gf
		}
		if bf < min {
			min = bf
		}

		lightness := (max + min) / 2.0

		var saturation float64
		if max != min {
			if lightness > 0.5 {
				saturation = (max - min) / (2.0 - max - min)
			} else {
				saturation = (max - min) / (max + min)
			}
		}

		// Skip colors that are too dark, too light (near-white), or too unsaturated
		if lightness < 0.3 || lightness > 0.85 || saturation < 0.25 {
			continue
		}

		// Score formula: balance saturation and lightness
		// Prefer vibrant colors (high saturation) that are reasonably light
		// Ideal lightness is around 0.5-0.7 (readable but not washed out)
		lightnessScore := lightness
		if lightness > 0.7 {
			// Penalize very light colors
			lightnessScore = 0.7 - (lightness - 0.7)
		}

		score := (saturation * 2.5) + (lightnessScore * 1.5) + (float64(count) / 1000.0)

		candidates = append(candidates, colorScore{rgb: rgb, count: count, score: score})
	}

	if len(candidates) == 0 {
		// Fallback: try K-means if our sampling didn't find good colors
		colors, err := prominentcolor.Kmeans(img)
		if err != nil || len(colors) == 0 {
			return "", fmt.Errorf("no suitable colors found")
		}
		c := colors[0]
		return fmt.Sprintf("#%02x%02x%02x", c.Color.R, c.Color.G, c.Color.B), nil
	}

	// Sort by score (highest first)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Use the highest scoring color
	best := candidates[0]
	r := uint8(best.rgb >> 16)
	g := uint8(best.rgb >> 8)
	b := uint8(best.rgb)

	return fmt.Sprintf("#%02x%02x%02x", r, g, b), nil
}

// Check if terminal supports Kitty graphics protocol
func supportsKittyGraphics() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")

	// Check TERM variable
	if strings.Contains(term, "kitty") || strings.Contains(term, "konsole") {
		return true
	}

	// Check TERM_PROGRAM for Ghostty and other terminals
	if termProgram == "ghostty" || termProgram == "WezTerm" {
		return true
	}

	return false
}

// Process and encode artwork for Kitty graphics protocol
// If vinyl mode is enabled, adds rotation effect metadata
func encodeArtworkForKitty(img image.Image, rotationAngle int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("nil image")
	}

	// Get config snapshot for this operation
	cfg := config.Get()

	// Resize maintaining aspect ratio - keep it reasonable for terminal display
	// We'll let Kitty handle the final sizing based on cell dimensions
	resized := resize.Resize(uint(cfg.Artwork.WidthPixels), 0, img, resize.Lanczos3)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", fmt.Errorf("failed to encode PNG: %w", err)
	}

	// Encode as base64 for Kitty protocol
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Kitty protocol needs chunking for large payloads (max 4096 bytes per chunk)
	const chunkSize = 4096
	var result strings.Builder

	// Use a fixed image ID and delete any previous image first
	const imageID = 42
	result.WriteString(fmt.Sprintf("\033_Ga=d,d=I,i=%d\033\\", imageID))

	if len(encoded) <= chunkSize {
		// Small enough to send in one go
		// Use columns (c) instead of pixels for zoom-independent sizing
		// Height is auto-calculated to maintain aspect ratio
		result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,t=d,i=%d,c=%d,C=1;%s\033\\", imageID, cfg.Artwork.WidthColumns, encoded))
	} else {
		// Need to chunk the data
		for i := 0; i < len(encoded); i += chunkSize {
			end := i + chunkSize
			if end > len(encoded) {
				end = len(encoded)
			}
			chunk := encoded[i:end]

			if i == 0 {
				// First chunk with columns-based sizing
				result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,t=d,i=%d,c=%d,C=1,m=1;%s\033\\", imageID, cfg.Artwork.WidthColumns, chunk))
			} else if end == len(encoded) {
				// Last chunk - m=0 (no more data)
				result.WriteString(fmt.Sprintf("\033_Gm=0;%s\033\\", chunk))
			} else {
				// Middle chunk - m=1 (more data coming)
				result.WriteString(fmt.Sprintf("\033_Gm=1;%s\033\\", chunk))
			}
		}
	}

	return result.String(), nil
}

// processArtwork decodes artwork data once and returns both the extracted color and Kitty-encoded string
// This is more efficient than calling extractDominantColor and encodeArtworkForKitty separately,
// as it avoids decoding the image twice
func processArtwork(artworkData []byte, extractColor bool, rotationAngle int) (color string, encoded string, err error) {
	// Decode the image once
	img, err := decodeArtworkData(artworkData)
	if err != nil {
		return "", "", err
	}

	// Extract color if requested
	if extractColor {
		if c, err := extractDominantColor(img); err == nil && c != "" {
			color = c
		}
	}

	// Encode for Kitty protocol
	if enc, err := encodeArtworkForKitty(img, rotationAngle); err == nil && enc != "" {
		encoded = enc
	}

	return color, encoded, nil
}
