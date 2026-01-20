package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

const (
	// UI timing constants for adaptive tick rates
	tickRatePlaying = 100 * time.Millisecond  // Smooth animations and progress
	tickRatePaused  = 500 * time.Millisecond  // Reduced frequency when paused
	tickRateIdle    = 1000 * time.Millisecond // Minimal updates when idle

	// Text scrolling constants
	scrollInterval     = 3  // Scroll every 3rd tick
	scrollPauseTicks   = 30 // Pause duration at start/end of scroll (in ticks)
	scrollSeparator    = "  •  "
	scrollSeparatorLen = 5 // Length of "  •  " in runes
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
	rawArtworkData []byte // Raw artwork data for vinyl rotation re-encoding
	forceDeleteImg bool   // Force delete image on next render (for resize cleanup)

	// Vinyl record animation (easter egg)
	vinylRotation     int      // Current rotation angle (0-89 or 0-44) for spinning record effect
	vinylFrameCache   []string // Pre-rendered vinyl frames for smooth playback (45 or 90 frames)
	vinylCacheTrackID string   // Track ID for which frames are cached
	vinylAccumulator  float64  // Fractional frame accumulator for smooth rotation at any RPM
	vinylCachedFrames int      // Number of frames in cache (for detecting config changes)

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
	title      string
	artist     string
	album      string
	status     string
	duration   int64
	position   float64
	artwork    string // Kitty-encoded artwork
	rawArtwork []byte // Raw artwork data for vinyl re-encoding
	color      string // Extracted dominant color
	err        error
}

// Result of generating vinyl frames in background
type vinylFramesMsg struct {
	frames  []string // Pre-rendered vinyl frames (45 frames)
	trackID string   // Track ID these frames belong to
}

// Clear the forceDeleteImg flag after one render cycle
type clearDeleteFlagMsg struct{}

