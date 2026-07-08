package main

import "errors"

// ErrNothingPlaying indicates no active player or track. The UI treats this
// as the friendly idle state rather than an error.
var ErrNothingPlaying = errors.New("nothing playing")

// MediaController defines the interface for controlling media playback across platforms
type MediaController interface {
	GetMetadata() (title, artist, album, status string, err error)
	GetDuration() (int64, error)
	GetPosition() (float64, error)
	Control(command string) error
	// GetArtwork returns raw image bytes (PNG/JPEG/etc), not base64
	GetArtwork() ([]byte, error)
}
