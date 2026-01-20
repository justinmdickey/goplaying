package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// SongData holds the current track metadata
type SongData struct {
	Status      string
	Title       string
	Artist      string
	Album       string
	CurrentTime string
	TotalTime   string
	Progress    float64
}

// model is the Bubble Tea model for the TUI application
type model struct {
	songData        SongData
	color           string
	width           int
	height          int
	lastError       error
	mediaController MediaController

	// For smooth position interpolation
	lastPosition     float64   // Last known position in seconds
	lastPositionTime time.Time // When we fetched that position
	duration         int64     // Track duration in seconds
	isPlaying        bool      // Whether song is currently playing

	// Album artwork support
	artworkEncoded string // Kitty protocol-encoded artwork for display
	supportsKitty  bool   // Whether terminal supports Kitty graphics
	lastTrackID    string // Track ID for caching artwork (title+artist)

	// Text scrolling state
	scrollOffset int // Current scroll position for text animation
	scrollPause  int // Pause counter at start/end of scroll
	scrollTick   int // Tick counter for slowing scroll speed

	// UI state
	showHelp bool // Whether to show help text
}

// UI refresh tick - fires every 100ms for smooth rendering
type tickMsg time.Time

// Data fetch tick - fires every second to get fresh metadata
type fetchMsg time.Time

// Result of fetching song data from media controller
type songDataMsg struct {
	title    string
	artist   string
	album    string
	status   string
	duration int64
	position float64
	artwork  string // Kitty-encoded artwork
	color    string // Extracted dominant color
	err      error
}

// Schedule next UI refresh tick
func tickCmd() tea.Cmd {
	cfg := config.Get()
	return tea.Tick(time.Duration(cfg.Timing.UIRefreshMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Schedule next data fetch
func fetchCmd() tea.Cmd {
	cfg := config.Get()
	return tea.Tick(time.Duration(cfg.Timing.DataFetchMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return fetchMsg(t)
	})
}

// Fetch song data in background (doesn't block UI)
func (m model) fetchSongData() tea.Cmd {
	return func() tea.Msg {
		// Get config snapshot at start of fetch
		cfg := config.Get()

		title, artist, album, status, err := m.mediaController.GetMetadata()
		if err != nil {
			return songDataMsg{err: err}
		}

		duration, err := m.mediaController.GetDuration()
		if err != nil {
			return songDataMsg{err: err}
		}

		position, err := m.mediaController.GetPosition()
		if err != nil {
			return songDataMsg{err: err}
		}

		// Fetch artwork if Kitty protocol is supported
		var artworkEncoded string
		var extractedColor string
		if m.supportsKitty && cfg.Artwork.Enabled {
			// Create track ID for caching
			trackID := fmt.Sprintf("%s|%s", title, artist)

			// Only fetch artwork if track changed or first load
			if trackID != m.lastTrackID || m.lastTrackID == "" {
				artworkData, err := m.mediaController.GetArtwork()
				if err == nil && len(artworkData) > 0 {
					// Extract dominant color if in auto mode
					if cfg.UI.ColorMode == "auto" {
						if color, err := extractDominantColor(artworkData); err == nil && color != "" {
							extractedColor = color
						}
					}

					// Wrap encoding in recovery to prevent crashes
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Silently ignore artwork encoding panics
								artworkEncoded = ""
							}
						}()
						encoded, err := encodeArtworkForKitty(artworkData)
						if err == nil && encoded != "" {
							artworkEncoded = encoded
						}
					}()
				}
			}
		}

		return songDataMsg{
			title:    title,
			artist:   artist,
			album:    album,
			status:   status,
			duration: duration,
			position: position,
			artwork:  artworkEncoded,
			color:    extractedColor,
		}
	}
}

// Calculate current position with smooth interpolation
func (m model) getCurrentPosition() float64 {
	// If paused, return last known position
	if !m.isPlaying {
		return m.lastPosition
	}

	// If playing, interpolate based on elapsed time since last fetch
	elapsed := time.Since(m.lastPositionTime).Seconds()
	currentPos := m.lastPosition + elapsed

	// Clamp to duration
	if m.duration > 0 && currentPos > float64(m.duration) {
		currentPos = float64(m.duration)
	}

	return currentPos
}

