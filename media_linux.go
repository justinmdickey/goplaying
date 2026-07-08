//go:build linux
// +build linux

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// artworkHTTPClient downloads remote artwork with a timeout so a hung CDN
// can't stall the fetch goroutine indefinitely.
var artworkHTTPClient = &http.Client{Timeout: 10 * time.Second}

// PlayerctlController implements MediaController using playerctl for Linux.
// GetMetadata fetches everything in a single playerctl invocation and caches
// duration/position/artUrl, so each fetch cycle spawns one process instead of four.
type PlayerctlController struct {
	mu             sync.Mutex
	cachedDuration int64
	cachedPosition float64
	cachedArtURL   string
}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	return &PlayerctlController{}
}

func (p *PlayerctlController) GetMetadata() (title, artist, album, status string, err error) {
	// Single invocation for all fields. Tab separator avoids conflicts with | in
	// metadata (e.g. album names like "Artist | Sessions"). Missing fields
	// (mpris:length on radio streams, etc.) render as empty strings.
	cmd := exec.Command("playerctl", "metadata", "--format",
		"{{title}}\t{{artist}}\t{{album}}\t{{status}}\t{{mpris:length}}\t{{position}}\t{{mpris:artUrl}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// playerctl exits non-zero when no player is running
		return "", "", "", "", ErrNothingPlaying
	}

	output := strings.TrimRight(out.String(), "\r\n")
	if strings.TrimSpace(output) == "" {
		return "", "", "", "", ErrNothingPlaying
	}

	parts := strings.Split(output, "\t")
	if len(parts) != 7 {
		return "", "", "", "", fmt.Errorf("unexpected metadata format: got %d parts, expected 7", len(parts))
	}

	// Duration and position are best-effort: some players/streams don't report
	// them, and the UI degrades gracefully (hides the progress bar).
	var duration int64
	if us, err := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64); err == nil {
		duration = us / 1e6 // microseconds → seconds
	}
	var position float64
	if us, err := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64); err == nil {
		position = us / 1e6 // microseconds → seconds
	}

	p.mu.Lock()
	p.cachedDuration = duration
	p.cachedPosition = position
	p.cachedArtURL = strings.TrimSpace(parts[6])
	p.mu.Unlock()

	return strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		nil
}

func (p *PlayerctlController) GetDuration() (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cachedDuration, nil
}

func (p *PlayerctlController) GetPosition() (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cachedPosition, nil
}

func (p *PlayerctlController) Control(command string) error {
	if err := exec.Command("playerctl", command).Run(); err != nil {
		return fmt.Errorf("playerctl %s failed: %w", command, err)
	}
	return nil
}

func (p *PlayerctlController) GetArtwork() ([]byte, error) {
	p.mu.Lock()
	artURL := p.cachedArtURL
	p.mu.Unlock()

	if artURL == "" {
		return nil, fmt.Errorf("no artwork URL")
	}

	// Handle file:// URLs (percent-decoded: paths with spaces etc.)
	if strings.HasPrefix(artURL, "file://") {
		u, err := url.Parse(artURL)
		if err != nil {
			return nil, fmt.Errorf("invalid artwork URL: %w", err)
		}
		data, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork file: %w", err)
		}
		return data, nil
	}

	// Handle http:// and https:// URLs
	if strings.HasPrefix(artURL, "http://") || strings.HasPrefix(artURL, "https://") {
		resp, err := artworkHTTPClient.Get(artURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download artwork: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("artwork download failed with status: %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork data: %w", err)
		}
		return data, nil
	}

	return nil, fmt.Errorf("unsupported artwork URL scheme: %s", artURL)
}
