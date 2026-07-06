//go:build darwin
// +build darwin

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
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// artworkHTTPClient downloads remote artwork with a timeout so a hung CDN
// can't stall the fetch goroutine indefinitely.
var artworkHTTPClient = &http.Client{Timeout: 10 * time.Second}

// mediaRemoteRetryInterval is how long to wait before retrying MediaRemote
// after a failure, instead of permanently falling back to AppleScript.
const mediaRemoteRetryInterval = 5 * time.Minute

// HybridController implements MediaController using MediaRemote with AppleScript fallback
// This provides reliable Now Playing info for music apps on macOS
type HybridController struct {
	helperPath string

	// mu guards the mutable state below. Bubble Tea commands run in separate
	// goroutines, and a scheduled fetch can overlap a key-triggered fetch.
	mu                   sync.Mutex
	currentPlayer        string
	mediaRemoteDownUntil time.Time // Skip MediaRemote until this time after a failure
	cachedDuration       int64     // Cached duration from last metadata call
	cachedPosition       float64   // Cached position from last metadata call
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

// useMediaRemote reports whether MediaRemote should be tried right now.
func (h *HybridController) useMediaRemote() bool {
	if h.helperPath == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return time.Now().After(h.mediaRemoteDownUntil)
}

// markMediaRemoteFailed disables MediaRemote for a while so we fall back to
// AppleScript quickly, but retry later in case the failure was transient.
func (h *HybridController) markMediaRemoteFailed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mediaRemoteDownUntil = time.Now().Add(mediaRemoteRetryInterval)
}

func (h *HybridController) runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		// Include stderr in error for better debugging
		if errOut.Len() > 0 {
			return "", fmt.Errorf("osascript failed: %w (%s)", err, errOut.String())
		}
		return "", fmt.Errorf("osascript failed: %w", err)
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

	return "", ErrNothingPlaying
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
			return "", fmt.Errorf("helper execution failed: %w (%s)", err, errOut.String())
		}
		return "", fmt.Errorf("helper execution failed: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

func (h *HybridController) GetMetadata() (title, artist, album, status string, err error) {
	// Try MediaRemote first (works with any app that registers Now Playing)
	if h.useMediaRemote() {
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
		// MediaRemote failed - fall back to AppleScript, retry later
		h.markMediaRemoteFailed()
	}

	// Fallback to AppleScript for Music/Spotify
	player, err := h.findActivePlayer()
	if err != nil {
		return "", "", "", "", ErrNothingPlaying
	}

	h.mu.Lock()
	h.currentPlayer = player
	h.mu.Unlock()

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
		return "", "", "", "", fmt.Errorf("AppleScript metadata failed: %w", scriptErr)
	}

	parts := strings.Split(output, "|")
	if len(parts) < 6 {
		return "", "", "", "", fmt.Errorf("unexpected metadata format: got %d parts, expected 6", len(parts))
	}

	// Cache duration and position for GetDuration() and GetPosition() calls
	var duration float64
	fmt.Sscanf(strings.TrimSpace(parts[4]), "%f", &duration)
	// Auto-detect unit: values > 1000 are likely milliseconds, otherwise seconds
	if duration > 1000 {
		duration = duration / 1000
	}

	var position float64
	fmt.Sscanf(strings.TrimSpace(parts[5]), "%f", &position)

	h.mu.Lock()
	h.cachedDuration = int64(duration)
	h.cachedPosition = position
	h.mu.Unlock()

	return strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		nil
}

func (h *HybridController) GetDuration() (int64, error) {
	// Try MediaRemote first
	if h.useMediaRemote() {
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
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cachedDuration, nil
}

func (h *HybridController) GetPosition() (float64, error) {
	// Try MediaRemote first
	if h.useMediaRemote() {
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
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cachedPosition, nil
}

func (h *HybridController) Control(command string) error {
	// Try MediaRemote first
	if h.useMediaRemote() {
		_, err := h.runHelper(command)
		if err == nil {
			return nil
		}
	}

	// Fallback to AppleScript
	h.mu.Lock()
	player := h.currentPlayer
	h.mu.Unlock()
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
	// Try MediaRemote first - helper returns base64, decode to raw bytes here
	if h.useMediaRemote() {
		output, err := h.runHelper("artwork")
		if err == nil && output != "" {
			if raw, decErr := base64.StdEncoding.DecodeString(output); decErr == nil && len(raw) > 0 {
				return raw, nil
			}
		}
	}

	// Fallback to AppleScript - save artwork to temp file then read it
	h.mu.Lock()
	player := h.currentPlayer
	h.mu.Unlock()
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
		// Spotify uses artwork url instead of raw artwork data
		// We need to download it separately
		script = `
			tell application "Spotify"
				try
					return artwork url of current track
				on error errMsg
					error errMsg
				end try
			end tell
		`
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

	output, err := h.runAppleScript(script)
	if err != nil {
		return nil, fmt.Errorf("AppleScript error: %w", err)
	}

	// Handle Spotify's URL-based artwork
	if player == "Spotify" {
		artworkURL := strings.TrimSpace(output)
		if artworkURL == "" {
			return nil, errors.New("no artwork URL available")
		}

		// Download the artwork from the URL
		resp, err := artworkHTTPClient.Get(artworkURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download artwork: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("artwork download failed with status: %d", resp.StatusCode)
		}

		artworkData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read artwork data: %w", err)
		}

		return artworkData, nil
	}

	// For Music.app and others, read from temp file
	if output != "success" {
		return nil, fmt.Errorf("unexpected AppleScript output: %s", output)
	}

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
