package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var colorFlag string

func init() {
	flag.StringVar(&colorFlag, "color", "2", "Set the desired color (name or hex)")
	flag.StringVar(&colorFlag, "c", "2", "Set the desired color (shorthand)")
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

		return songDataMsg{
			title:    title,
			artist:   artist,
			album:    album,
			status:   status,
			duration: duration,
			position: position,
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
			m.songData.Title = truncateText(msg.title, 30)
			m.songData.Artist = truncateText(msg.artist, 30)
			m.songData.Album = truncateText(msg.album, 30)
			m.songData.Status = msg.status
			m.songData.TotalTime = formatTime(msg.duration)

			// Update tracking info for smooth interpolation
			m.lastPosition = msg.position
			m.lastPositionTime = time.Now()
			m.duration = msg.duration
			m.isPlaying = (msg.status == "playing")
			m.lastError = nil
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

	var content strings.Builder

	if m.lastError != nil {
		content.WriteString(errorStyle.Render("Error: " + m.lastError.Error()))
	} else {
		addLine := func(label, value string) {
			if value != "" {
				content.WriteString(
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
			// Progress bar with smooth interpolated position
			barWidth := 32
			filled := int(float64(barWidth) * progress)
			progressBar := highlight.Render(strings.Repeat("█", filled)) +
				white.Render(strings.Repeat("─", barWidth-filled))

			content.WriteString(fmt.Sprintf(
				"\n%s %s/%s",
				progressBar,
				highlight.Render(currentTime),
				highlight.Render(m.songData.TotalTime),
			))
		}
	}

	contentStr := borderStyle.
		Width(50).
		Render(titleStyle.Render("                Now Playing") + "\n\n" + content.String())

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
	}

	if _, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
