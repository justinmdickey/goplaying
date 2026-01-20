package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/nfnt/resize"
	"github.com/spf13/viper"
	_ "golang.org/x/image/webp"
)

// Config holds all application configuration
type Config struct {
	UI struct {
		Color     string `mapstructure:"color"`
		ColorMode string `mapstructure:"color_mode"`
		MaxWidth  int    `mapstructure:"max_width"`
	} `mapstructure:"ui"`
	Artwork struct {
		Enabled bool `mapstructure:"enabled"`
		Padding int  `mapstructure:"padding"`
	} `mapstructure:"artwork"`
	Text struct {
		MaxLengthWithArt int `mapstructure:"max_length_with_art"`
		MaxLengthNoArt   int `mapstructure:"max_length_no_art"`
	} `mapstructure:"text"`
	Timing struct {
		UIRefreshMs int `mapstructure:"ui_refresh_ms"`
		DataFetchMs int `mapstructure:"data_fetch_ms"`
	} `mapstructure:"timing"`
}

var config Config

var colorFlag string
var noArtworkFlag bool

func init() {
	flag.StringVar(&colorFlag, "color", "2", "Set the desired color (name or hex)")
	flag.StringVar(&colorFlag, "c", "2", "Set the desired color (shorthand)")
	flag.BoolVar(&noArtworkFlag, "no-artwork", false, "Disable album artwork display")
}

type SongData struct {
	Status      string
	Title       string
	Artist      string
	Album       string
	CurrentTime string
	TotalTime   string
	Progress    float64
}

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

// Config file changed notification
type configReloadMsg struct{}

// Channel for config change notifications
var configChangeChan = make(chan struct{}, 1)

func formatTime(seconds int64) string {
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func scrollText(text string, max int, offset int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}

	// Add padding for smooth loop
	fullText := append(runes, []rune("  •  ")...)
	textLen := len(fullText)

	// Wrap offset around
	offset = offset % textLen

	// Build visible window
	var result []rune
	for i := 0; i < max; i++ {
		result = append(result, fullText[(offset+i)%textLen])
	}
	return string(result)
}

// Extract dominant color from image and convert to hex
// Uses a sampling approach to find vibrant, light colors suitable for dark backgrounds
func extractDominantColor(imgData []byte) (string, error) {
	// Decode base64 if needed (from MediaRemote/playerctl)
	var imageData []byte
	if decoded, err := base64.StdEncoding.DecodeString(string(imgData)); err == nil {
		imageData = decoded
	} else {
		// Already raw data
		imageData = imgData
	}

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()

	// Sample colors from the image by taking every Nth pixel
	// This is much faster than analyzing every pixel
	colorMap := make(map[uint32]int)
	sampleRate := 5 // Sample every 5th pixel

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleRate {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleRate {
			r, g, b, a := img.At(x, y).RGBA()

			// Skip transparent pixels
			if a < 32768 {
				continue
			}

			// Convert from 16-bit to 8-bit color
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			// Pack RGB into a single uint32 for counting
			rgb := (uint32(r8) << 16) | (uint32(g8) << 8) | uint32(b8)
			colorMap[rgb]++
		}
	}

	// Find colors that are light and saturated enough for readability
	type colorScore struct {
		rgb   uint32
		count int
		score float64
	}

	var candidates []colorScore

	for rgb, count := range colorMap {
		r := uint8(rgb >> 16)
		g := uint8(rgb >> 8)
		b := uint8(rgb)

		// Calculate lightness and saturation
		rf := float64(r) / 255.0
		gf := float64(g) / 255.0
		bf := float64(b) / 255.0

		max := rf
		if gf > max {
			max = gf
		}
		if bf > max {
			max = bf
		}

		min := rf
		if gf < min {
			min = gf
		}
		if bf < min {
			min = bf
		}

		lightness := (max + min) / 2.0

		var saturation float64
		if max != min {
			if lightness > 0.5 {
				saturation = (max - min) / (2.0 - max - min)
			} else {
				saturation = (max - min) / (max + min)
			}
		}

		// Skip colors that are too dark, too light (near-white), or too unsaturated
		if lightness < 0.3 || lightness > 0.85 || saturation < 0.25 {
			continue
		}

		// Score formula: balance saturation and lightness
		// Prefer vibrant colors (high saturation) that are reasonably light
		// Ideal lightness is around 0.5-0.7 (readable but not washed out)
		lightnessScore := lightness
		if lightness > 0.7 {
			// Penalize very light colors
			lightnessScore = 0.7 - (lightness - 0.7)
		}

		score := (saturation * 2.5) + (lightnessScore * 1.5) + (float64(count) / 1000.0)

		candidates = append(candidates, colorScore{rgb: rgb, count: count, score: score})
	}

	if len(candidates) == 0 {
		// Fallback: try K-means if our sampling didn't find good colors
		colors, err := prominentcolor.Kmeans(img)
		if err != nil || len(colors) == 0 {
			return "", fmt.Errorf("no suitable colors found")
		}
		c := colors[0]
		return fmt.Sprintf("#%02x%02x%02x", c.Color.R, c.Color.G, c.Color.B), nil
	}

	// Sort by score (highest first)
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// Use the highest scoring color
	best := candidates[0]
	r := uint8(best.rgb >> 16)
	g := uint8(best.rgb >> 8)
	b := uint8(best.rgb)

	return fmt.Sprintf("#%02x%02x%02x", r, g, b), nil
}

