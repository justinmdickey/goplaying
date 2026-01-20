# goplaying - Roadmap & TODO

This document tracks planned improvements, features, and known issues for goplaying. Items are organized by priority, with effort estimates and impact ratings to help with planning.

## Legend

**Priority Levels:**
- 🔴 **Critical** - Bug fixes and reliability issues (fix ASAP)
- 🟠 **High** - Important improvements for next releases
- 🟡 **Medium** - Nice-to-have features and optimizations
- 🟢 **Low** - Polish and quality-of-life improvements
- 🔵 **Future** - Exploratory ideas without concrete timeline

**Status:**
- [ ] Todo
- [x] Completed

**Effort:** Time estimate (S=<2hr, M=2-6hr, L=6-12hr, XL=>12hr)  
**Impact:** User/developer benefit (High/Medium/Low)

---

## 🔴 Critical Priority

### Reliability & Bug Fixes

- [x] **Fix config race condition** - Global config accessed by multiple goroutines without synchronization (Effort: M, Impact: High) ✅
  - Wrap in `sync.RWMutex` or use `atomic.Value`
  - Potential crashes on config reload
  - Files: `main.go`
  - **Completed**: PR #32 merged, includes unit tests with race detector

- [ ] **Improve MediaRemote failure handling (macOS)** - Single failure permanently disables better/faster method (Effort: S, Impact: Medium)
  - Add retry counter + time-based reset (retry after 5 min)
  - Currently falls back to slower AppleScript permanently
  - Files: `media_darwin.go`

- [ ] **Add config validation** - Prevent invalid config values causing runtime errors (Effort: M, Impact: High)
  - Validate: negative values, padding > max_width, unreasonable refresh rates
  - Show clear error messages on startup
  - Files: `config.go`

---

## 🟠 High Priority

### Code Quality & Performance

- [x] **Split main.go into modules** - Break 929-line file into focused modules (Effort: M, Impact: High) ✅
  - Created: `config.go`, `model.go`, `view.go`, `artwork.go`, `text.go`
  - Improves maintainability and testability
  - Files: `main.go` (39 lines) + 5 new modular files
  - **Completed**: PR #33 merged, 95% reduction in main.go size

- [x] **Fix redundant image decoding** - Artwork decoded twice (color + Kitty encoding) (Effort: S, Impact: High) ✅
  - Decode once, pass `image.Image` to both functions
  - 50% reduction in image processing time on track changes
  - Files: `artwork.go`, `model.go`
  - **Completed**: Created `decodeArtworkData()` helper and `processArtwork()` convenience function

- [ ] **Replace bubble sort with sort.Slice()** - Color candidate sorting uses O(n²) algorithm (Effort: S, Impact: Low)
  - 2-line fix: use `sort.Slice()` from stdlib
  - Better performance with many color candidates
  - Files: `artwork.go` (extractDominantColor function)

- [ ] **Better error wrapping** - Generic errors without context make debugging hard (Effort: S, Impact: Medium)
  - Use `fmt.Errorf(...: %w, err)` throughout
  - Add stderr logging for silent failures
  - Files: `media_darwin.go`, `media_linux.go`, `model.go`, `artwork.go`

### User-Facing Features

- [ ] **Volume control** - Adjust volume with keybinds (+ / - keys) (Effort: M, Impact: High)
  - Platform-specific: osascript (macOS), playerctl/pactl (Linux)
  - Show volume percentage in UI
  - Files: `model.go`, `view.go`, `media_darwin.go`, `media_linux.go`

- [ ] **Seek/scrub support** - Skip forward/back by N seconds (→ / ← keys) (Effort: M, Impact: High)
  - Platform-specific commands for position control
  - Configurable seek interval (default 5-10 seconds)
  - Files: `model.go`, `media_darwin.go`, `media_linux.go`

