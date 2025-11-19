//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HybridController implements MediaController using MediaRemote with AppleScript fallback
// This provides reliable Now Playing info for music apps on macOS
type HybridController struct {
	helperPath        string
	currentPlayer     string
	skipMediaRemote   bool // Skip MediaRemote if it failed previously for faster fallback
}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	// Find the nowplaying helper
	// Try multiple locations in order of preference
	var helperPath string

	// 1. Same directory as the binary
	helperPath = "./nowplaying"
	if _, err := os.Stat(helperPath); err == nil {
		return &HybridController{helperPath: helperPath}
	}

	// 2. helpers/nowplaying/ subdirectory
	helperPath = "./helpers/nowplaying/nowplaying"
	if _, err := os.Stat(helperPath); err == nil {
		return &HybridController{helperPath: helperPath}
	}

	// 3. Relative to executable location
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		helperPath = filepath.Join(exeDir, "nowplaying")
		if _, err := os.Stat(helperPath); err == nil {
			return &HybridController{helperPath: helperPath}
		}
	}

	// If helper not found, return controller anyway - will fallback to AppleScript only
	return &HybridController{helperPath: ""}
}

func (h *HybridController) runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// findActivePlayer checks if Music or Spotify are playing
func (h *HybridController) findActivePlayer() (string, error) {
	players := []string{"Music", "Spotify"}

	for _, player := range players {
		checkScript := fmt.Sprintf(`
			tell application "System Events"
				if exists (process "%s") then
					try
						tell application "%s"
							if player state is not stopped then
								return "true"
							end if
						end tell
					end try
				end if
				return "false"
			end tell`, player, player)

		result, err := h.runAppleScript(checkScript)
		if err == nil && result == "true" {
			return player, nil
		}
	}

	return "", errors.New("no active music player found")
}

func (h *HybridController) runHelper(args ...string) (string, error) {
	// If helper path is empty, skip MediaRemote and fall back to AppleScript
	if h.helperPath == "" {
		return "", errors.New("helper not available")
	}

	cmd := exec.Command(h.helperPath, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// Include stderr in error message for debugging
		if errOut.Len() > 0 {
			return "", fmt.Errorf("%v: %s", err, errOut.String())
		}
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (h *HybridController) GetMetadata() (title, artist, album, status string, err error) {
	// Try MediaRemote first if not previously failed (works with any app that registers Now Playing)
	if !h.skipMediaRemote {
		output, err := h.runHelper("metadata")
		if err == nil && output != "" {
			parts := strings.Split(output, "|")
			if len(parts) >= 4 {
				return strings.TrimSpace(parts[0]),
					strings.TrimSpace(parts[1]),
					strings.TrimSpace(parts[2]),
					strings.TrimSpace(parts[3]),
					nil
			}
		}
		// MediaRemote failed - skip it for future calls this session
		h.skipMediaRemote = true
	}

	// Fallback to AppleScript for Music/Spotify
	player, err := h.findActivePlayer()
	if err != nil {
		return "", "", "", "", errors.New("no song playing")
	}

	h.currentPlayer = player

	script := fmt.Sprintf(`tell application "%s"
		if player state is stopped then
			error "no song playing"
		end if
		set trackName to ""
		set trackArtist to ""
		set trackAlbum to ""
		try
			set trackName to name of current track
		end try
		try
			set trackArtist to artist of current track
		end try
		try
			set trackAlbum to album of current track
		end try
		set playerState to player state as string
		return trackName & "|" & trackArtist & "|" & trackAlbum & "|" & playerState
	end tell`, player)

	output, err = h.runAppleScript(script)
	if err != nil {
		return "", "", "", "", errors.New("no song playing")
	}

	parts := strings.Split(output, "|")
	if len(parts) < 4 {
		return "", "", "", "", errors.New("unexpected metadata format")
	}

	return strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		nil
}

func (h *HybridController) GetDuration() (int64, error) {
	// Try MediaRemote first if not previously failed
	if !h.skipMediaRemote {
		output, err := h.runHelper("duration")
		if err == nil {
			var duration int64
			n, err := fmt.Sscanf(output, "%d", &duration)
			if err == nil && n == 1 && duration > 0 {
				return duration, nil
			}
		}
	}

	// Fallback to AppleScript
	player := h.currentPlayer
	if player == "" {
		var findErr error
		player, findErr = h.findActivePlayer()
		if findErr != nil {
			// No player found - return 0 but no error (not playing is not an error state)
			return 0, nil
		}
	}

	script := fmt.Sprintf(`tell application "%s"
		try
			return duration of current track
		on error
			return 0
		end try
	end tell`, player)

	output, err = h.runAppleScript(script)
	if err != nil {
		// AppleScript execution failed - this is an error
		return 0, fmt.Errorf("failed to get duration via AppleScript: %w", err)
	}

	var duration float64
	n, err := fmt.Sscanf(output, "%f", &duration)
	if err != nil || n != 1 {
		// Parse failed - this is an error
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	// Auto-detect unit: values > 1000 are likely milliseconds, otherwise seconds
	// This handles both Spotify (milliseconds) and Apple Music (seconds) robustly
	if duration > 1000 {
		duration = duration / 1000
	}

	return int64(duration), nil
}

func (h *HybridController) GetPosition() (float64, error) {
	// Try MediaRemote first if not previously failed
	if !h.skipMediaRemote {
		output, err := h.runHelper("position")
		if err == nil {
			var position float64
			n, err := fmt.Sscanf(output, "%f", &position)
			if err == nil && n == 1 {
				return position, nil
			}
		}
	}

	// Fallback to AppleScript
	player := h.currentPlayer
	if player == "" {
		var findErr error
		player, findErr = h.findActivePlayer()
		if findErr != nil {
			// No player found - return 0 but no error (not playing is not an error state)
			return 0, nil
		}
	}

	script := fmt.Sprintf(`tell application "%s"
		try
			return player position
		on error
			return 0
		end try
	end tell`, player)

	output, err = h.runAppleScript(script)
	if err != nil {
		// AppleScript execution failed - this is an error
		return 0, fmt.Errorf("failed to get position via AppleScript: %w", err)
	}

	var position float64
	n, err := fmt.Sscanf(output, "%f", &position)
	if err != nil || n != 1 {
		// Parse failed - this is an error
		return 0, fmt.Errorf("failed to parse position: %w", err)
	}

	return position, nil
}

func (h *HybridController) Control(command string) error {
	// Try MediaRemote first if not previously failed
	if !h.skipMediaRemote {
		_, err := h.runHelper(command)
		if err == nil {
			return nil
		}
	}

	// Fallback to AppleScript
	player := h.currentPlayer
	if player == "" {
		var err error
		player, err = h.findActivePlayer()
		if err != nil {
			return err
		}
	}

	var script string
	switch command {
	case "play-pause":
		script = fmt.Sprintf(`tell application "%s" to playpause`, player)
	case "next":
		script = fmt.Sprintf(`tell application "%s" to next track`, player)
	case "previous":
		script = fmt.Sprintf(`tell application "%s" to previous track`, player)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}

	_, err = h.runAppleScript(script)
	return err
}

