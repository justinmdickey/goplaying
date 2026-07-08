package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"

	tea "github.com/charmbracelet/bubbletea"
)

// Set by GoReleaser via ldflags (-X main.version=... etc.)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var colorFlag string
var noArtworkFlag bool
var versionFlag bool
var cpuProfile string
var memProfile string

func init() {
	flag.StringVar(&colorFlag, "color", "2", "Set the desired color (name or hex)")
	flag.StringVar(&colorFlag, "c", "2", "Set the desired color (shorthand)")
	flag.BoolVar(&noArtworkFlag, "no-artwork", false, "Disable album artwork display")
	flag.BoolVar(&versionFlag, "version", false, "Print version and exit")
	flag.StringVar(&cpuProfile, "cpuprofile", "", "Write CPU profile to file")
	flag.StringVar(&memProfile, "memprofile", "", "Write memory profile to file")
}

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Printf("goplaying %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	// Track which flags were explicitly passed, so defaults don't shadow
	// config-file values (e.g. an explicit `-c 2` works, and an absent -c
	// doesn't override the config color)
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	initConfig(explicitFlags)

	// CPU profiling
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = f.Close() }()
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
		// Terminal capability only — whether artwork is shown is a config
		// decision checked at render/fetch time, so toggling artwork on at
		// runtime works even when it was disabled at startup
		supportsKitty: supportsKittyGraphics(),
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
		defer func() { _ = f.Close() }()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not write memory profile: %v\n", err)
		}
	}
}
