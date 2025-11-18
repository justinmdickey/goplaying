//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// MediaRemoteController implements MediaController using MediaRemote framework
// This provides system-wide Now Playing info for ANY audio source on macOS
type MediaRemoteController struct {
	helperPath string
}

// NewMediaController creates a new media controller for the current platform
func NewMediaController() MediaController {
	// Find the nowplaying helper
	// Try in the same directory as the binary first, then in helpers/nowplaying/
	helperPath := "./nowplaying"
	if _, err := exec.LookPath(helperPath); err != nil {
		helperPath = "./helpers/nowplaying/nowplaying"
		if _, err := exec.LookPath(helperPath); err != nil {
			// Try relative to executable
			if exePath, err := exec.LookPath("goplaying"); err == nil {
				exeDir := filepath.Dir(exePath)
				helperPath = filepath.Join(exeDir, "nowplaying")
			}
		}
	}

	return &MediaRemoteController{
		helperPath: helperPath,
	}
}

func (m *MediaRemoteController) runHelper(args ...string) (string, error) {
	cmd := exec.Command(m.helperPath, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (m *MediaRemoteController) GetMetadata() (title, artist, album, status string, err error) {
	output, err := m.runHelper("metadata")
	if err != nil {
		return "", "", "", "", errors.New("no song playing")
	}

	if output == "" {
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

func (m *MediaRemoteController) GetDuration() (int64, error) {
	output, err := m.runHelper("duration")
	if err != nil {
		return 0, errors.New("can't get duration")
	}

	var duration int64
	n, err := fmt.Sscanf(output, "%d", &duration)
	if err != nil || n != 1 {
		return 0, nil // Return 0 for streams without duration
	}

	return duration, nil
}

func (m *MediaRemoteController) GetPosition() (float64, error) {
	output, err := m.runHelper("position")
	if err != nil {
		return 0, errors.New("can't get position")
	}

	var position float64
	n, err := fmt.Sscanf(output, "%f", &position)
	if err != nil || n != 1 {
		return 0, nil // Return 0 for streams without position
	}

	return position, nil
}

func (m *MediaRemoteController) Control(command string) error {
	_, err := m.runHelper(command)
	return err
}

