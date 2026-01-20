# Claude Code Assistant Guide for goplaying

## Development Workflow

### Branch Strategy
**ALWAYS create a new branch for any changes** - features, fixes, refactors, documentation, etc.

Branch naming conventions:
- **Features**: `feat/description` (e.g., `feat/volume-control`)
- **Bug fixes**: `fix/description` (e.g., `fix/redundant-image-decoding`)
- **Refactoring**: `refactor/description` (e.g., `refactor/split-main`)
- **Documentation**: `docs/description` (e.g., `docs/update-readme`)
- **Chores**: `chore/description` (e.g., `chore/update-deps`)

**Process**:
1. Create branch: `git checkout -b <type>/short-description`
2. Make changes and commit
3. Push and create PR when ready
4. Merge to main after review/approval

Never commit directly to main unless explicitly requested.

## Project Overview

**goplaying** is a cross-platform TUI (Terminal User Interface) music player status display written in Go. It shows currently playing music with album artwork, playback controls, and auto-extracted color themes.

- **Platforms**: macOS (MediaRemote + AppleScript), Linux (playerctl/MPRIS)
- **Frameworks**: Bubble Tea (TUI), Lipgloss (styling)
- **Special Features**: Kitty graphics protocol for album artwork, K-means color extraction, live config reload

## Key Files

### Core Application (Modular Architecture)
The application is split into focused modules for better maintainability:

- **main.go** (39 lines): Application entry point
  - Command-line flag definitions
  - Initializes configuration
  - Creates and runs Bubble Tea program
  
- **config.go** (146 lines): Configuration management
  - `Config` struct with Viper configuration management
  - `SafeConfig`: Thread-safe config wrapper with `sync.RWMutex`
  - `initConfig()`: Loads config from `~/.config/goplaying/config.yaml`
  - `watchConfigCmd()`: Live config reload via fsnotify
  - Global `config` variable (use `config.Get()` and `config.Set()` for thread safety)

- **model.go** (335 lines): Bubble Tea model and business logic
  - `SongData` struct: Current track metadata
  - `model` struct: Application state (song data, artwork, scrolling state)
  - `Init()`, `Update()`: Bubble Tea lifecycle methods
  - `fetchSongData()`: Background data fetching from media controller
  - `getCurrentPosition()`: Smooth position interpolation for progress bar
  - Message types: `tickMsg`, `fetchMsg`, `songDataMsg`

- **view.go** (168 lines): UI rendering
  - `View()`: Renders TUI with lipgloss styling
  - Handles artwork display with Kitty graphics protocol
  - Progress bar, scrolling text, help text
  - Dynamic status icons (play/pause/stop)

- **artwork.go** (248 lines): Image processing and color extraction
  - `extractDominantColor()`: K-means color extraction from artwork
  - `encodeArtworkForKitty()`: Converts artwork to Kitty graphics protocol
  - `supportsKittyGraphics()`: Terminal capability detection
  - Handles base64 encoding/decoding, image resizing, chunking

- **text.go** (32 lines): Text utilities
  - `formatTime()`: Converts seconds to MM:SS format
  - `scrollText()`: Unicode-aware text scrolling with looping

### Platform-Specific Media Controllers
- **media_darwin.go**: macOS implementation
  - `HybridController`: MediaRemote framework (via Swift helper) + AppleScript fallback
  - Artwork via temp file (AppleScript limitation)
  - Supports: Music, Spotify, any Now Playing source
  
- **media_linux.go**: Linux implementation
  - Uses `playerctl` command-line tool
  - MPRIS D-Bus protocol support
  - Artwork via `mpris:artUrl` metadata

- **media.go**: Interface definition
  - `MediaController` interface for cross-platform abstraction

### macOS Swift Helper
- **helpers/nowplaying/main.swift**: MediaRemote private framework wrapper
  - Returns base64-encoded artwork data
  - Required for broader app support beyond Music/Spotify
  - Build with: `cd helpers/nowplaying && make`
  - Requires: Xcode Command Line Tools (`xcode-select --install`)

### Configuration
- **config.example.yaml**: Configuration template
  - `ui.color`: Manual color (ANSI or hex)
  - `ui.color_mode`: "manual" or "auto" (extract from artwork)
  - `ui.max_width`: Border width
  - `artwork.enabled`: Toggle artwork display
  - `artwork.padding`: Space for artwork (columns)
  - `text.max_length_with_art` / `text.max_length_no_art`: Scrolling text width
  - `timing.ui_refresh_ms`: UI tick rate (100ms default)
  - `timing.data_fetch_ms`: Metadata fetch rate (1000ms default)
  
- Config location: `~/.config/goplaying/config.yaml`
- Live reload: Changes apply immediately via fsnotify

### Build System
- **Makefile**: Build targets
  - `make` or `make goplaying`: Build main binary
  - `make darwin`: Build main + Swift helper (macOS)
  - `make linux`: Build main binary only
  - `make fmt`: Run gofmt + goimports
  - `make lint`: Run golangci-lint
  - `make test`: Run tests
  
- **.golangci.yml**: Linter configuration
  - Enabled: errcheck, gofmt, goimports, govet, staticcheck, unused, gosimple

