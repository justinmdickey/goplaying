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
