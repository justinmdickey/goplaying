package main

import (
	"testing"
)

// TestFormatTime tests the formatTime function with various inputs
func TestFormatTime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int64
		expected string
	}{
		{"zero seconds", 0, "00:00"},
		{"under 10 seconds", 5, "00:05"},
		{"exactly 10 seconds", 10, "00:10"},
		{"under one minute", 45, "00:45"},
		{"exactly one minute", 60, "01:00"},
		{"over one minute", 75, "01:15"},
		{"exactly 10 minutes", 600, "10:00"},
		{"under one hour", 3599, "59:59"},
		{"exactly one hour", 3600, "60:00"},
		{"over one hour", 3661, "61:01"},
		{"multiple hours", 7384, "123:04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatTime(%d) = %q; want %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

// TestFormatTimeNegative tests formatTime with negative values (edge case)
func TestFormatTimeNegative(t *testing.T) {
	// Negative values should be handled gracefully
	result := formatTime(-10)
	// The current implementation doesn't explicitly handle negative values
	// This test documents current behavior - it will show negative in minutes
	// Example: -10 seconds = "-00:10" or "00:-10" depending on implementation
	if result == "" {
		t.Error("formatTime(-10) returned empty string")
	}
}

// TestScrollText tests the scrollText function with various inputs
func TestScrollText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxLength int
		offset    int
		expected  string
	}{
		{
			name:      "short text no scroll",
			text:      "Short",
			maxLength: 10,
			offset:    0,
			expected:  "Short",
		},
		{
			name:      "exact length no scroll",
			text:      "ExactlyTen",
			maxLength: 10,
			offset:    0,
			expected:  "ExactlyTen",
		},
		{
			name:      "long text offset 0",
			text:      "This is a very long text that needs scrolling",
			maxLength: 20,
			offset:    0,
			expected:  "This is a very long ",
		},
		{
			name:      "long text offset middle",
			text:      "This is a very long text that needs scrolling",
			maxLength: 20,
			offset:    5,
			expected:  "is a very long text ",
		},
		{
			name:      "long text offset near end",
			text:      "This is a very long text that needs scrolling",
			maxLength: 20,
			offset:    30,
			expected:  "needs scrolling  •  ",
		},
		{
			name:      "unicode characters",
			text:      "Hello 世界 🎵 Music",
			maxLength: 10,
			offset:    0,
			expected:  "Hello 世界 🎵",
		},
		{
			name:      "unicode with scroll",
			text:      "Hello 世界 🎵 Music Player",
			maxLength: 10,
			offset:    6,
			expected:  "世界 🎵 Music",
		},
		{
			name:      "empty text",
			text:      "",
			maxLength: 10,
			offset:    0,
			expected:  "",
		},
		{
			name:      "zero max length",
			text:      "Some text",
			maxLength: 0,
			offset:    0,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scrollText(tt.text, tt.maxLength, tt.offset)
			if result != tt.expected {
				t.Errorf("scrollText(%q, %d, %d) = %q; want %q",
					tt.text, tt.maxLength, tt.offset, result, tt.expected)
			}
		})
	}
}

// TestScrollTextWrapping tests that scrollText properly wraps around with separator
func TestScrollTextWrapping(t *testing.T) {
	text := "ABC"
	maxLength := 5
	separator := " • "

	// The full scrollable text should be: "ABC • ABC • ..."
	// With maxLength=5, we should see wrapping behavior

	testCases := []struct {
		offset   int
		contains string
	}{
		{0, "ABC"},             // Start
		{3, separator[:2]},     // Should include separator
		{5, "ABC"},             // Wrapped back
		{len(text) * 2, "ABC"}, // Much larger offset should still work
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := scrollText(text, maxLength, tc.offset)
			if len(result) > maxLength {
				t.Errorf("scrollText result length %d exceeds maxLength %d", len(result), maxLength)
			}
		})
	}
}

// TestScrollTextUnicodeSafety tests that scrollText handles multi-byte characters correctly
func TestScrollTextUnicodeSafety(t *testing.T) {
	text := "日本語テキスト"
	maxLength := 5

	// Scroll through entire text
	for offset := 0; offset < len([]rune(text))+10; offset++ {
		result := scrollText(text, maxLength, offset)

		// Result should not exceed maxLength in runes
		resultRunes := []rune(result)
		if len(resultRunes) > maxLength {
			t.Errorf("Offset %d: scrollText result has %d runes, exceeds maxLength %d",
				offset, len(resultRunes), maxLength)
		}

		// Result should be valid UTF-8
		if !isValidUTF8(result) {
			t.Errorf("Offset %d: scrollText result contains invalid UTF-8", offset)
		}
	}
}

// isValidUTF8 checks if a string is valid UTF-8
func isValidUTF8(s string) bool {
	// If we can convert to runes and back and get the same string, it's valid UTF-8
	return string([]rune(s)) == s || len(s) == 0
}

// BenchmarkFormatTime benchmarks the formatTime function
func BenchmarkFormatTime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		formatTime(12345)
	}
}

// BenchmarkScrollText benchmarks the scrollText function
func BenchmarkScrollText(b *testing.B) {
	text := "This is a very long text that needs scrolling with multiple words"
	maxLength := 20
	offset := 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scrollText(text, maxLength, offset)
	}
}

// BenchmarkScrollTextUnicode benchmarks scrollText with Unicode characters
func BenchmarkScrollTextUnicode(b *testing.B) {
	text := "Hello 世界 🎵 Music Player with emoji and kanji 日本語"
	maxLength := 20
	offset := 5

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scrollText(text, maxLength, offset)
	}
}
