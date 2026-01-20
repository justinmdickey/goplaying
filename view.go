package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// getVinylSpinner returns a spinning character based on rotation angle (0-7)
// These create the illusion of a vinyl record spinning
func getVinylSpinner(rotation int) string {
	// Use braille patterns or other Unicode characters for rotation effect
	spinners := []string{"⠁", "⠂", "⠄", "⡀", "⢀", "⠠", "⠐", "⠈"}
	return spinners[rotation%len(spinners)]
}

func (m model) View() string {
	// Get config snapshot for rendering
	cfg := config.Get()

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
		maxLen := cfg.Text.MaxLengthWithArt
		if !m.supportsKitty || !cfg.Artwork.Enabled {
			maxLen = cfg.Text.MaxLengthNoArt
		}

		addLine("󰎈 ", scrollText(m.songData.Title, maxLen, m.scrollOffset))
		addLine("󰠃 ", scrollText(m.songData.Artist, maxLen, m.scrollOffset))
		addLine("󰀥 ", scrollText(m.songData.Album, maxLen, m.scrollOffset))

		// Use different icon based on play state (case-insensitive)
		statusIcon := "󰐊 " // play icon (default)
		statusLower := strings.ToLower(m.songData.Status)
		if statusLower == "paused" {
			statusIcon = "󰏤 " // pause icon
		} else if statusLower == "stopped" {
			statusIcon = "󰓛 " // stop icon
		}
		addLine(statusIcon, m.songData.Status)

		if progress > 0 {
			// Progress bar with smooth interpolated position - will be placed below
			// Bar width calculated from max_width, leaving room for timestamps
			barWidth := cfg.UI.MaxWidth - 17
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
	if m.artworkEncoded != "" && m.supportsKitty && cfg.Artwork.Enabled {
		// Add vinyl spinning effect if enabled (easter egg)
		artworkDisplay := m.artworkEncoded
		if cfg.Artwork.VinylMode {
			// Add spinning indicator overlay
			spinner := getVinylSpinner(m.vinylRotation)
			vinylLabel := highlight.Render(fmt.Sprintf(" %s 33⅓ RPM ", spinner))
			artworkDisplay = m.artworkEncoded + "\n" + vinylLabel
		}

		// Add padding to the left of text to make room for the image
		paddedText := lipgloss.NewStyle().
			PaddingLeft(cfg.Artwork.Padding).
			Render(textContent.String())

		// Place image and padded text together
		topSection = artworkDisplay + paddedText
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
		Width(cfg.UI.MaxWidth).
		Render(mainContent)

	// Build help text - either full help or hint to press ?
	var helpText string
	if m.showHelp {
		helpText = lipgloss.NewStyle().
			Width(cfg.UI.MaxWidth).
			Align(lipgloss.Center).
			Render(lipgloss.JoinHorizontal(
				lipgloss.Center,
				"Play/Pause: "+highlight.Render("p"),
				"  Next: "+highlight.Render("n"),
				"  Previous: "+highlight.Render("b"),
				"  Toggle Art: "+highlight.Render("a"),
				"  Quit: "+highlight.Render("q"),
				"  Hide: "+highlight.Render("?"),
			))
	} else {
		helpText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("Press ? for help")
	}

	fullUI := lipgloss.JoinVertical(lipgloss.Center, contentStr, "\n"+helpText)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		fullUI,
	)
}
