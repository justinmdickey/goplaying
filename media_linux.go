//go:build linux
// +build linux

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// PlayerctlController implements MediaController using playerctl for Linux
type PlayerctlController struct{}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	return &PlayerctlController{}
}

func (p *PlayerctlController) GetMetadata() (title, artist, album, status string, err error) {
	// Use tab separator to avoid conflicts with | in metadata (e.g. album names like "Artist | Sessions")
	cmd := exec.Command("playerctl", "metadata", "--format", "{{title}}\t{{artist}}\t{{album}}\t{{status}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// When no player is running or nothing is playing, return friendly error
		// that matches the "nothing playing" check in view.go
		return "", "", "", "", errors.New("no song playing")
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return "", "", "", "", errors.New("no song playing")
	}

	parts := strings.Split(output, "\t")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("unexpected metadata format: got %d parts, expected 4", len(parts))
	}

	return strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		nil
}

func (p *PlayerctlController) GetDuration() (int64, error) {
	cmd := exec.Command("playerctl", "metadata", "mpris:length")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, errors.New("no song playing")
	}

	var duration int64
	n, err := fmt.Sscanf(strings.TrimSpace(out.String()), "%d", &duration)
	if n != 1 || err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}
	// Convert from microseconds to seconds
	duration = duration / 1e6

	return duration, nil
}

func (p *PlayerctlController) GetPosition() (float64, error) {
	cmd := exec.Command("playerctl", "position")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, errors.New("no song playing")
	}

	var position float64
	n, err := fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &position)
	if n != 1 || err != nil {
		return 0, fmt.Errorf("failed to parse position: %w", err)
	}

	return position, nil
}

func (p *PlayerctlController) Control(command string) error {
	if err := exec.Command("playerctl", command).Run(); err != nil {
		return fmt.Errorf("playerctl %s failed: %w", command, err)
	}
	return nil
}

func (p *PlayerctlController) GetArtwork() ([]byte, error) {
	// Get artwork URL from playerctl
	cmd := exec.Command("playerctl", "metadata", "mpris:artUrl")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, errors.New("no artwork available")
	}

	artUrl := strings.TrimSpace(out.String())
	if artUrl == "" {
		return nil, errors.New("no artwork URL")
	}

	// Handle file:// URLs
	if strings.HasPrefix(artUrl, "file://") {
		filePath := strings.TrimPrefix(artUrl, "file://")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork file: %w", err)
		}
		// Return base64-encoded data to match macOS behavior
		encoded := base64.StdEncoding.EncodeToString(data)
		return []byte(encoded), nil
	}

	// Handle http:// and https:// URLs
	if strings.HasPrefix(artUrl, "http://") || strings.HasPrefix(artUrl, "https://") {
		resp, err := http.Get(artUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to download artwork: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("artwork download failed with status: %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork data: %w", err)
		}

		// Return base64-encoded data to match macOS behavior
		encoded := base64.StdEncoding.EncodeToString(data)
		return []byte(encoded), nil
	}

	return nil, fmt.Errorf("unsupported artwork URL scheme: %s", artUrl)
}
