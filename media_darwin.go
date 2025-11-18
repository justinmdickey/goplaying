//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// AppleScriptController implements MediaController using AppleScript for macOS
// It supports multiple music players: Apple Music, Spotify, VLC, and others
type AppleScriptController struct {
	currentPlayer string // Cache the current active player
}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	return &AppleScriptController{}
}

func (a *AppleScriptController) runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// findActivePlayer checks multiple music applications to find one that's playing
func (a *AppleScriptController) findActivePlayer() (string, error) {
	// List of supported players in priority order
	// Music and Spotify first, then browsers for YouTube/web audio
	players := []string{"Music", "Spotify", "Safari", "Chrome"}

	for _, player := range players {
		// Check if the application is running and playing
		var checkScript string

		switch player {
		case "Music", "Spotify":
			checkScript = fmt.Sprintf(`
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
		case "Safari":
			// Check if Safari has an active tab playing audio
			checkScript = `
				tell application "System Events"
					if exists (process "Safari") then
						tell application "Safari"
							try
								repeat with w in windows
									repeat with t in tabs of w
										if name of t contains "▶" or name of t contains "YouTube" or name of t contains "Spotify" then
											return "true"
										end if
									end repeat
								end repeat
							end try
						end tell
					end if
					return "false"
				end tell`
		case "Chrome", "Google Chrome":
			// Check if Chrome has an active tab playing audio
			checkScript = `
				tell application "System Events"
					if exists (process "Google Chrome") then
						tell application "Google Chrome"
							try
								repeat with w in windows
									repeat with t in tabs of w
										if title of t contains "▶" or title of t contains "YouTube" or title of t contains "Spotify" then
											return "true"
										end if
									end repeat
								end repeat
							end try
						end tell
					end if
					return "false"
				end tell`
		}

		result, err := a.runAppleScript(checkScript)
		if err == nil && result == "true" {
			return player, nil
		}
	}

	return "", errors.New("no active music player found")
}

func (a *AppleScriptController) GetMetadata() (title, artist, album, status string, err error) {
	// Find which player is currently active
	player, err := a.findActivePlayer()
	if err != nil {
		return "", "", "", "", err
	}

	a.currentPlayer = player

	var script string

	switch player {
	case "Music":
		// Apple Music / iTunes - handle missing metadata gracefully for Radio
		script = `tell application "Music"
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
		end tell`
	case "Spotify":
		// Spotify
		script = `tell application "Spotify"
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
		end tell`
	case "Safari":
		// Get info from Safari tab
		script = `tell application "Safari"
			try
				repeat with w in windows
					repeat with t in tabs of w
						set tabTitle to name of t
						if tabTitle contains "▶" or tabTitle contains "YouTube" or tabTitle contains "Spotify" then
							-- Extract title from tab name (usually "Title - YouTube" or similar)
							set trackName to tabTitle
							return trackName & "|||playing"
						end if
					end repeat
				end repeat
			end try
			error "no song playing"
		end tell`
	case "Chrome":
		// Get info from Chrome tab
		script = `tell application "Google Chrome"
			try
				repeat with w in windows
					repeat with t in tabs of w
						set tabTitle to title of t
						if tabTitle contains "▶" or tabTitle contains "YouTube" or tabTitle contains "Spotify" then
							-- Extract title from tab name
							set trackName to tabTitle
							return trackName & "|||playing"
						end if
					end repeat
				end repeat
			end try
			error "no song playing"
		end tell`
	}

	output, err := a.runAppleScript(script)
	if err != nil {
		return "", "", "", "", errors.New("no song playing")
	}

	if output == "" {
		return "", "", "", "", errors.New("no song playing")
	}

	parts := strings.Split(output, "|")
	// Handle both full metadata (4 parts) and partial metadata
	if len(parts) < 4 {
		return "", "", "", "", errors.New("unexpected metadata format")
	}

	// Clean up browser titles (remove " - YouTube" etc.)
	trackTitle := strings.TrimSpace(parts[0])
	if player == "Safari" || player == "Chrome" {
		// Remove common suffixes
		trackTitle = strings.TrimSuffix(trackTitle, " - YouTube")
		trackTitle = strings.TrimSuffix(trackTitle, " - YouTube Music")
		trackTitle = strings.TrimPrefix(trackTitle, "▶ ")
		trackTitle = strings.TrimPrefix(trackTitle, "▶")
		trackTitle = strings.TrimSpace(trackTitle)
	}

	return trackTitle,
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		nil
}

func (a *AppleScriptController) GetDuration() (int64, error) {
	// Use cached player from GetMetadata, or find active player
	player := a.currentPlayer
	if player == "" {
		var err error
		player, err = a.findActivePlayer()
		if err != nil {
			return 0, err
		}
	}

	// Browsers don't reliably expose duration via AppleScript
	if player == "Safari" || player == "Chrome" {
		return 0, errors.New("duration not available for browser")
	}

	var script string

	switch player {
	case "Music":
		script = `tell application "Music"
			try
				return duration of current track
			on error
				return 0
			end try
		end tell`
	case "Spotify":
		script = `tell application "Spotify"
			try
				return duration of current track
			on error
				return 0
			end try
		end tell`
	default:
		return 0, errors.New("duration not available")
	}

	output, err := a.runAppleScript(script)
	if err != nil {
		return 0, errors.New("can't get duration")
	}

	var duration float64
	n, err := fmt.Sscanf(output, "%f", &duration)
	if err != nil || n != 1 {
		// If parsing fails, return 0 (might be Radio or stream)
		return 0, nil
	}

	// Apple Music returns duration in seconds (as float), Spotify in milliseconds (as int)
	if player == "Spotify" {
		duration = duration / 1000
	}

	return int64(duration), nil
}

func (a *AppleScriptController) GetPosition() (float64, error) {
	// Use cached player from GetMetadata, or find active player
	player := a.currentPlayer
	if player == "" {
		var err error
		player, err = a.findActivePlayer()
		if err != nil {
			return 0, err
		}
	}

	// Browsers don't reliably expose position via AppleScript
	if player == "Safari" || player == "Chrome" {
		return 0, errors.New("position not available for browser")
	}

	var script string

	switch player {
	case "Music":
		script = `tell application "Music"
			try
				return player position
			on error
				return 0
			end try
		end tell`
	case "Spotify":
		script = `tell application "Spotify"
			try
				return player position
			on error
				return 0
			end try
		end tell`
	default:
		return 0, errors.New("position not available")
	}

	output, err := a.runAppleScript(script)
	if err != nil {
		return 0, errors.New("can't get position")
	}

	var position float64
	n, err := fmt.Sscanf(output, "%f", &position)
	if err != nil || n != 1 {
		// If parsing fails, return 0 (might be Radio or stream)
		return 0, nil
	}

	return position, nil
}

func (a *AppleScriptController) Control(command string) error {
	// Use cached player from GetMetadata, or find active player
	player := a.currentPlayer
	if player == "" {
		var err error
		player, err = a.findActivePlayer()
		if err != nil {
			return err
		}
	}

	// Browser control is limited - we can simulate key presses for play/pause
	if player == "Safari" || player == "Chrome" {
		// For browsers, we can't reliably control playback via AppleScript
		// This would require browser extensions or different approach
		return fmt.Errorf("playback control not available for %s", player)
	}

	var script string

	switch player {
	case "Music":
		switch command {
		case "play-pause":
			script = `tell application "Music" to playpause`
		case "next":
			script = `tell application "Music" to next track`
		case "previous":
			script = `tell application "Music" to previous track`
		default:
			return fmt.Errorf("unknown command: %s", command)
		}
	case "Spotify":
		switch command {
		case "play-pause":
			script = `tell application "Spotify" to playpause`
		case "next":
			script = `tell application "Spotify" to next track`
		case "previous":
			script = `tell application "Spotify" to previous track`
		default:
			return fmt.Errorf("unknown command: %s", command)
		}
	default:
		return fmt.Errorf("playback control not available for %s", player)
	}

	_, err := a.runAppleScript(script)
	return err
}
