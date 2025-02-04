package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	songData  SongData
	color     string
	width     int
	height    int
	lastError error
}

type tickMsg struct{}

func getSongInfo() (SongData, error) {
	var data SongData

	cmd := exec.Command("playerctl", "metadata", "--format", "{{title}}|{{artist}}|{{album}}|{{status}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return data, errors.New("can't get metadata")
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return data, errors.New("no song playing")
	}

	parts := strings.Split(output, "|")
	if len(parts) != 4 {
		return data, errors.New("unexpected metadata format")
	}

	data.Title = truncateText(strings.TrimSpace(parts[0]), 30)
	data.Artist = truncateText(strings.TrimSpace(parts[1]), 30)
	data.Album = truncateText(strings.TrimSpace(parts[2]), 30)
	data.Status = strings.TrimSpace(parts[3])

	cmd = exec.Command("playerctl", "metadata", "mpris:length")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return data, errors.New("can't get duration")
	}

	var duration int64
	fmt.Sscanf(strings.TrimSpace(out.String()), "%d", &duration)
	duration = duration / 1e6

	cmd = exec.Command("playerctl", "position")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return data, errors.New("can't get position")
	}

	var position float64
	fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &position)

	data.CurrentTime = formatTime(int64(position))
	data.TotalTime = formatTime(duration)
	data.Progress = position / float64(duration)

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
			controlPlayer("play-pause")
		case "n":
			controlPlayer("next")
		case "b":
			controlPlayer("previous")
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		data, err := getSongInfo()
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
		highlight.Render("Play/Pause: [p]"),
		highlight.Render("  Next: [n]"),
		highlight.Render("  Previous: [b]"),
		highlight.Render("  Quit: [q]"),
	)

	fullUI := lipgloss.JoinVertical(lipgloss.Center, contentStr, "\n"+helpText)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		fullUI,
	)
}

func controlPlayer(command string) error {
	return exec.Command("playerctl", command).Run()
}

func main() {
	flag.Parse()

	initialModel := model{
		color: colorFlag,
	}

	if _, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
