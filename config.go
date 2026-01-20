package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		Enabled      bool `mapstructure:"enabled"`
		Padding      int  `mapstructure:"padding"`
		WidthPixels  int  `mapstructure:"width_pixels"`
		WidthColumns int  `mapstructure:"width_columns"`
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
	viper.SetDefault("ui.color_mode", "manual")
	viper.SetDefault("ui.max_width", 45)
	viper.SetDefault("artwork.enabled", true)
	viper.SetDefault("artwork.padding", 15)
	viper.SetDefault("artwork.width_pixels", 300)
	viper.SetDefault("artwork.width_columns", 13)
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
	config.Set(cfg)

	// Watch for config file changes and live reload
	viper.OnConfigChange(func(e fsnotify.Event) {
		var newCfg Config
		if err := viper.Unmarshal(&newCfg); err == nil {
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