// Check if terminal supports Kitty graphics protocol
func supportsKittyGraphics() bool {
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")

	// Check TERM variable
	if strings.Contains(term, "kitty") || strings.Contains(term, "konsole") {
		return true
	}

	// Check TERM_PROGRAM for Ghostty and other terminals
	if termProgram == "ghostty" || termProgram == "WezTerm" {
		return true
	}

	return false
}

// Process and encode artwork for Kitty graphics protocol
func encodeArtworkForKitty(artworkData []byte) (string, error) {
	// Decode base64 if needed (from MediaRemote/playerctl)
	var imageData []byte
	if decoded, err := base64.StdEncoding.DecodeString(string(artworkData)); err == nil {
		imageData = decoded
	} else {
		// Already raw data
		imageData = artworkData
	}

	// Validate we have data
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize maintaining aspect ratio - keep it reasonable for terminal display
	// We'll let Kitty handle the final sizing based on cell dimensions
	resized := resize.Resize(300, 0, img, resize.Lanczos3)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return "", fmt.Errorf("failed to encode PNG: %w", err)
	}

	// Encode as base64 for Kitty protocol
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Kitty protocol needs chunking for large payloads (max 4096 bytes per chunk)
	const chunkSize = 4096
	var result strings.Builder

	// Use a fixed image ID and delete any previous image first
	const imageID = 42
	result.WriteString(fmt.Sprintf("\033_Ga=d,d=I,i=%d\033\\", imageID))

	if len(encoded) <= chunkSize {
		// Small enough to send in one go
		// Use columns (c) instead of pixels for zoom-independent sizing
		// c=13 means 13 terminal columns wide, height auto-calculated
		result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,t=d,i=%d,c=13,C=1;%s\033\\", imageID, encoded))
	} else {
		// Need to chunk the data
		for i := 0; i < len(encoded); i += chunkSize {
			end := i + chunkSize
			if end > len(encoded) {
				end = len(encoded)
			}
			chunk := encoded[i:end]

			if i == 0 {
				// First chunk with columns-based sizing
				result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,t=d,i=%d,c=13,C=1,m=1;%s\033\\", imageID, chunk))
			} else if end == len(encoded) {
				// Last chunk - m=0 (no more data)
				result.WriteString(fmt.Sprintf("\033_Gm=0;%s\033\\", chunk))
			} else {
				// Middle chunk - m=1 (more data coming)
				result.WriteString(fmt.Sprintf("\033_Gm=1;%s\033\\", chunk))
			}
		}
	}

	return result.String(), nil
}

