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
	songData       SongData
	color          string
	width          int
	height         int
	lastError      error
	mediaController MediaController
}

type tickMsg struct{}

func getSongInfo(mc MediaController) (SongData, error) {
	var data SongData

	title, artist, album, status, err := mc.GetMetadata()
	if err != nil {
		return data, err
	}

	data.Title = truncateText(title, 30)
	data.Artist = truncateText(artist, 30)
	data.Album = truncateText(album, 30)
	data.Status = status

	duration, err := mc.GetDuration()
	if err != nil {
		return data, err
	}

	position, err := mc.GetPosition()
	if err != nil {
		return data, err
	}

	data.CurrentTime = formatTime(int64(position))
	data.TotalTime = formatTime(duration)

	// Guard against division by zero
	if duration > 0 {
		data.Progress = position / float64(duration)
	} else {
		data.Progress = 0.0
	}

	return data, nil
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

func (m model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
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
		case "n":
			if err := m.mediaController.Control("next"); err != nil {
				m.lastError = err
			}
		case "b":
			if err := m.mediaController.Control("previous"); err != nil {
				m.lastError = err
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		data, err := getSongInfo(m.mediaController)
		if err != nil {
			m.lastError = err
		} else {
			m.songData = data
			m.lastError = nil
		}
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg {
			return tickMsg{}
		})
	}
	return m, nil
}

func (m model) View() string {
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

		if m.songData.Progress > 0 {
			// Progress bar
			barWidth := 32
			filled := int(float64(barWidth) * m.songData.Progress)
			progressBar := highlight.Render(strings.Repeat("█", filled)) +
				white.Render(strings.Repeat("─", barWidth-filled))

			content.WriteString(fmt.Sprintf(
				"\n%s %s/%s",
				progressBar,
				highlight.Render(m.songData.CurrentTime),
				highlight.Render(m.songData.TotalTime),
			))
		}
	}

	contentStr := borderStyle.
		Width(50).
		Render(titleStyle.Render("                Now Playing") + "\n\n" + content.String())

  helpText := lipgloss.JoinHorizontal(
    lipgloss.Center,
    "Play/Pause: " + highlight.Render("p"),
    "  Next: " + highlight.Render("n"),
    "  Previous: " + highlight.Render("b"),
    "  Quit: " + highlight.Render("q"),
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
