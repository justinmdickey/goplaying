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
	players := []string{"Music", "Spotify"}

	for _, player := range players {
		// Check if the application is running and playing
		var checkScript string

		if player == "Music" {
			checkScript = fmt.Sprintf(`
				tell application "System Events"
					if exists (process "%s") then
						tell application "%s"
							if player state is not stopped then
								return "true"
							end if
						end tell
					end if
					return "false"
				end tell`, player, player)
		} else if player == "Spotify" {
			checkScript = fmt.Sprintf(`
				tell application "System Events"
					if exists (process "%s") then
						tell application "%s"
							if player state is not stopped then
								return "true"
							end if
						end tell
					end if
					return "false"
				end tell`, player, player)
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

	if player == "Music" {
		// Apple Music / iTunes
		script = `tell application "Music"
			if player state is stopped then
				error "no song playing"
			end if
			set trackName to name of current track
			set trackArtist to artist of current track
			set trackAlbum to album of current track
			set playerState to player state as string
			return trackName & "|" & trackArtist & "|" & trackAlbum & "|" & playerState
		end tell`
	} else if player == "Spotify" {
		// Spotify
		script = `tell application "Spotify"
			if player state is stopped then
				error "no song playing"
			end if
			set trackName to name of current track
			set trackArtist to artist of current track
			set trackAlbum to album of current track
			set playerState to player state as string
			return trackName & "|" & trackArtist & "|" & trackAlbum & "|" & playerState
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
	if len(parts) != 4 {
		return "", "", "", "", errors.New("unexpected metadata format")
	}

	return strings.TrimSpace(parts[0]),
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

	var script string

	if player == "Music" {
		script = `tell application "Music"
			return duration of current track
		end tell`
	} else if player == "Spotify" {
		script = `tell application "Spotify"
			return duration of current track
		end tell`
	}

	output, err := a.runAppleScript(script)
	if err != nil {
		return 0, errors.New("can't get duration")
	}

	var duration int64
	n, err := fmt.Sscanf(output, "%d", &duration)
	if err != nil || n != 1 {
		return 0, errors.New("failed to parse duration")
	}

	// Apple Music returns duration in seconds, Spotify in milliseconds
	if player == "Spotify" {
		duration = duration / 1000
	}

	return duration, nil
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

	var script string

	if player == "Music" {
		script = `tell application "Music"
			return player position
		end tell`
	} else if player == "Spotify" {
		script = `tell application "Spotify"
			return player position
		end tell`
	}

	output, err := a.runAppleScript(script)
	if err != nil {
		return 0, errors.New("can't get position")
	}

	var position float64
	n, err := fmt.Sscanf(output, "%f", &position)
	if err != nil || n != 1 {
		return 0, errors.New("failed to parse position")
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

	var script string

	if player == "Music" {
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
	} else if player == "Spotify" {
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
	}

	_, err := a.runAppleScript(script)
	return err
}
