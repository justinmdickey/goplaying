package main

import (
	"fmt"
)

// formatTime converts seconds to MM:SS format
func formatTime(seconds int64) string {
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// scrollText returns a scrolling window of text with smooth looping
func scrollText(text string, max int, offset int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}

	// Add padding for smooth loop (matches scrollSeparator from model.go: "  •  ")
	fullText := append(runes, []rune("  •  ")...)
	textLen := len(fullText)

	// Wrap offset around
	offset = offset % textLen

	// Build visible window
	var result []rune
	for i := 0; i < max; i++ {
		result = append(result, fullText[(offset+i)%textLen])
	}
	return string(result)
}
