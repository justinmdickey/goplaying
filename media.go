package main

// MediaController defines the interface for controlling media playback across platforms
type MediaController interface {
	GetMetadata() (title, artist, album, status string, err error)
	GetDuration() (int64, error)
	GetPosition() (float64, error)
	Control(command string) error
	GetArtwork() ([]byte, error)
}
