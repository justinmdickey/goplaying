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
type AppleScriptController struct{}

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

func (a *AppleScriptController) GetMetadata() (title, artist, album, status string, err error) {
	// Check if Spotify is running
	checkScript := `tell application "System Events" to (name of processes) contains "Spotify"`
	result, err := a.runAppleScript(checkScript)
	if err != nil || result != "true" {
		return "", "", "", "", errors.New("Spotify is not running")
	}

	// Get track information
	script := `tell application "Spotify"
		if player state is stopped then
			error "no song playing"
		end if
		set trackName to name of current track
		set trackArtist to artist of current track
		set trackAlbum to album of current track
		set playerState to player state as string
		return trackName & "|" & trackArtist & "|" & trackAlbum & "|" & playerState
	end tell`

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
	script := `tell application "Spotify"
		return duration of current track
	end tell`

	output, err := a.runAppleScript(script)
	if err != nil {
		return 0, errors.New("can't get duration")
	}

	var duration int64
	n, err := fmt.Sscanf(output, "%d", &duration)
	if err != nil || n != 1 {
		return 0, errors.New("failed to parse duration")
	}
	// Convert from milliseconds to seconds
	duration = duration / 1000

	return duration, nil
}

func (a *AppleScriptController) GetPosition() (float64, error) {
	script := `tell application "Spotify"
		return player position
	end tell`

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
	var script string

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

	_, err := a.runAppleScript(script)
	return err
}
