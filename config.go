package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	UI struct {
		Color     string `mapstructure:"color"`
		ColorMode string `mapstructure:"color_mode"`
		MaxWidth  int    `mapstructure:"max_width"`
	} `mapstructure:"ui"`
	Artwork struct {
		Enabled      bool    `mapstructure:"enabled"`
		Padding      int     `mapstructure:"padding"`
		WidthPixels  int     `mapstructure:"width_pixels"`
		WidthColumns int     `mapstructure:"width_columns"`
		VinylMode    bool    `mapstructure:"vinyl_mode"` // Easter egg: spinning vinyl record animation
		VinylRPM     float64 `mapstructure:"vinyl_rpm"`  // Rotation speed in RPM (revolutions per minute)
	} `mapstructure:"artwork"`
	Text struct {
		MaxLengthWithArt int `mapstructure:"max_length_with_art"`
		MaxLengthNoArt   int `mapstructure:"max_length_no_art"`
	} `mapstructure:"text"`
	Timing struct {
		UIRefreshMs int `mapstructure:"ui_refresh_ms"`
		DataFetchMs int `mapstructure:"data_fetch_ms"`
	} `mapstructure:"timing"`
}

// SafeConfig wraps Config with thread-safe access
type SafeConfig struct {
	mu  sync.RWMutex
	cfg Config
}

// Get returns a copy of the current config (thread-safe read)
func (sc *SafeConfig) Get() Config {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.cfg
}

// Set updates the config (thread-safe write)
func (sc *SafeConfig) Set(cfg Config) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cfg = cfg
}

var config = &SafeConfig{}

// Validation error types
type configError struct {
	field   string
	message string
}

func (e configError) Error() string {
	return fmt.Sprintf("%s: %s", e.field, e.message)
}

