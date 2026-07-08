package main

import (
	"math"
	"testing"
	"time"
)

// TestVinylTickRate verifies frame-rate-synced tick calculation and clamping
func TestVinylTickRate(t *testing.T) {
	makeCfg := func(rpm float64, frames int) Config {
		cfg := Config{}
		cfg.Artwork.VinylRPM = rpm
		cfg.Artwork.VinylFrames = frames
		return cfg
	}

	t.Run("default config (10 RPM, 90 frames)", func(t *testing.T) {
		// 10 RPM / 60 * 90 = 15 fps → ~66ms per frame
		got := vinylTickRate(makeCfg(10, 90))
		want := 66 * time.Millisecond
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("clamps to 50ms minimum", func(t *testing.T) {
		// 100 RPM / 60 * 90 = 150 fps → 6.7ms, clamped to 50ms
		if got := vinylTickRate(makeCfg(100, 90)); got != 50*time.Millisecond {
			t.Errorf("got %v, want 50ms", got)
		}
	})

	t.Run("clamps to 300ms maximum", func(t *testing.T) {
		// 1 RPM / 60 * 45 = 0.75 fps → 1333ms, clamped to 300ms
		if got := vinylTickRate(makeCfg(1, 45)); got != 300*time.Millisecond {
			t.Errorf("got %v, want 300ms", got)
		}
	})

	t.Run("zero for invalid config", func(t *testing.T) {
		if got := vinylTickRate(makeCfg(0, 90)); got != 0 {
			t.Errorf("got %v, want 0", got)
		}
	})
}

// TestVinylRotationSpeed is a regression test for the vinyl spinning at the
// wrong RPM: updateVinylRotation used a hardcoded 100ms tick duration while
// tickCmd synced ticks to the frame rate, making the record spin ~50% fast at
// default settings. Simulate one second of ticks and check frames advanced.
func TestVinylRotationSpeed(t *testing.T) {
	cfg := Config{}
	cfg.Artwork.VinylMode = true
	cfg.Artwork.VinylRPM = 10
	cfg.Artwork.VinylFrames = 90
	config.Set(cfg)

	m := &model{
		isPlaying:       true,
		vinylFrameCache: make([]string, cfg.Artwork.VinylFrames),
	}

	// Simulate 1 second of wall time at the actual tick rate
	tickRate := vinylTickRate(cfg)
	ticks := int(time.Second / tickRate)
	framesAdvanced := 0
	prev := m.vinylRotation
	for i := 0; i < ticks; i++ {
		m.updateVinylRotation(cfg)
		diff := (m.vinylRotation - prev + cfg.Artwork.VinylFrames) % cfg.Artwork.VinylFrames
		framesAdvanced += diff
		prev = m.vinylRotation
	}

	// Expected: RPM/60 * frames = 15 frames per second (allow rounding slack)
	want := cfg.Artwork.VinylRPM / 60.0 * float64(cfg.Artwork.VinylFrames)
	if math.Abs(float64(framesAdvanced)-want) > 1.5 {
		t.Errorf("advanced %d frames in 1s of ticks, want ~%.1f", framesAdvanced, want)
	}
}
