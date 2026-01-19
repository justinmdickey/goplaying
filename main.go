package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nfnt/resize"
)

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
	artworkEncoded   string // Kitty protocol-encoded artwork for display
	supportsKitty    bool   // Whether terminal supports Kitty graphics
	lastTrackID      string // Track ID for caching artwork (title+artist)
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
	err      error
}

func formatTime(seconds int64) string {
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

func truncateText(text string, max int) string {
	if len(text) > max {
		return text[:max-3] + "..."
	}
	return text
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
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Schedule next data fetch
func fetchCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return fetchMsg(t)
	})
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
		if m.supportsKitty {
			// Create track ID for caching
			trackID := fmt.Sprintf("%s|%s", title, artist)
			
			// Only fetch artwork if track changed
			if trackID != m.lastTrackID {
				artworkData, err := m.mediaController.GetArtwork()
				if err == nil && len(artworkData) > 0 {
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

	case tickMsg:
		// UI refresh tick - just re-render, no I/O
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
		} else {
			// Use longer truncation when no artwork is displayed
			maxLen := 26
			if !m.supportsKitty {
				maxLen = 35
			}
			
			m.songData.Title = truncateText(msg.title, maxLen)
			m.songData.Artist = truncateText(msg.artist, maxLen)
			m.songData.Album = truncateText(msg.album, maxLen)
			m.songData.Status = msg.status
			m.songData.TotalTime = formatTime(msg.duration)

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

	titleStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	var textContent strings.Builder
	var progressBarContent string

	if m.lastError != nil {
		textContent.WriteString(errorStyle.Render("Error: " + m.lastError.Error()))
	} else {
		textContent.WriteString(titleStyle.Render("🎵 Now Playing") + "\n\n")
		
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

		addLine("Title: ", m.songData.Title)
		addLine("Artist:", m.songData.Artist)
		addLine("Album: ", m.songData.Album)
		addLine("Status:", m.songData.Status)

		if progress > 0 {
			// Progress bar with smooth interpolated position - will be placed below
			barWidth := 43
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
	if m.artworkEncoded != "" && m.supportsKitty {
		// Add padding to the left of text to make room for the image
		paddedText := lipgloss.NewStyle().
			PaddingLeft(16).
			Render(textContent.String())
		
		// Place image and padded text together
		topSection = m.artworkEncoded + paddedText
	} else {
		// No artwork - just show the content without extra header
		topSection = textContent.String()
	}

	// Add progress bar below everything
	var mainContent string
	if progressBarContent != "" {
		mainContent = topSection + progressBarContent
	} else {
		mainContent = topSection
	}

	contentStr := borderStyle.
		Width(60).
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

func main() {
	flag.Parse()

	initialModel := model{
		color:           colorFlag,
		mediaController: NewMediaController(),
		supportsKitty:   supportsKittyGraphics() && !noArtworkFlag,
	}

	if _, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
