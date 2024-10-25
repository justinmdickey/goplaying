package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"errors"
	"flag"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Helper function to convert seconds to "mm:ss" format
func formatTime(seconds int64) string {
	minutes := seconds / 60
	seconds = seconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// Strip tview color tags (e.g., [green]) for measuring content width correctly
func stripColorTags(input string) string {
	re := regexp.MustCompile(`\[[^\]]+\]`)
	return re.ReplaceAllString(input, "")
}

// Limit the length of a string and add "..." if too long
func truncateText(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength-3] + "..." // Leave room for ellipsis.
	}
	return text
}

func getSongInfo() (string, error) {
	// Limits for title, artist, and album
	const (
		maxTitleLength  = 30
		maxArtistLength = 30
		maxAlbumLength  = 30
	)

	var cmd *exec.Cmd

	// Fetch song metadata (Title, Artist, Album)
	cmd = exec.Command("playerctl", "metadata", "--format", "{{title}}|{{artist}}|{{album}}|{{status}}")

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", errors.New("Can't get player metadata for " + player)
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return "No song is currently playing.", nil
	}

	info := strings.Split(output, "|")
	if len(info) != 4 {
		return "Unexpected output format.", nil
	}

	emptyField := "—" // em dash

	// Truncate the title, artist, and album to the specified max length
	title := truncateText(strings.TrimSpace(info[0]), maxTitleLength)
	artist := truncateText(strings.TrimSpace(info[1]), maxArtistLength)
	if artist == "" {
		artist = emptyField
	}
	album := truncateText(strings.TrimSpace(info[2]), maxAlbumLength)
	if album == "" {
		album = emptyField
	}
	status := strings.TrimSpace(info[3])

	// Get song length
	cmd = exec.Command("playerctl", "-p", player, "metadata", "mpris:length")
	out.Reset()
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return "", errors.New("Can't get track length")
	}

	// Song length is in microseconds, so convert it to seconds
	songLengthMicroseconds := strings.TrimSpace(out.String())
	var songLengthSeconds int64
	fmt.Sscanf(songLengthMicroseconds, "%d", &songLengthSeconds)
	songLengthSeconds = songLengthSeconds / 1e6 // Convert to seconds

	// Get current position
	cmd = exec.Command("playerctl", "-p", player, "position")
	out.Reset()
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return "", errors.New("Can't get track position")
	}

	var currentPosition float64
	fmt.Sscanf(strings.TrimSpace(out.String()), "%f", &currentPosition)

	// Convert song length and position from seconds to mm:ss format
	currentPositionStr := formatTime(int64(currentPosition))
	songLengthStr := formatTime(songLengthSeconds)

	// Calculate progress percentage
	progressPercentage := currentPosition / float64(songLengthSeconds) * 100

	// Set a fixed progress bar width (e.g., 50 characters)
	progressBarTotalWidth := 25

	filledLength := int(progressPercentage / 100 * float64(progressBarTotalWidth))

	// Build the progress bar (e.g., [█████-----]) with the current progress
	progressBar := "[" + strings.Repeat("[green]█", filledLength) + strings.Repeat("[white]-", progressBarTotalWidth-filledLength) + "]"

	// Padding for display
	padding := "    "

	// Display song details with the progress bar and time
	songInfo := fmt.Sprintf(
		"\n%s[green]Title: [-] %s\n%s[green]Artist:[-] %s\n%s[green]Album: [-] %s\n%s[green]Status:[-] %s\n\n%s%s %s/%s\n",
		padding, title,
		padding, artist,
		padding, album,
		padding, status,
		padding, progressBar, currentPositionStr, songLengthStr, // Progress bar with time
	)

	return songInfo, nil
}

// Function to execute playerctl commands
func controlPlayer(command string) error {
	cmd := exec.Command("playerctl", command)
	return cmd.Run()
}

func main() {

	// Create a TextView widget
	songText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	songText.SetBorder(true).
		SetTitle("  Now Playing ").
		SetBorderPadding(1, 1, 1, 1).
		SetBorderColor(tcell.ColorGreen).
		SetTitleColor(tcell.ColorGreen).
		SetTitleAlign(tview.AlignCenter)

	controlText := tview.NewTextView().
		SetDynamicColors(true).
		SetText("\nPlay/Pause: [green]p[-]  Next: [green]n[-]  Previous: [green]b[-]  Quit: [green]q[-]").
		SetTextAlign(tview.AlignCenter)

	outerBox := tview.NewBox().
		SetBorder(false).
		SetTitle("  Now Playing ").
		SetBorderColor(tcell.ColorGreen).
		SetTitleColor(tcell.ColorGreen).
		SetTitleAlign(tview.AlignCenter)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(outerBox, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(tview.NewBox().SetBorder(false), 0, 1, false).
			AddItem(songText, 52, 1, true).
			AddItem(tview.NewBox().SetBorder(false), 0, 1, false),
			0, 4, false).
		AddItem(controlText, 5, 1, false)

	app := tview.NewApplication()

	// Goroutine to update song information periodically
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				songInfo, err := getSongInfo()
				if err != nil {
					songInfo = fmt.Sprintf("Error: %v", err)
				}

				app.QueueUpdateDraw(func() {
					songText.SetText(songInfo)
				})
			}
		}
	}()

	// Set up keybinding handlers
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q': // Quit the application
			app.Stop()
		case 'p': // Play/pause
			err := controlPlayer("play-pause")
			if err != nil {
				fmt.Println("Error executing play-pause:", err)
			}
		case 'n': // Next track
			err := controlPlayer("next")
			if err != nil {
				fmt.Println("Error executing next:", err)
			}
		case 'b': // Previous track
			err := controlPlayer("previous")
			if err != nil {
				fmt.Println("Error executing previous:", err)
			}
		}
		return event
	})

	// Run the application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
