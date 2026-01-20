package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
)

var colorFlag string
var noArtworkFlag bool

func init() {
	flag.StringVar(&colorFlag, "color", "2", "Set the desired color (name or hex)")
	flag.StringVar(&colorFlag, "c", "2", "Set the desired color (shorthand)")
	flag.BoolVar(&noArtworkFlag, "no-artwork", false, "Disable album artwork display")
}

func main() {
	flag.Parse()
	initConfig()

	// Start with manual color, auto mode will override when artwork loads
	cfg := config.Get()
	initialColor := cfg.UI.Color

	initialModel := model{
		color:           initialColor,
		mediaController: NewMediaController(),
		supportsKitty:   supportsKittyGraphics() && cfg.Artwork.Enabled,
	}

	if _, err := tea.NewProgram(initialModel, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