// Schedule next UI refresh tick
func tickCmd() tea.Cmd {
	return tea.Tick(time.Duration(config.Timing.UIRefreshMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Schedule next data fetch
func fetchCmd() tea.Cmd {
	return tea.Tick(time.Duration(config.Timing.DataFetchMs)*time.Millisecond, func(t time.Time) tea.Msg {
		return fetchMsg(t)
	})
}

// Watch for config file changes
func watchConfigCmd() tea.Cmd {
	return func() tea.Msg {
		<-configChangeChan
		return configReloadMsg{}
	}
}

// Fetch song data in background (doesn't block UI)
func (m model) fetchSongData() tea.Cmd {
	return func() tea.Msg {
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
		if m.supportsKitty && config.Artwork.Enabled {
			// Create track ID for caching
			trackID := fmt.Sprintf("%s|%s", title, artist)

			// Only fetch artwork if track changed or first load
			if trackID != m.lastTrackID || m.lastTrackID == "" {
				artworkData, err := m.mediaController.GetArtwork()
				if err == nil && len(artworkData) > 0 {
					// Extract dominant color if in auto mode
					if config.UI.ColorMode == "auto" {
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
			err:      nil,
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case configReloadMsg:
		// Config file changed, update color and artwork setting
		if config.UI.ColorMode == "manual" {
			m.color = config.UI.Color
		}
		if !config.Artwork.Enabled && m.artworkEncoded != "" {
			// Delete the image from terminal and clear the encoded data
			m.artworkEncoded = ""
			m.lastTrackID = "" // Clear track ID so artwork can be re-fetched later
		} else if config.Artwork.Enabled && m.artworkEncoded == "" && m.supportsKitty {
			// Artwork was just enabled, clear track ID and fetch it for the current song
			m.lastTrackID = ""
			return m, tea.Batch(watchConfigCmd(), m.fetchSongData())
		}
		// Continue watching for more config changes
		return m, watchConfigCmd()

	case tickMsg:
		// UI refresh tick - advance scroll animation slowly
		m.scrollTick++

		if m.scrollPause > 0 {
			m.scrollPause--
		} else if m.scrollTick%3 == 0 { // Scroll every 3rd tick (300ms)
			m.scrollOffset++

			// Check if we've completed a full loop - pause at the end
			maxLen := config.Text.MaxLengthWithArt
			if !m.supportsKitty || !config.Artwork.Enabled {
				maxLen = config.Text.MaxLengthNoArt
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
			if config.UI.ColorMode == "auto" && msg.color != "" {
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

func (m model) View() string {
	// Calculate current interpolated position for smooth progress bar
	currentPos := m.getCurrentPosition()
	currentTime := formatTime(int64(currentPos))
	var progress float64
	if m.duration > 0 {
		progress = currentPos / float64(m.duration)
	}

	// Use lipgloss.Color to validate the color input
	color := lipgloss.Color(m.color)
	highlight := lipgloss.NewStyle().Foreground(color)
	white := lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // ANSI white

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	var textContent strings.Builder
	var progressBarContent string

	if m.lastError != nil {
		// Check if it's a "nothing playing" state vs actual error
		errMsg := m.lastError.Error()
		isNothingPlaying := strings.Contains(errMsg, "can't get metadata") ||
			strings.Contains(errMsg, "no active music player") ||
			strings.Contains(errMsg, "no song playing")

		if isNothingPlaying {
			// Show friendly placeholder for "nothing playing" state
			textContent.WriteString(highlight.Render("󰓃 Now Playing") + "\n\n")
			textContent.WriteString(mutedStyle.Render("Nothing playing") + "\n\n")
			textContent.WriteString(dimStyle.Render("Start playing music to begin"))
		} else {
			// Actual error - show in muted color (not bright red)
			textContent.WriteString(errorStyle.Render("Error: " + errMsg))
		}
	} else {
		textContent.WriteString(highlight.Render("󰓃 Now Playing") + "\n\n")

		addLine := func(label, value string) {
			if value != "" {
				textContent.WriteString(
					fmt.Sprintf("%s %s\n",
						labelStyle.Render(label),
						value,
					),
				)
			}
		}

		// Calculate max length for text
		maxLen := config.Text.MaxLengthWithArt
		if !m.supportsKitty || !config.Artwork.Enabled {
			maxLen = config.Text.MaxLengthNoArt
		}

		addLine("󰎈 ", scrollText(m.songData.Title, maxLen, m.scrollOffset))
		addLine("󰠃 ", scrollText(m.songData.Artist, maxLen, m.scrollOffset))
		addLine("󰀥 ", scrollText(m.songData.Album, maxLen, m.scrollOffset))
		addLine("󰐊 ", m.songData.Status)

		if progress > 0 {
			// Progress bar with smooth interpolated position - will be placed below
			// Bar width calculated from max_width, leaving room for timestamps
			barWidth := config.UI.MaxWidth - 17
			filled := int(float64(barWidth) * progress)
			progressBar := highlight.Render(strings.Repeat("█", filled)) +
				white.Render(strings.Repeat("─", barWidth-filled))

			progressBarContent = fmt.Sprintf(
				"\n%s %s/%s",
				progressBar,
				highlight.Render(currentTime),
				highlight.Render(m.songData.TotalTime),
			)
		}
	}

	// Combine artwork and text content
	var topSection string
	if m.artworkEncoded != "" && m.supportsKitty && config.Artwork.Enabled {
		// Add padding to the left of text to make room for the image
		paddedText := lipgloss.NewStyle().
			PaddingLeft(config.Artwork.Padding).
			Render(textContent.String())

		// Place image and padded text together
		topSection = m.artworkEncoded + paddedText
	} else {
		// No artwork - delete any existing image and show content without padding
		if m.supportsKitty {
			// Send delete command for the image
			const imageID = 42
			topSection = fmt.Sprintf("\033_Ga=d,d=I,i=%d\033\\", imageID) + textContent.String()
		} else {
			topSection = textContent.String()
		}
	}

	// Add progress bar below everything
	var mainContent string
	if progressBarContent != "" {
		mainContent = topSection + progressBarContent
	} else {
		mainContent = topSection
	}

	contentStr := borderStyle.
		Width(config.UI.MaxWidth).
		Render(mainContent)

	helpText := lipgloss.JoinHorizontal(
		lipgloss.Center,
		"Play/Pause: "+highlight.Render("p"),
		"  Next: "+highlight.Render("n"),
		"  Previous: "+highlight.Render("b"),
		"  Quit: "+highlight.Render("q"),
	)

	fullUI := lipgloss.JoinVertical(lipgloss.Center, contentStr, "\n"+helpText)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		fullUI,
	)
}

func initConfig() {
	// Set defaults
	viper.SetDefault("ui.color", "2")
	viper.SetDefault("ui.color_mode", "manual")
	viper.SetDefault("ui.max_width", 45)
	viper.SetDefault("artwork.enabled", true)
	viper.SetDefault("artwork.padding", 15)
	viper.SetDefault("text.max_length_with_art", 22)
	viper.SetDefault("text.max_length_no_art", 36)
	viper.SetDefault("timing.ui_refresh_ms", 100)
	viper.SetDefault("timing.data_fetch_ms", 1000)

	// Set config file location following XDG standard
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Check XDG_CONFIG_HOME first, fallback to ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configHome = filepath.Join(homeDir, ".config")
		}
	}

	if configHome != "" {
		viper.AddConfigPath(filepath.Join(configHome, "goplaying"))
	}

	// Environment variable support with GOPLAYING_ prefix
	viper.SetEnvPrefix("GOPLAYING")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (ignore error if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file found but had errors
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file: %v\n", err)
		}
	}

	// Bind command-line flags (they take precedence)
	if colorFlag != "2" { // Only override if flag was explicitly set
		viper.Set("ui.color", colorFlag)
	}
	if noArtworkFlag {
		viper.Set("artwork.enabled", false)
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error parsing config: %v\n", err)
	}

	// Watch for config file changes and live reload
	viper.OnConfigChange(func(e fsnotify.Event) {
		if err := viper.Unmarshal(&config); err == nil {
			// Config reloaded successfully, notify the app
			select {
			case configChangeChan <- struct{}{}:
			default:
				// Channel full, skip notification
			}
		}
	})
	viper.WatchConfig()
}

func main() {
	flag.Parse()
	initConfig()

	// Start with manual color, auto mode will override when artwork loads
	initialColor := config.UI.Color

	initialModel := model{
		color:           initialColor,
		mediaController: NewMediaController(),
		supportsKitty:   supportsKittyGraphics() && config.Artwork.Enabled,
	}

	if _, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