- [ ] **Add unit tests** - Limited test coverage currently (Effort: M, Impact: High)
  - Priority: `scrollText()`, `extractDominantColor()`, `formatTime()`, `getCurrentPosition()`
  - Add benchmark tests for performance-critical functions
  - Files: New `text_test.go`, `artwork_test.go`, `model_test.go`
  - Note: `config_test.go` already exists with SafeConfig tests

---

## 🟡 Medium Priority

### Performance Optimizations

- [ ] **Move color extraction to background goroutine** - Blocks UI thread during track changes (Effort: S, Impact: Medium)
  - Extract in goroutine with 100ms timeout
  - Smoother track transitions
  - Files: `model.go` (fetchSongData function)

- [ ] **Cache text length calculation** - Recalculates on every UI tick (100ms) (Effort: S, Impact: Low)
  - Cache max length, recalculate only on track change
  - Minor CPU savings
  - Files: `model.go` (Update function, tickMsg case)

- [ ] **Extract shared command execution helper** - Duplicate exec.Command patterns (Effort: S, Impact: Low)
  - Create `media_common.go` with `runCommand()` helper
  - Reduce code duplication (~50 lines saved)
  - Files: New `media_common.go`, `media_darwin.go`, `media_linux.go`

### User Features

- [ ] **Like/favorite tracks** - Mark track as favorite, save to file (Effort: S, Impact: Medium)
  - Simple JSON/YAML storage in config dir
  - Show indicator in UI for favorited tracks
  - Keybind: `f` to toggle favorite
  - Files: `model.go`, `view.go`, new `favorites.go`

- [ ] **Notification support** - Desktop notifications on track change (Effort: M, Impact: Medium)
  - Platform-specific: terminal-notifier/osascript (macOS), notify-send (Linux)
  - Config option: `notifications.enabled: true`
  - Files: `model.go`, `media_darwin.go`, `media_linux.go`

- [ ] **Artwork caching to disk** - Cache artwork locally, skip redownload (Effort: M, Impact: Medium)
  - Storage: `~/.cache/goplaying/artwork/`
  - Key: Hash of track ID or artwork URL
  - Faster loading, less bandwidth
  - Files: New `cache.go`, `model.go`

- [ ] **Custom key bindings** - User-configurable keybinds (Effort: M, Impact: Medium)
  - Config section: `keybinds.play_pause: "space"`
  - Allow remapping all controls
  - Files: `config.go`, `model.go`

- [ ] **Repeat/shuffle toggles** - Show and toggle repeat/shuffle modes (Effort: M, Impact: Medium)
  - Keybinds: `r` (repeat), `s` (shuffle)
  - Fetch current state from player
  - Show indicators in UI
  - Files: `model.go`, `view.go`, `media_darwin.go`, `media_linux.go`

---

## 🟢 Low Priority

### Code Quality

- [ ] **Document magic numbers** - Add const declarations with comments (Effort: S, Impact: Low)
  - Examples: scroll rate, pause duration, sampling rate, score coefficients
  - Improves code clarity
  - Files: `model.go`, `artwork.go`, `text.go`

- [ ] **Add debug mode** - Flag `--debug` for verbose logging (Effort: S, Impact: Low)
  - Log: player commands, timing info, artwork processing
  - Output to stderr or file
  - Easier issue diagnosis
  - Files: `main.go`, `model.go`, `media_darwin.go`, `media_linux.go`

- [ ] **Add benchmarking script** - Test performance with various configs (Effort: S, Impact: Low)
  - Measure: startup time, color extraction, rendering FPS
  - Track performance regressions
  - Files: New `scripts/benchmark.sh`, `*_test.go`

### Features

- [ ] **Mini mode** - Compact single-line display (Effort: S, Impact: Low)
  - Flag `--mini` or config `ui.mode: "mini"`
  - Use case: status bar integration, tmux/screen
  - Files: `main.go`, `view.go`

- [ ] **Playback history/stats** - Track listening history, show stats (Effort: M, Impact: Medium)
  - Storage: SQLite or JSON log
  - Features: most played, recent tracks, total play time
  - Optional UI panel to view stats
  - Files: New `history.go`, `stats.go`

