//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HybridController implements MediaController using MediaRemote with AppleScript fallback
// This provides reliable Now Playing info for music apps on macOS
type HybridController struct {
	helperPath      string
	currentPlayer   string
	skipMediaRemote bool    // Skip MediaRemote if it failed previously for faster fallback
	cachedDuration  int64   // Cached duration from last metadata call
	cachedPosition  float64 // Cached position from last metadata call
}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	// Find the nowplaying helper
	// Try multiple locations in order of preference
	var helperPath string

	// 1. Same directory as the binary
	helperPath = "./nowplaying"
	if _, err := os.Stat(helperPath); err == nil {
		fmt.Fprintf(os.Stderr, "Found nowplaying helper at: %s\n", helperPath)
		return &HybridController{helperPath: helperPath}
	}

	// 2. helpers/nowplaying/ subdirectory
	helperPath = "./helpers/nowplaying/nowplaying"
	if _, err := os.Stat(helperPath); err == nil {
		fmt.Fprintf(os.Stderr, "Found nowplaying helper at: %s\n", helperPath)
		return &HybridController{helperPath: helperPath}
	}

	// 3. Relative to executable location
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		helperPath = filepath.Join(exeDir, "nowplaying")
		if _, err := os.Stat(helperPath); err == nil {
			fmt.Fprintf(os.Stderr, "Found nowplaying helper at: %s\n", helperPath)
			return &HybridController{helperPath: helperPath}
		}
	}

	// If helper not found, return controller anyway - will fallback to AppleScript only
	fmt.Fprintf(os.Stderr, "Warning: nowplaying helper not found, using AppleScript only (limited to Music/Spotify)\n")
	return &HybridController{helperPath: ""}
}

func (h *HybridController) runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// Include stderr in error for better debugging
		if errOut.Len() > 0 {
			return "", fmt.Errorf("%v: %s", err, errOut.String())
		}
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

	output, scriptErr := h.runAppleScript(script)
	if scriptErr != nil {
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

	_, err := h.runAppleScript(script)
	return err
}

func (h *HybridController) GetArtwork() ([]byte, error) {
	// Try MediaRemote first if not previously failed
	if !h.skipMediaRemote {
		output, err := h.runHelper("artwork")
		if err != nil {
			// Log MediaRemote failure details
			fmt.Fprintf(os.Stderr, "MediaRemote artwork fetch failed: %v\n", err)
		} else if output == "" {
			fmt.Fprintf(os.Stderr, "MediaRemote returned empty artwork\n")
		} else {
			fmt.Fprintf(os.Stderr, "MediaRemote artwork fetch succeeded, got %d bytes (base64)\n", len(output))
			// Helper returns base64-encoded data
			return []byte(output), nil
		}
	} else {
		fmt.Fprintf(os.Stderr, "Skipping MediaRemote (previously failed), using AppleScript for artwork\n")
	}

	// Fallback to AppleScript - save artwork to temp file then read it
	player := h.currentPlayer
	if player == "" {
		var err error
		player, err = h.findActivePlayer()
		if err != nil {
			return nil, err
		}
	}

	// Create temp file for artwork
	tmpFile, err := os.CreateTemp("", "goplaying-artwork-*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // Clean up after we're done

	// Different AppleScript syntax for different players
	var script string
	if player == "Spotify" {
		// Spotify uses artwork_url instead of raw artwork data
		// We need to download it separately
		script = fmt.Sprintf(`
			tell application "Spotify"
				try
					return artwork url of current track
				on error errMsg
					error errMsg
				end try
			end tell
		`)
	} else {
		// Music.app and other apps use raw data of artwork
		script = fmt.Sprintf(`
			tell application "%s"
				try
					set artworkData to raw data of artwork 1 of current track
					set fileRef to open for access POSIX file "%s" with write permission
					write artworkData to fileRef
					close access fileRef
					return "success"
				on error errMsg
					try
						close access POSIX file "%s"
					end try
					error errMsg
				end try
			end tell
		`, player, tmpPath, tmpPath)
	}

	fmt.Fprintf(os.Stderr, "Attempting to fetch artwork via AppleScript from %s\n", player)
	output, err := h.runAppleScript(script)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AppleScript artwork fetch error: %v\n", err)
		return nil, fmt.Errorf("AppleScript error: %w", err)
	}

	// Handle Spotify's URL-based artwork
	if player == "Spotify" {
		artworkURL := strings.TrimSpace(output)
		if artworkURL == "" {
			return nil, errors.New("no artwork URL available")
		}

		fmt.Fprintf(os.Stderr, "Spotify artwork URL: %s\n", artworkURL)

		// Download the artwork from the URL
		resp, err := http.Get(artworkURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download artwork: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("artwork download failed with status: %d", resp.StatusCode)
		}

		artworkData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork data: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Downloaded %d bytes of Spotify artwork\n", len(artworkData))
		return artworkData, nil
	}

	// For Music.app and others, read from temp file
	if output != "success" {
		fmt.Fprintf(os.Stderr, "AppleScript returned unexpected output: %s\n", output)
		return nil, fmt.Errorf("unexpected AppleScript output: %s", output)
	}

	fmt.Fprintf(os.Stderr, "AppleScript artwork saved to %s\n", tmpPath)

	// Read the artwork file
	artworkData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read artwork file: %w", err)
	}

	if len(artworkData) == 0 {
		return nil, errors.New("artwork file is empty")
	}

	return artworkData, nil
}
