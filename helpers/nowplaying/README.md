# Now Playing Helper for macOS

This Swift helper uses macOS's private MediaRemote framework to get Now Playing information from apps that register with the system's media controls. This includes:

- Apple Music (including Radio streams)
- Spotify
- Other music players that implement macOS Now Playing API
- Some browser audio (when websites implement Media Session API)

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

The helper dynamically loads the MediaRemote framework (`/System/Library/PrivateFrameworks/MediaRemote.framework`) and calls its functions to get Now Playing information. This is the same framework macOS uses for Control Center, keyboard media keys, and lock screen media controls.

The main app uses this helper in combination with AppleScript for maximum reliability with Apple Music and Spotify.