## Architecture Patterns

### Data Flow
1. **Timer loops**: Two concurrent tick loops (UI refresh 100ms, data fetch 1000ms)
2. **Background fetching**: `fetchSongData()` returns `tea.Cmd` for non-blocking I/O
3. **Track caching**: Artwork only fetched when track ID (title|artist) changes
4. **Smooth interpolation**: Position calculated client-side between fetches for fluid progress bar

### Color Modes
- **Manual**: Uses `config.UI.Color` always
- **Auto**: 
  - Extracts dominant color via K-means when track changes
  - Falls back to manual color on initial load (before artwork)
  - Persists extracted color until next track (doesn't revert on fetch)

### Artwork Flow
1. Media controller retrieves artwork (base64 or raw bytes)
2. `extractDominantColor()`: Decode base64 → image.Decode → prominentcolor.Kmeans
3. `encodeArtworkForKitty()`: Decode base64 → resize to 300px → PNG encode → base64 → chunk → Kitty protocol
4. Kitty protocol: Escape sequences with image ID 42, cell-based sizing (c=13 columns)

### Text Scrolling
- Unicode-aware (rune-based, not byte-based)
- Scrolls every 3rd tick (300ms) for readability
- Pauses 30 ticks (3s) at start/end of text
- Adds " • " separator for smooth looping

## Common Tasks

### Adding Configuration Options
1. Add field to `Config` struct in config.go with mapstructure tag
2. Set default in `initConfig()` with `viper.SetDefault()`
3. Add to config.example.yaml
4. Access via `config.Get().SectionName.FieldName`

### Platform-Specific Code
- Use build tags: `//go:build darwin` or `//go:build linux`
- Implement `MediaController` interface methods
- Test on both platforms before merging

### Debugging Tips
- Use `fmt.Fprintf(os.Stderr, "Debug: ...\n")` for logging (doesn't disrupt TUI)
- Run with `2> debug.log` to capture stderr
- Check terminal env vars: `echo $TERM $TERM_PROGRAM`
- Test Kitty protocol manually: `printf '\033_Ga=T,f=100,t=d,i=99;%s\033\\' "$(base64 < image.png)"`

### Before Committing
```bash
make fmt    # Format code
make lint   # Check for issues
make test   # Run tests
go build    # Verify compilation
```

## Release Process

### Overview
The project uses automated releases via GitHub Actions + GoReleaser. Pushing a version tag triggers builds for all platforms and updates distribution channels.

### Version Numbering
- **Format**: `v0.MINOR.PATCH` (0-based versioning during development)
- **Examples**: v0.2.0, v0.2.1, v0.2.2
- **When to bump**:
  - MINOR: New features, significant changes
  - PATCH: Bug fixes, small improvements

### Release Workflow

#### 1. Verify Changes are Ready
```bash
# Check recent commits
git log --oneline -5

# Verify clean working tree
git status

# Ensure main branch is up to date
git pull origin main
```

#### 2. Create and Push Release Tag
```bash
# Create annotated tag with release notes
git tag -a v0.2.X -m "Release v0.2.X - Brief description

New features:
- Feature description

Bug fixes:
- Fix description

UX improvements:
- Improvement description"

# Push tag to trigger GitHub Actions
git push origin v0.2.X
```

#### 3. Monitor GitHub Actions Build
```bash
# Watch the workflow
gh run list --limit 1
gh run watch <run-id>

# Or check on GitHub
# https://github.com/justinmdickey/goplaying/actions
```

**Expected results**:
- Build completes in ~1m20s
- All platform binaries created (Linux/macOS, x86_64/arm64)
- GitHub Release published with assets
- Homebrew formula auto-updated

#### 4. Verify Release
```bash
# Check release was created
gh release view v0.2.X

# Verify Homebrew formula updated
gh api repos/justinmdickey/homebrew-tap/contents/Formula/goplaying.rb | jq -r '.content' | base64 -d | head -10
```

#### 5. Update AUR Package (goplaying-git)

**Get version info**:
```bash
# In main repo
git rev-list --count HEAD  # Get commit count (e.g., 84)
git rev-parse --short HEAD # Get short hash (e.g., 645ec14)
```

**Update AUR package**:
```bash
cd ~/Dev/aur/goplaying-git

# Edit PKGBUILD - update pkgver line
# pkgver=r84.645ec14

# Regenerate .SRCINFO
makepkg --printsrcinfo > .SRCINFO

# Verify
cat .SRCINFO | head -5

# Commit and push to AUR
git add PKGBUILD .SRCINFO
git commit -m "Update to r84.645ec14 - Brief description"
git push origin master
```

**Note**: AUR updates appear within 1-2 minutes. Users see updates via `yay -Syu`.

### Distribution Channels

#### Homebrew (Auto-updated)
- **Tap**: `justinmdickey/tap/goplaying`
- **Formula**: Auto-updated by GoReleaser on tag push
- **Users install**: `brew install justinmdickey/tap/goplaying`
- **Users upgrade**: `brew upgrade goplaying`

#### AUR (Manual update)
- **Package**: `goplaying-git` (VCS package, tracks git HEAD)
- **Maintainer**: justinmdickey
- **Update**: Manual PKGBUILD version bump + push
- **Users install**: `yay -S goplaying-git`
- **Users upgrade**: `yay -Syu`

#### GitHub Releases (Auto)
- **Assets**: Binaries for Linux/macOS (x86_64/arm64)
- **Format**: `.tar.gz` and `.zip` archives
- **Checksums**: `checksums.txt` included
- **Created by**: GoReleaser via GitHub Actions

### Release Checklist

Before tagging a release:
- [ ] All changes committed and pushed to main
- [ ] Code formatted (`make fmt`)
- [ ] Tests passing (`make test`)
- [ ] README updated if needed
- [ ] Version number decided (MINOR vs PATCH)

After tagging:
- [ ] GitHub Actions workflow completes successfully
- [ ] GitHub Release created with all assets
- [ ] Homebrew formula shows new version
- [ ] AUR package updated and pushed
- [ ] Test install from at least one channel

### Common Release Commands

```bash
# Quick release flow
git tag -a v0.2.X -m "Release notes" && git push origin v0.2.X
gh run watch $(gh run list --limit 1 --json databaseId -q '.[0].databaseId')

# Update AUR
cd ~/Dev/aur/goplaying-git
# Update pkgver in PKGBUILD
makepkg --printsrcinfo > .SRCINFO
git add PKGBUILD .SRCINFO && git commit -m "Update to rXX.XXXXXXX" && git push origin master

# Check versions
gh release view v0.2.X
yay -Si goplaying-git  # Check AUR
```

### Troubleshooting

**GitHub Actions fails**:
- Check workflow logs: `gh run view <run-id>`
- Common issues: GoReleaser config syntax, missing secrets

**Homebrew not updating**:
- Check GoReleaser config: `.goreleaser.yml`
- Verify tap repo: `justinmdickey/homebrew-tap`

**AUR update not showing**:
- AUR API cache refreshes in 1-2 minutes
- Verify push succeeded: `cd ~/Dev/aur/goplaying-git && git log -1`
- Check AUR web: https://aur.archlinux.org/packages/goplaying-git

## Known Issues & Gotchas

### macOS Artwork
- Swift helper must be compiled (`make darwin`)
- AppleScript fallback uses temp files (raw data doesn't work via osascript stdout)
- MediaRemote returns base64, AppleScript returns raw bytes (code handles both)

### Terminal Support
- Kitty graphics: Requires `TERM` containing "kitty" or `TERM_PROGRAM` = "ghostty"/"WezTerm"
- Not all terminals support graphics protocol
- Gracefully degrades to text-only mode

### Color Extraction
- Artwork data is base64-encoded on macOS (MediaRemote/playerctl)
- Must decode before passing to `image.Decode()`
- K-means on 300px image balances speed vs accuracy
- Silent failures return empty string (should fall back to manual color)

### Track Caching
- Track ID = `title|artist` (not unique for compilations with same artist)
- Artwork only fetched on track change to avoid redundant processing
- First load: manual color → switches to auto when artwork loads
- Color persists for entire track (doesn't revert on subsequent fetches)

## Dependencies

### Go Modules
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/spf13/viper` - Configuration management
- `github.com/fsnotify/fsnotify` - File watching for live reload
- `github.com/EdlinOrg/prominentcolor` - K-means color extraction
- `github.com/nfnt/resize` - Image resizing
- `golang.org/x/image/webp` - WebP image support

### System Dependencies
- **macOS**: Swift compiler (optional but recommended)
- **Linux**: playerctl (`sudo apt install playerctl` or equivalent)

## Testing Strategy

### Manual Testing Checklist
- [ ] Artwork displays in Kitty/Ghostty/WezTerm
- [ ] Auto color mode extracts colors and persists per track
- [ ] Manual color mode uses config color
- [ ] Config live reload works (edit ~/.config/goplaying/config.yaml)
- [ ] Text scrolling smooth for long titles
- [ ] Playback controls work (p/n/b/a/? keys)
- [ ] Artwork toggle (a key) works
- [ ] Help toggle (? key) shows/hides keybinds
- [ ] Dynamic status icons change (play/pause/stop)
- [ ] "Nothing playing" state shows friendly message
- [ ] Works with different players (Music, Spotify, browser on macOS; various MPRIS on Linux)
- [ ] Graceful degradation without artwork support

### Edge Cases to Consider
- Empty/missing artwork
- Very long track names (100+ characters)
- Rapid track skipping
- Config file deleted while running
- Terminal resize during playback
- No active media player

## Code Style

- Use `gofmt` and `goimports` (run via `make fmt`)
- Keep functions focused and under 50 lines when possible
- Document complex logic with comments
- Platform-specific code in separate files with build tags
- Error handling: Return errors up, handle at boundaries
- Prefer lipgloss styles over raw ANSI codes

## Future Enhancement Ideas

- Volume control
- Playlist view
- Save favorite tracks
- Notification support
- Web remote control
- Plugin system for data sources
- Custom key bindings
- Multiple color extraction algorithms
- Artwork caching to disk
- History/stats tracking
