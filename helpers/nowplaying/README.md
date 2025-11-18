# Now Playing Helper for macOS

This Swift helper uses macOS's private MediaRemote framework to get system-wide Now Playing information. This works with **any** audio source on macOS, including:

- Apple Music (including Radio streams)
- Spotify
- YouTube in any browser (Firefox, Safari, Chrome, etc.)
- Any web audio
- Any other media player

## Building

```bash
make
```

This will compile the `nowplaying` binary using Swift.

## Usage

```bash
# Get metadata (title|artist|album|status)
./nowplaying metadata

# Get duration in seconds
./nowplaying duration

# Get current position in seconds
./nowplaying position

# Control playback
./nowplaying play-pause
./nowplaying next
./nowplaying previous
```

## How It Works

The helper dynamically loads the MediaRemote framework (`/System/Library/PrivateFrameworks/MediaRemote.framework`) and calls its functions to get system-wide Now Playing information. This is the same framework macOS uses for the Control Center, keyboard media keys, and lock screen media controls.

## Note

MediaRemote is a private framework, but it's been stable and widely used by third-party apps for years. It provides the most reliable way to get system-wide media information on macOS.
