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

This will compile `libnowplaying.dylib` using Swift.

Since macOS 15.4, `mediaremoted` only returns Now Playing info to Apple-signed processes; anything else gets an empty dictionary. The helper is therefore a dylib that goplaying loads into `/usr/bin/perl` (an Apple platform binary), calling its `nowplaying_main` entry point.

## Usage

The dylib must be loaded by absolute path (dyld rejects relative paths in perl's hardened runtime):

```bash
LOADER='use DynaLoader; my $l = DynaLoader::dl_load_file($ARGV[0], 0) or die DynaLoader::dl_error(); my $s = DynaLoader::dl_find_symbol($l, "nowplaying_main") or die DynaLoader::dl_error(); DynaLoader::dl_install_xsub("main::nowplaying_main", $s); main::nowplaying_main();'

# Get metadata (title|artist|album|status)
/usr/bin/perl -e "$LOADER" "$PWD/libnowplaying.dylib" metadata

# Other commands: duration, position, artwork, play-pause, next, previous
```

If `metadata` reports `No media playing (info keys: 0)` while something is playing, MediaRemote is refusing the host process.

## How It Works

The helper dynamically loads the MediaRemote framework (`/System/Library/PrivateFrameworks/MediaRemote.framework`) and calls its functions to get Now Playing information. This is the same framework macOS uses for Control Center, keyboard media keys, and lock screen media controls.

The main app uses this helper in combination with AppleScript for maximum reliability with Apple Music and Spotify.
