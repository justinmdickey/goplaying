package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var colorFlag string // Declare the flag variable as a global variable

func init() {
	// Define and initialize the flag
	flag.StringVar(&colorFlag, "color", "green", "set the desired color")
  flag.StringVar(&colorFlag, "c", "green", "set the desired color")
}

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

	flag.Parse()

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
		return "", errors.New("Can't get player metadata for player")
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return "No song is currently playing.", nil
	}

	info := strings.Split(output, "|")
	if len(info) != 4 {
		return "Unexpected output format.", nil
	}

	// Truncate the title, artist, and album to the specified max length
	title := truncateText(strings.TrimSpace(info[0]), maxTitleLength)
	artist := truncateText(strings.TrimSpace(info[1]), maxArtistLength)
	album := truncateText(strings.TrimSpace(info[2]), maxAlbumLength)
	status := strings.TrimSpace(info[3])

	// Get song length
	cmd = exec.Command("playerctl", "metadata", "mpris:length")
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
	cmd = exec.Command("playerctl", "position")
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

	progressBarIcon := fmt.Sprintf("[%s]█", colorFlag)

	// Build the progress bar (e.g., [█████-----]) with the current progress
	progressBar := "[" + strings.Repeat(progressBarIcon, filledLength) + strings.Repeat("[white]-", progressBarTotalWidth-filledLength) + "]"

	// Padding for display
	padding := "    "

	// Display song details with the progress bar and time
	songInfo := ""

	if title != "" {
		songInfo += fmt.Sprintf("\n%s[%s]Title: [-] %s\n", padding, colorFlag, title)
	}

	if artist != "" {
		songInfo += fmt.Sprintf("%s[%s]Artist:[-] %s\n", padding, colorFlag, artist)
	}

	if album != "" {
		songInfo += fmt.Sprintf("%s[%s]Album: [-] %s\n", padding, colorFlag, album)
	}

	if status != "" {
		songInfo += fmt.Sprintf("%s[%s]Status:[-] %s\n", padding, colorFlag, status)
	}

	// Progress bar with time (conditionally include progress bar if times are not empty)
	if progressBar != "" || currentPositionStr != "" || songLengthStr != "" {
		songInfo += fmt.Sprintf("\n%s%s %s/%s\n", padding, progressBar, currentPositionStr, songLengthStr)
	}

	return songInfo, nil
}

// Function to execute playerctl commands
func controlPlayer(command string) error {
	cmd := exec.Command("playerctl", command)
	return cmd.Run()
}

func main() {

  flag.Parse()

	// Set the color for the border
	borderColor := tcell.GetColor(colorFlag)

	// Create a TextView widget
	songText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	songText.SetBorder(true).
		SetTitle("  Now Playing ").
		SetBorderPadding(1, 1, 1, 1).
		SetBorderColor(borderColor).
		SetTitleColor(borderColor).
		SetTitleAlign(tview.AlignCenter)

	controlText := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("\nPlay/Pause: [%s]p[-]  Next: [%s]n[-]  Previous: [%s]b[-]  Quit: [%s]q[-]", borderColor, borderColor, borderColor, borderColor)).
		SetTextAlign(tview.AlignCenter)

	outerBox := tview.NewBox().
		SetBorder(false).
		SetBorderColor(borderColor)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(outerBox, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(tview.NewBox().SetBorder(false), 0, 1, false).
			AddItem(songText, 52, 1, true).
			AddItem(tview.NewBox().SetBorder(false), 0, 1, false),
			0, 4, false).
		AddItem(controlText, 0, 1, false)

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
