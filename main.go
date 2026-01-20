package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/charmbracelet/bubbletea"
)

var colorFlag string
var noArtworkFlag bool
var cpuProfile string
var memProfile string

func init() {
	flag.StringVar(&colorFlag, "color", "2", "Set the desired color (name or hex)")
	flag.StringVar(&colorFlag, "c", "2", "Set the desired color (shorthand)")
	flag.BoolVar(&noArtworkFlag, "no-artwork", false, "Disable album artwork display")
	flag.StringVar(&cpuProfile, "cpuprofile", "", "Write CPU profile to file")
	flag.StringVar(&memProfile, "memprofile", "", "Write memory profile to file")
}

func main() {
	flag.Parse()
	initConfig()

	// CPU profiling
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

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

	// Memory profiling
	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create memory profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not write memory profile: %v\n", err)
		}
	}
}
