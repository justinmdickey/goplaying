package main

import (
	"sync"
	"testing"
)

// TestSafeConfigConcurrency tests that SafeConfig can be safely accessed from multiple goroutines
func TestSafeConfigConcurrency(t *testing.T) {
	sc := &SafeConfig{}

	// Initial config
	initialCfg := Config{}
	initialCfg.UI.Color = "1"
	initialCfg.UI.MaxWidth = 45
	initialCfg.Artwork.Enabled = true
	sc.Set(initialCfg)

	var wg sync.WaitGroup

	// Start 10 writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cfg := Config{}
				cfg.UI.Color = string(rune('0' + (id % 10)))
				cfg.UI.MaxWidth = 40 + id
				cfg.Artwork.Enabled = (j % 2) == 0
				sc.Set(cfg)
			}
		}(i)
	}

	// Start 10 readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cfg := sc.Get()
				// Just access the fields to ensure no panic
				_ = cfg.UI.Color
				_ = cfg.UI.MaxWidth
				_ = cfg.Artwork.Enabled
			}
		}()
	}

	wg.Wait()

	// If we got here without panic or data race, test passes
}

// TestSafeConfigGetReturnsCopy tests that Get() returns a copy, not a reference
func TestSafeConfigGetReturnsCopy(t *testing.T) {
	sc := &SafeConfig{}

	cfg1 := Config{}
	cfg1.UI.Color = "1"
	cfg1.UI.MaxWidth = 45
	sc.Set(cfg1)

	// Get a copy
	retrieved1 := sc.Get()

	// Modify the local copy
	retrieved1.UI.Color = "2"
	retrieved1.UI.MaxWidth = 100

	// Get another copy - should have original values
	retrieved2 := sc.Get()

	if retrieved2.UI.Color != "1" {
		t.Errorf("Expected color '1', got '%s'", retrieved2.UI.Color)
	}

	if retrieved2.UI.MaxWidth != 45 {
		t.Errorf("Expected max_width 45, got %d", retrieved2.UI.MaxWidth)
	}
}

// TestIsValidColor tests the color validation function
func TestIsValidColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		valid bool
	}{
		// ANSI codes
		{"ansi single digit", "1", true},
		{"ansi double digit", "15", true},
		{"ansi triple digit", "255", true},
		{"ansi zero", "0", true},
		{"ansi invalid", "256", false}, // not validated as out of range, just format
		{"ansi with letter", "1a", false},

		// Hex colors
		{"hex 6 digits", "#FF5733", true},
		{"hex lowercase", "#ff5733", true},
		{"hex 3 digits", "#F00", true},
		{"hex mixed case", "#Ff5733", true},
		{"hex no hash", "FF5733", false},
		{"hex invalid char", "#GG5733", false},
		{"hex wrong length", "#FF57", false},

		// Edge cases
		{"empty", "", false},
		{"just hash", "#", false},
		{"spaces", " 1 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidColor(tt.color)
			if result != tt.valid {
				t.Errorf("isValidColor(%q) = %v; want %v", tt.color, result, tt.valid)
			}
		})
	}
}

// TestValidateConfig tests configuration validation
func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "2"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Enabled = true
		cfg.Artwork.Padding = 15
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Artwork.VinylFrames = 90
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) > 0 {
			t.Errorf("Expected no errors for valid config, got %d: %v", len(errors), errors)
		}
	})

	t.Run("invalid max_width too small", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.MaxWidth = 10
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "manual"
		cfg.Artwork.Padding = 5
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for max_width < 20")
		}
	})

	t.Run("invalid color_mode", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "invalid"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = 15
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for invalid color_mode")
		}
	})

	t.Run("invalid color", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "invalid"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = 15
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for invalid color")
		}
	})

	t.Run("padding exceeds max_width", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = 50
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for padding >= max_width")
		}
	})

	t.Run("negative padding", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = -5
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for negative padding")
		}
	})

	t.Run("invalid width_pixels", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = 15
		cfg.Artwork.WidthPixels = 0
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 100
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for width_pixels = 0")
		}
	})

	t.Run("ui_refresh_ms too fast", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "1"
		cfg.UI.ColorMode = "manual"
		cfg.UI.MaxWidth = 45
		cfg.Artwork.Padding = 15
		cfg.Artwork.WidthPixels = 300
		cfg.Artwork.WidthColumns = 13
		cfg.Artwork.VinylRPM = 33.33
		cfg.Text.MaxLengthWithArt = 22
		cfg.Text.MaxLengthNoArt = 36
		cfg.Timing.UIRefreshMs = 5
		cfg.Timing.DataFetchMs = 1000

		errors := validateConfig(&cfg)
		if len(errors) == 0 {
			t.Error("Expected error for ui_refresh_ms < 10")
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		cfg := Config{}
		cfg.UI.Color = "invalid"
		cfg.UI.ColorMode = "wrong"
		cfg.UI.MaxWidth = 10
		cfg.Artwork.Padding = -5
		cfg.Artwork.WidthPixels = 0
		cfg.Artwork.WidthColumns = 0
		cfg.Text.MaxLengthWithArt = 0
		cfg.Text.MaxLengthNoArt = 0
		cfg.Timing.UIRefreshMs = 0
		cfg.Timing.DataFetchMs = 0

		errors := validateConfig(&cfg)
		if len(errors) < 5 {
			t.Errorf("Expected multiple errors, got %d", len(errors))
		}
	})
}