func (m model) Init() tea.Cmd {
	// Start both the UI refresh loop and data fetch loop
	return tea.Batch(
		tickCmd(),
		fetchCmd(),
		watchConfigCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "p":
			if err := m.mediaController.Control("play-pause"); err != nil {
				m.lastError = err
			}
			// Immediately fetch fresh state after control action
			return m, m.fetchSongData()
		case "n":
			if err := m.mediaController.Control("next"); err != nil {
				m.lastError = err
			}
			return m, m.fetchSongData()
		case "b":
			if err := m.mediaController.Control("previous"); err != nil {
				m.lastError = err
			}
			return m, m.fetchSongData()
		case "a":
			// Toggle artwork on/off
			cfg := config.Get()
			cfg.Artwork.Enabled = !cfg.Artwork.Enabled
			config.Set(cfg)
			if !cfg.Artwork.Enabled {
				// Clear artwork when disabling
				m.artworkEncoded = ""
			} else if m.supportsKitty {
				// Re-fetch artwork when enabling
				m.lastTrackID = "" // Clear track ID to force artwork fetch
				return m, m.fetchSongData()
			}
			return m, nil
		case "?":
			// Toggle help text
			m.showHelp = !m.showHelp
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case configReloadMsg:
		// Config file changed, update color and artwork setting
		cfg := config.Get()
		if cfg.UI.ColorMode == "manual" {
			m.color = cfg.UI.Color
		}
		if !cfg.Artwork.Enabled && m.artworkEncoded != "" {
			// Delete the image from terminal and clear the encoded data
			m.artworkEncoded = ""
			m.lastTrackID = "" // Clear track ID so artwork can be re-fetched later
		} else if cfg.Artwork.Enabled && m.artworkEncoded == "" && m.supportsKitty {
			// Artwork was just enabled, clear track ID and fetch it for the current song
			m.lastTrackID = ""
			return m, tea.Batch(watchConfigCmd(), m.fetchSongData())
		}
		// Continue watching for more config changes
		return m, watchConfigCmd()

	case tickMsg:
		// UI refresh tick - advance scroll animation slowly
		m.scrollTick++
		cfg := config.Get()

		if m.scrollPause > 0 {
			m.scrollPause--
		} else if m.scrollTick%3 == 0 { // Scroll every 3rd tick (300ms)
			m.scrollOffset++

			// Check if we've completed a full loop - pause at the end
			maxLen := cfg.Text.MaxLengthWithArt
			if !m.supportsKitty || !cfg.Artwork.Enabled {
				maxLen = cfg.Text.MaxLengthNoArt
			}

			// Calculate the longest text length to determine loop point
			longestLen := len([]rune(m.songData.Title))
			if l := len([]rune(m.songData.Artist)); l > longestLen {
				longestLen = l
			}
			if l := len([]rune(m.songData.Album)); l > longestLen {
				longestLen = l
			}

			if longestLen > maxLen {
				loopPoint := longestLen + 5 // Text length + separator length
				if m.scrollOffset >= loopPoint {
					m.scrollOffset = 0
					m.scrollPause = 30 // Pause for 3 seconds when looping back
				}
			}
		}
		// Schedule next tick immediately for consistent timing
		return m, tickCmd()

	case fetchMsg:
		// Data fetch tick - get fresh data and schedule next fetch
		return m, tea.Batch(
			fetchCmd(),
			m.fetchSongData(),
		)

	case songDataMsg:
		// Received fresh song data
		cfg := config.Get()
		if msg.err != nil {
			m.lastError = msg.err
			// Clear artwork when nothing is playing
			m.artworkEncoded = ""
			m.lastTrackID = ""
		} else {
			// Store full text and reset scroll when track changes
			trackID := fmt.Sprintf("%s|%s", msg.title, msg.artist)
			if trackID != m.lastTrackID {
				m.scrollOffset = 0
				m.scrollPause = 30 // Pause at start for 3 seconds
				m.scrollTick = 0
			}

			m.songData.Title = msg.title
			m.songData.Artist = msg.artist
			m.songData.Album = msg.album
			m.songData.Status = msg.status
			m.songData.TotalTime = formatTime(msg.duration)

			// Update color if we extracted one in auto mode
			// Don't fall back to manual on every fetch - only when track changes
			if cfg.UI.ColorMode == "auto" && msg.color != "" {
				m.color = msg.color
			}

			// Update tracking info for smooth interpolation
			m.lastPosition = msg.position
			m.lastPositionTime = time.Now()
			m.duration = msg.duration
			m.isPlaying = (msg.status == "playing")
			m.lastError = nil

			// Update artwork if changed
			if msg.artwork != "" {
				m.artworkEncoded = msg.artwork
				m.lastTrackID = fmt.Sprintf("%s|%s", msg.title, msg.artist)
			}
		}
		return m, nil
	}

	return m, nil
}