- [ ] **Multiple color extraction modes** - More algorithms beyond vibrant (Effort: M, Impact: Low)
  - Options: vibrant (current), muted/pastel, complementary, accent only
  - Config: `ui.color_strategy: "vibrant"`
  - Files: `artwork.go`

---

## 🔵 Future / Ideas

### Major Features (Exploratory)

- [ ] **Playlist view** - Show current playlist/queue, navigate tracks (Effort: XL, Impact: High)
  - New UI state with scrolling
  - Challenge: MPRIS doesn't expose playlists well
  - Major feature with significant complexity

- [ ] **Lyrics display** - Show synchronized lyrics if available (Effort: XL, Impact: High)
  - Options: Musixmatch API, tag metadata, local .lrc files
  - Sync lyrics to playback position
  - Requires lyrics source/API

- [ ] **Windows support** - Port to Windows using WinRT APIs (Effort: XL, Impact: High)
  - Different media APIs, no playerctl equivalent
  - Significant platform-specific work
  - Cross-platform completeness

- [ ] **Plugin system for data sources** - Allow custom media controller plugins (Effort: XL, Impact: Medium)
  - Dynamic loading of media backends
  - Support for web APIs (Last.fm, Spotify Web API, etc.)
  - Major architectural change

- [ ] **Web remote control** - Control playback from browser/mobile (Effort: XL, Impact: Medium)
  - Embedded HTTP server
  - WebSocket for real-time updates
  - Mobile-friendly web UI

---

## ✅ Recently Completed

### v0.3.0 (2026-01-20)
- [x] **Split main.go into modules** - Complete refactoring into focused modules
  - Reduced main.go from 929 lines to 39 lines (95% reduction)
  - Created: `config.go`, `model.go`, `view.go`, `artwork.go`, `text.go`
  - Each module has single, clear responsibility
  - Dramatically improved maintainability and testability

### v0.2.4 (2026-01-20)
- [x] **Configurable artwork size** - Made image dimensions configurable
  - Added `artwork.width_pixels` and `artwork.width_columns` config options
  - Live-reloadable with helpful sizing tips

### v0.2.3 (2026-01-17)
- [x] **Dynamic status icons** - Icons change based on playback state (play/pause/stop)
- [x] **Case-insensitive status matching** - Better cross-platform compatibility

### v0.2.2 (2026-01-16)
- [x] **Artwork toggle keybind** - Press `a` to toggle artwork on/off
- [x] **Toggleable help text** - Press `?` to show/hide keybinds
- [x] **Compact UI mode** - Default minimal, full help on demand

### v0.2.1 (2026-01-15)
- [x] **Friendly "Nothing playing" state** - Better UX when no music is playing
- [x] **Clear stale artwork** - Remove artwork when player stops

### v0.2.0 (2026-01-14)
- [x] **Album artwork support** - Kitty graphics protocol integration
- [x] **Auto color extraction** - Extract vibrant colors from artwork
- [x] **Live config reload** - Changes apply immediately via fsnotify

---

## Contributing

Interested in working on any of these? Here's how to get started:

1. **Check for related issues** - Some TODO items may have tracking issues with more details
2. **Discuss first** - For major features, open a discussion or issue before implementing
3. **Follow conventions** - See [CLAUDE.md](CLAUDE.md) for architecture patterns and code style
4. **Test thoroughly** - Manual testing checklist in CLAUDE.md, automated tests appreciated
5. **Update docs** - Update README.md and this TODO.md when adding features

See [CLAUDE.md](CLAUDE.md) for detailed development guide, including:
- Architecture patterns (Bubble Tea MVC)
- Platform-specific code guidelines
- Release process
- Testing strategies

---

## Questions or Suggestions?

Have ideas for new features or found issues not listed here? 

- **Bugs**: Open an issue on GitHub
- **Features**: Start a discussion to gauge interest
- **Questions**: Check CLAUDE.md first, then open an issue

This roadmap is a living document - priorities may shift based on user feedback and contributions!