// Schedule next UI refresh tick with adaptive rate
// Playing: 100ms (smooth progress + vinyl rotation)
// Paused: 500ms (just scrolling, save CPU)
// Idle/Error: 1000ms (minimal updates)
func (m model) tickCmd() tea.Cmd {
	cfg := config.Get()
	tickRate := time.Duration(cfg.Timing.UIRefreshMs) * time.Millisecond

	// Adaptive tick rate based on playback state
	if m.lastError != nil {
		// Idle/Error state: slowest rate
		tickRate = tickRateIdle
	} else if !m.isPlaying {
		// Paused: medium rate (still need scrolling)
		tickRate = tickRatePaused
	}
	// Playing: use configured rate (default 100ms)

	return tea.Tick(tickRate, func(t time.Time) tea.Msg {
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

// Generate vinyl frames in background (doesn't block UI)
func generateVinylFramesCmd(rawArtwork []byte, trackID string, frameCount int) tea.Cmd {
	return func() tea.Msg {
		frames := make([]string, frameCount)

		for i := 0; i < frameCount; i++ {
			// Pass frame index and total frame count
			if _, encoded, err := processArtwork(rawArtwork, false, i, frameCount); err == nil {
				frames[i] = encoded
			}
		}
		return vinylFramesMsg{
			frames:  frames,
			trackID: trackID,
		}
	}
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
		var rawArtwork []byte
		if m.supportsKitty && cfg.Artwork.Enabled {
			// Create track ID for caching
			trackID := fmt.Sprintf("%s|%s", title, artist)

			// Skip expensive artwork fetch if paused AND track hasn't changed
			// This is the main CPU bottleneck when idle
			skipArtworkFetch := (strings.ToLower(status) != "playing") && (trackID == m.lastTrackID)

			// Fetch artwork data
			var artworkData []byte
			if !skipArtworkFetch {
				artworkData, err = m.mediaController.GetArtwork()
			}
			if err == nil && len(artworkData) > 0 {
				// Always store raw artwork for vinyl mode
				rawArtwork = artworkData

				// Only re-process if track changed (expensive operation)
				if trackID != m.lastTrackID || m.lastTrackID == "" {
					// Determine if we need color extraction
					shouldExtractColor := cfg.UI.ColorMode == "auto"

					// Process artwork once (decode, extract color, encode for Kitty)
					// This is more efficient than decoding twice
					// Pass rotation angle for vinyl mode (will be 0 for normal mode)
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Silently ignore artwork processing panics
								artworkEncoded = ""
								extractedColor = ""
							}
						}()
						// Pass frame count for proper rotation angle calculation
						color, encoded, err := processArtwork(artworkData, shouldExtractColor, m.vinylRotation, cfg.Artwork.VinylFrames)
						if err == nil {
							if shouldExtractColor && color != "" {
								extractedColor = color
							}
							if encoded != "" {
								artworkEncoded = encoded
							}
						}
					}()
				}
			}
		}

		return songDataMsg{
			title:      title,
			artist:     artist,
			album:      album,
			status:     status,
			duration:   duration,
			position:   position,
			artwork:    artworkEncoded,
			rawArtwork: rawArtwork,
			color:      extractedColor,
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

// updateVinylRotation handles vinyl record rotation animation
// Isolated function to minimize performance impact on normal mode
func (m *model) updateVinylRotation(cfg Config) {
	frameCount := cfg.Artwork.VinylFrames

	// Early return if vinyl mode is disabled or not ready
	if !cfg.Artwork.VinylMode || !m.isPlaying || len(m.vinylFrameCache) != frameCount {
		return
	}

	// Calculate how many frames to advance per tick based on RPM
	// Formula: frames_per_second = RPM / 60 * frame_count
	//          frames_per_tick = frames_per_second * tick_duration_seconds
	framesPerSecond := cfg.Artwork.VinylRPM / 60.0 * float64(frameCount)

	// Tick duration depends on playback state (adaptive tick rate)
	tickDuration := 0.1 // 100ms when playing
	framesPerTick := framesPerSecond * tickDuration

	// Accumulate fractional frames
	m.vinylAccumulator += framesPerTick

	// Advance whole frames when accumulator >= 1
	for m.vinylAccumulator >= 1.0 {
		m.vinylRotation = (m.vinylRotation + 1) % frameCount
		m.vinylAccumulator -= 1.0

		// Use pre-cached frame - no expensive re-encoding!
		m.artworkEncoded = m.vinylFrameCache[m.vinylRotation]
	}
}

func (m model) Init() tea.Cmd {
	// Start both the UI refresh loop and data fetch loop
	return tea.Batch(
		m.tickCmd(),
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

		// On resize, delete the Kitty image to prevent duplication, then redraw
		if m.supportsKitty && m.artworkEncoded != "" {
			// Set flag to trigger delete on next View(), then clear it
			m.forceDeleteImg = true

			// Return a command that will clear the flag after one render cycle
			return m, func() tea.Msg {
				return clearDeleteFlagMsg{}
			}
		}

	case configReloadMsg:
		// Config file changed, update color and artwork setting
		cfg := config.Get()
		if cfg.UI.ColorMode == "manual" {
			m.color = cfg.UI.Color
		}

		// If vinyl mode was disabled, clear cache and reload normal artwork
		if !cfg.Artwork.VinylMode && len(m.vinylFrameCache) > 0 {
			m.vinylFrameCache = nil
			m.vinylCacheTrackID = ""
			m.vinylCachedFrames = 0
			m.lastTrackID = "" // Force artwork reload
			return m, tea.Batch(watchConfigCmd(), m.fetchSongData())
		}

		// If vinyl_frames changed, regenerate cache
		if cfg.Artwork.VinylMode && len(m.vinylFrameCache) > 0 && m.vinylCachedFrames != cfg.Artwork.VinylFrames {
			m.vinylFrameCache = nil
			m.vinylCacheTrackID = ""
			m.vinylCachedFrames = 0
			m.vinylRotation = 0
			m.vinylAccumulator = 0
			if len(m.rawArtworkData) > 0 && m.lastTrackID != "" {
				return m, tea.Batch(watchConfigCmd(), generateVinylFramesCmd(m.rawArtworkData, m.lastTrackID, cfg.Artwork.VinylFrames))
			}
		}

		// If vinyl mode was enabled, generate frames
		if cfg.Artwork.VinylMode && len(m.vinylFrameCache) == 0 && len(m.rawArtworkData) > 0 && m.lastTrackID != "" {
			m.vinylCacheTrackID = ""
			return m, tea.Batch(watchConfigCmd(), generateVinylFramesCmd(m.rawArtworkData, m.lastTrackID, cfg.Artwork.VinylFrames))
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

		// Update vinyl rotation if enabled
		m.updateVinylRotation(cfg)

		// Text scrolling - only if text doesn't fit on screen
		maxLen := cfg.Text.MaxLengthWithArt
		if !m.supportsKitty || !cfg.Artwork.Enabled {
			maxLen = cfg.Text.MaxLengthNoArt
		}

		// Calculate the longest text length to determine if scrolling is needed
		longestLen := len([]rune(m.songData.Title))
		if l := len([]rune(m.songData.Artist)); l > longestLen {
			longestLen = l
		}
		if l := len([]rune(m.songData.Album)); l > longestLen {
			longestLen = l
		}

		// Only scroll if text is longer than max length
		if longestLen > maxLen {
			if m.scrollPause > 0 {
				m.scrollPause--
			} else if m.scrollTick%scrollInterval == 0 { // Scroll every 3rd tick (interval depends on adaptive tick rate)
				m.scrollOffset++

				// Check if we've completed a full loop - pause at the end
				loopPoint := longestLen + scrollSeparatorLen // Text length + separator length (" • ")
				if m.scrollOffset >= loopPoint {
					m.scrollOffset = 0
					m.scrollPause = scrollPauseTicks // Pause for 30 ticks before restarting scroll (3s when playing, 15s when paused, 30s when idle)
				}
			}
		} else {
			// Text fits, no scrolling needed - reset offset
			m.scrollOffset = 0
			m.scrollPause = 0
		}
		// Schedule next tick with adaptive rate
		return m, m.tickCmd()

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
				m.scrollPause = scrollPauseTicks // Pause at start (duration depends on adaptive tick rate)
				m.scrollTick = 0

				// Clear vinyl cache immediately so old artwork doesn't keep spinning
				m.vinylFrameCache = nil
				m.vinylCacheTrackID = ""
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
			m.isPlaying = (strings.ToLower(msg.status) == "playing")
			m.lastError = nil

			// Update artwork if changed
			// trackID already declared above on line 329
			if msg.artwork != "" {
				m.artworkEncoded = msg.artwork
				m.lastTrackID = trackID
			}

			// Store raw artwork data for vinyl mode re-encoding
			if len(msg.rawArtwork) > 0 {
				m.rawArtworkData = msg.rawArtwork

				// Pre-generate all 45 vinyl frames if vinyl mode enabled and track changed
				// Generate in background to avoid blocking the UI
				if cfg.Artwork.VinylMode && trackID != m.vinylCacheTrackID {
					m.vinylCacheTrackID = trackID
					return m, generateVinylFramesCmd(msg.rawArtwork, trackID, cfg.Artwork.VinylFrames)
				}
			}
		}
		return m, nil

	case vinylFramesMsg:
		// Vinyl frames generated in background - cache them if for current track
		if msg.trackID == m.lastTrackID && len(msg.frames) > 0 {
			m.vinylFrameCache = msg.frames
			m.vinylCachedFrames = len(msg.frames) // Store frame count to detect config changes
			// Start from frame 0
			m.vinylRotation = 0
			m.vinylAccumulator = 0
			// Display first frame immediately
			m.artworkEncoded = msg.frames[0]
		}
		return m, nil

	case clearDeleteFlagMsg:
		// Clear the flag after one render cycle
		m.forceDeleteImg = false
		return m, nil
	}

	return m, nil
}