// isValidColor checks if a color string is valid (ANSI code or hex color)
func isValidColor(color string) bool {
	// Check for ANSI color codes (0-255)
	if len(color) > 0 && len(color) <= 3 {
		// Could be ANSI code like "1", "15", "255"
		for _, c := range color {
			if c < '0' || c > '9' {
				return false
			}
		}
		// Parse and check range
		if num, err := strconv.Atoi(color); err == nil && num >= 0 && num <= 255 {
			return true
		}
		return false
	}

	// Check for hex color (#RRGGBB or #RGB)
	if len(color) == 7 || len(color) == 4 {
		if color[0] != '#' {
			return false
		}
		for i := 1; i < len(color); i++ {
			c := color[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}

	return false
}

// validateConfig validates all configuration values and returns a list of errors
func validateConfig(cfg *Config) []error {
	var errors []error

	// UI validation
	if cfg.UI.MaxWidth < 20 {
		errors = append(errors, configError{
			field:   "ui.max_width",
			message: fmt.Sprintf("must be >= 20 (got %d)", cfg.UI.MaxWidth),
		})
	}

	if cfg.UI.ColorMode != "manual" && cfg.UI.ColorMode != "auto" {
		errors = append(errors, configError{
			field:   "ui.color_mode",
			message: fmt.Sprintf("must be 'manual' or 'auto' (got '%s')", cfg.UI.ColorMode),
		})
	}

	if !isValidColor(cfg.UI.Color) {
		errors = append(errors, configError{
			field:   "ui.color",
			message: fmt.Sprintf("must be valid ANSI code (0-255) or hex color (#RRGGBB) (got '%s')", cfg.UI.Color),
		})
	}

	// Artwork validation
	if cfg.Artwork.Padding < 0 {
		errors = append(errors, configError{
			field:   "artwork.padding",
			message: fmt.Sprintf("must be >= 0 (got %d)", cfg.Artwork.Padding),
		})
	}

	if cfg.Artwork.Padding >= cfg.UI.MaxWidth {
		errors = append(errors, configError{
			field:   "artwork.padding",
			message: fmt.Sprintf("must be < ui.max_width (%d >= %d)", cfg.Artwork.Padding, cfg.UI.MaxWidth),
		})
	}

	if cfg.Artwork.WidthPixels <= 0 || cfg.Artwork.WidthPixels > 10000 {
		errors = append(errors, configError{
			field:   "artwork.width_pixels",
			message: fmt.Sprintf("must be > 0 and <= 10000 (got %d)", cfg.Artwork.WidthPixels),
		})
	}

	if cfg.Artwork.WidthColumns <= 0 || cfg.Artwork.WidthColumns > 100 {
		errors = append(errors, configError{
			field:   "artwork.width_columns",
			message: fmt.Sprintf("must be > 0 and <= 100 (got %d)", cfg.Artwork.WidthColumns),
		})
	}

	if cfg.Artwork.VinylRPM <= 0 || cfg.Artwork.VinylRPM > 1000 {
		errors = append(errors, configError{
			field:   "artwork.vinyl_rpm",
			message: fmt.Sprintf("must be > 0 and <= 1000 (got %.2f)", cfg.Artwork.VinylRPM),
		})
	}

	// Text validation
	if cfg.Text.MaxLengthWithArt <= 0 || cfg.Text.MaxLengthWithArt > 200 {
		errors = append(errors, configError{
			field:   "text.max_length_with_art",
			message: fmt.Sprintf("must be > 0 and <= 200 (got %d)", cfg.Text.MaxLengthWithArt),
		})
	}

	if cfg.Text.MaxLengthNoArt <= 0 || cfg.Text.MaxLengthNoArt > 200 {
		errors = append(errors, configError{
			field:   "text.max_length_no_art",
			message: fmt.Sprintf("must be > 0 and <= 200 (got %d)", cfg.Text.MaxLengthNoArt),
		})
	}

	// Timing validation
	if cfg.Timing.UIRefreshMs < 10 || cfg.Timing.UIRefreshMs > 5000 {
		errors = append(errors, configError{
			field:   "timing.ui_refresh_ms",
			message: fmt.Sprintf("must be >= 10 and <= 5000 (got %d)", cfg.Timing.UIRefreshMs),
		})
	}

	if cfg.Timing.DataFetchMs < 100 || cfg.Timing.DataFetchMs > 60000 {
		errors = append(errors, configError{
			field:   "timing.data_fetch_ms",
			message: fmt.Sprintf("must be >= 100 and <= 60000 (got %d)", cfg.Timing.DataFetchMs),
		})
	}

	return errors
}

// applyDefaultsForInvalidFields fixes invalid config values with defaults
func applyDefaultsForInvalidFields(cfg *Config, errors []error) {
	for _, err := range errors {
		configErr, ok := err.(configError)
		if !ok {
			continue
		}

		switch configErr.field {
		case "ui.max_width":
			cfg.UI.MaxWidth = 45
		case "ui.color_mode":
			cfg.UI.ColorMode = "auto"
		case "ui.color":
			cfg.UI.Color = "2"
		case "artwork.padding":
			cfg.Artwork.Padding = 16
		case "artwork.width_pixels":
			cfg.Artwork.WidthPixels = 300
		case "artwork.width_columns":
			cfg.Artwork.WidthColumns = 14
		case "artwork.vinyl_rpm":
			cfg.Artwork.VinylRPM = 10.0
		case "text.max_length_with_art":
			cfg.Text.MaxLengthWithArt = 22
		case "text.max_length_no_art":
			cfg.Text.MaxLengthNoArt = 36
		case "timing.ui_refresh_ms":
			cfg.Timing.UIRefreshMs = 100
		case "timing.data_fetch_ms":
			cfg.Timing.DataFetchMs = 1000
		}
	}
}

// printConfigWarnings prints validation errors to stderr with helpful formatting
func printConfigWarnings(errors []error) {
	if len(errors) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "\n⚠️  Configuration validation warnings:\n")
	for _, err := range errors {
		fmt.Fprintf(os.Stderr, "   • %s\n", err.Error())
	}
	fmt.Fprintf(os.Stderr, "   → Using default values for invalid settings\n")
	fmt.Fprintf(os.Stderr, "   → Check ~/.config/goplaying/config.yaml\n\n")
}

// Config file changed notification
type configReloadMsg struct{}

var configChangeChan = make(chan struct{}, 1)

// Watch for config file changes
func watchConfigCmd() tea.Cmd {
	return func() tea.Msg {
		<-configChangeChan
		return configReloadMsg{}
	}
}

func initConfig() {
	// Set defaults
	viper.SetDefault("ui.color", "2")
	viper.SetDefault("ui.color_mode", "auto")
	viper.SetDefault("ui.max_width", 45)
	viper.SetDefault("artwork.enabled", true)
	viper.SetDefault("artwork.padding", 16)
	viper.SetDefault("artwork.width_pixels", 300)
	viper.SetDefault("artwork.width_columns", 14)
	viper.SetDefault("artwork.vinyl_mode", false) // Disabled by default - see config.example.yaml
	viper.SetDefault("artwork.vinyl_rpm", 10.0)   // Slow, dramatic spin when enabled
	viper.SetDefault("text.max_length_with_art", 22)
	viper.SetDefault("text.max_length_no_art", 36)
	viper.SetDefault("timing.ui_refresh_ms", 100)
	viper.SetDefault("timing.data_fetch_ms", 1000)

	// Set config file location following XDG standard
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Check XDG_CONFIG_HOME first, fallback to ~/.config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configHome = filepath.Join(homeDir, ".config")
		}
	}

	if configHome != "" {
		viper.AddConfigPath(filepath.Join(configHome, "goplaying"))
	}

	// Environment variable support with GOPLAYING_ prefix
	viper.SetEnvPrefix("GOPLAYING")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (ignore error if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file found but had errors
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file: %v\n", err)
		}
	}

	// Bind command-line flags (they take precedence)
	if colorFlag != "2" { // Only override if flag was explicitly set
		viper.Set("ui.color", colorFlag)
	}
	if noArtworkFlag {
		viper.Set("artwork.enabled", false)
	}

	// Unmarshal into config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error parsing config: %v\n", err)
	}

	// Validate configuration and fix invalid values
	if validationErrors := validateConfig(&cfg); len(validationErrors) > 0 {
		printConfigWarnings(validationErrors)
		applyDefaultsForInvalidFields(&cfg, validationErrors)
	}

	config.Set(cfg)

	// Watch for config file changes and live reload
	viper.OnConfigChange(func(e fsnotify.Event) {
		var newCfg Config
		if err := viper.Unmarshal(&newCfg); err == nil {
			// Validate the new config
			if validationErrors := validateConfig(&newCfg); len(validationErrors) > 0 {
				// Invalid config - silently keep old config
				// Don't print to stderr during TUI operation as it corrupts the display
				return
			}

			// Valid config - apply it
			config.Set(newCfg)
			// Config reloaded successfully, notify the app
			select {
			case configChangeChan <- struct{}{}:
			default:
				// Channel full, skip notification
			}
		}
	})
	viper.WatchConfig()
}