// TestApplyDefaultsForInvalidFields tests default value application
func TestApplyDefaultsForInvalidFields(t *testing.T) {
	cfg := Config{}
	cfg.UI.MaxWidth = 10
	cfg.UI.Color = "invalid"
	cfg.UI.ColorMode = "wrong"
	cfg.Artwork.Padding = -5
	cfg.Artwork.WidthPixels = 0
	cfg.Artwork.WidthColumns = 0
	cfg.Text.MaxLengthWithArt = 0
	cfg.Text.MaxLengthNoArt = 0
	cfg.Timing.UIRefreshMs = 0
	cfg.Timing.DataFetchMs = 0

	errors := validateConfig(&cfg)
	applyDefaultsForInvalidFields(&cfg, errors)

	// Check that defaults were applied
	if cfg.UI.MaxWidth != 45 {
		t.Errorf("Expected max_width default 45, got %d", cfg.UI.MaxWidth)
	}
	if cfg.UI.Color != "2" {
		t.Errorf("Expected color default '2', got '%s'", cfg.UI.Color)
	}
	if cfg.UI.ColorMode != "auto" {
		t.Errorf("Expected color_mode default 'auto', got '%s'", cfg.UI.ColorMode)
	}
	if cfg.Artwork.Padding != 16 {
		t.Errorf("Expected padding default 16, got %d", cfg.Artwork.Padding)
	}
	if cfg.Artwork.WidthPixels != 300 {
		t.Errorf("Expected width_pixels default 300, got %d", cfg.Artwork.WidthPixels)
	}
	if cfg.Timing.UIRefreshMs != 100 {
		t.Errorf("Expected ui_refresh_ms default 100, got %d", cfg.Timing.UIRefreshMs)
	}

	// Validate that corrected config is now valid
	newErrors := validateConfig(&cfg)
	if len(newErrors) > 0 {
		t.Errorf("Expected no errors after applying defaults, got %d: %v", len(newErrors), newErrors)
	}
}

func TestPrintConfigWarnings(t *testing.T) {
	// Test that warnings are formatted correctly
	errors := []error{
		configError{field: "ui.max_width", message: "must be at least 20 (got 5)"},
		configError{field: "ui.color", message: "invalid color format 'notacolor'"},
	}

	// Just verify it doesn't panic - output goes to stderr
	// In real usage, users will see these warnings when app starts
	printConfigWarnings(errors)
}

func TestValidationIntegration(t *testing.T) {
	// Test the full validation flow: validate -> apply defaults -> re-validate
	cfg := Config{}
	cfg.UI.Color = "999" // Invalid ANSI code
	cfg.UI.ColorMode = "invalid"
	cfg.UI.MaxWidth = 10
	cfg.Artwork.Padding = -5
	cfg.Artwork.WidthPixels = 0
	cfg.Artwork.WidthColumns = 13
	cfg.Artwork.VinylRPM = 33.33
	cfg.Text.MaxLengthWithArt = 0
	cfg.Text.MaxLengthNoArt = 300
	cfg.Timing.UIRefreshMs = 5
	cfg.Timing.DataFetchMs = 70000

	// Step 1: Validate and collect errors
	errors := validateConfig(&cfg)
	if len(errors) == 0 {
		t.Fatal("Expected multiple validation errors")
	}

	// We should have errors for all 9 invalid fields
	expectedMinErrors := 9
	if len(errors) < expectedMinErrors {
		t.Errorf("Expected at least %d errors, got %d", expectedMinErrors, len(errors))
	}

	// Step 2: Apply defaults for invalid fields
	applyDefaultsForInvalidFields(&cfg, errors)

	// Step 3: Re-validate - should have no errors now
	newErrors := validateConfig(&cfg)
	if len(newErrors) > 0 {
		t.Errorf("Expected no errors after applying defaults, got %d: %v", len(newErrors), newErrors)
	}

	// Step 4: Verify specific defaults were applied
	if cfg.UI.Color != "2" {
		t.Errorf("Expected color default '2', got '%s'", cfg.UI.Color)
	}
	if cfg.UI.ColorMode != "auto" {
		t.Errorf("Expected color_mode default 'auto', got '%s'", cfg.UI.ColorMode)
	}
	if cfg.UI.MaxWidth != 45 {
		t.Errorf("Expected max_width default 45, got %d", cfg.UI.MaxWidth)
	}
	if cfg.Artwork.Padding != 16 {
		t.Errorf("Expected padding default 16, got %d", cfg.Artwork.Padding)
	}
	if cfg.Artwork.WidthPixels != 300 {
		t.Errorf("Expected width_pixels default 300, got %d", cfg.Artwork.WidthPixels)
	}
	if cfg.Text.MaxLengthWithArt != 22 {
		t.Errorf("Expected max_length_with_art default 22, got %d", cfg.Text.MaxLengthWithArt)
	}
	if cfg.Text.MaxLengthNoArt != 36 {
		t.Errorf("Expected max_length_no_art default 36, got %d", cfg.Text.MaxLengthNoArt)
	}
	if cfg.Timing.UIRefreshMs != 100 {
		t.Errorf("Expected ui_refresh_ms default 100, got %d", cfg.Timing.UIRefreshMs)
	}
	if cfg.Timing.DataFetchMs != 1000 {
		t.Errorf("Expected data_fetch_ms default 1000, got %d", cfg.Timing.DataFetchMs)
	}
}
