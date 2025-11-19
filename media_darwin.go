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
	skipMediaRemote   bool    // Skip MediaRemote if it failed previously for faster fallback
	cachedDuration    int64   // Cached duration from last metadata call
	cachedPosition    float64 // Cached position from last metadata call
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

	// Get all data in a single AppleScript call for performance
	// Returns: title|artist|album|status|duration|position
	script := fmt.Sprintf(`tell application "%s"
		if player state is stopped then
			error "no song playing"
		end if
		set trackName to ""
		set trackArtist to ""
		set trackAlbum to ""
		set trackDuration to 0
		set trackPosition to 0
		try
			set trackName to name of current track
		end try
		try
			set trackArtist to artist of current track
		end try
		try
			set trackAlbum to album of current track
		end try
		try
			set trackDuration to duration of current track
		end try
		try
			set trackPosition to player position
		end try
		set playerState to player state as string
		return trackName & "|" & trackArtist & "|" & trackAlbum & "|" & playerState & "|" & trackDuration & "|" & trackPosition
	end tell`, player)

	output, err = h.runAppleScript(script)
	if err != nil {
		return "", "", "", "", errors.New("no song playing")
	}

	parts := strings.Split(output, "|")
	if len(parts) < 6 {
		return "", "", "", "", errors.New("unexpected metadata format")
	}

	// Cache duration and position for GetDuration() and GetPosition() calls
	var duration float64
	fmt.Sscanf(strings.TrimSpace(parts[4]), "%f", &duration)
	// Auto-detect unit: values > 1000 are likely milliseconds, otherwise seconds
	if duration > 1000 {
		duration = duration / 1000
	}
	h.cachedDuration = int64(duration)

	fmt.Sscanf(strings.TrimSpace(parts[5]), "%f", &h.cachedPosition)

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

	// Return cached value from GetMetadata() call (batched AppleScript)
	// This avoids a second osascript invocation for better performance
	return h.cachedDuration, nil
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

	// Return cached value from GetMetadata() call (batched AppleScript)
	// This avoids a second osascript invocation for better performance
	return h.cachedPosition, nil
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

