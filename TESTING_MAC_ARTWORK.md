# Testing Album Artwork on macOS

## Current Issue
Album artwork is not displaying on macOS even with config set to show it.

## Debug Steps

### 1. Build and run with debug output
```bash
# On your Mac:
make darwin  # This builds both the Swift helper and main binary
./goplaying 2> /tmp/goplaying_debug.log
```

### 2. Check the debug log
```bash
cat /tmp/goplaying_debug.log
```

Look for these messages:
- `Found nowplaying helper at: <path>` - Helper was found ✅
- `Warning: nowplaying helper not found` - Helper missing ❌
- `MediaRemote artwork fetch failed` - Helper ran but failed
- `Attempting to fetch artwork via AppleScript` - Fallback method
- `AppleScript artwork fetch failed` - AppleScript also failed

### 3. Test the Swift helper directly
```bash
# Test if the helper can get artwork
./helpers/nowplaying/nowplaying artwork
# or if built in same directory:
./nowplaying artwork
```

Should output base64-encoded artwork data. If it outputs nothing or errors, the helper has issues.

### 4. Test AppleScript directly
```bash
osascript -e 'tell application "Music" to get raw data of artwork 1 of current track'
# or for Spotify:
osascript -e 'tell application "Spotify" to get raw data of artwork 1 of current track'
```

## Common Issues

### Helper not built
**Symptom**: `Warning: nowplaying helper not found`

**Solution**: 
```bash
cd helpers/nowplaying
make
cd ../..
# Or just run: make darwin
```

### Helper lacks permissions
**Symptom**: `MediaRemote artwork fetch failed: ... operation not permitted`

**Solution**: macOS may need special permissions. Try granting Full Disk Access to Terminal in System Settings.

### Wrong player
**Symptom**: Artwork works for Music but not Spotify (or vice versa)

**Issue**: The helper gets artwork from whatever is currently in "Now Playing". Make sure the right app is actually playing.

### Kitty protocol not working
**Symptom**: Debug log shows artwork fetched but not displayed

**Check**:
```bash
echo $TERM
echo $TERM_PROGRAM
```
Should be `xterm-kitty` or similar, or `TERM_PROGRAM` should be `ghostty` or `kitty`.

## Expected Behavior

When working correctly, you should see:
1. `Found nowplaying helper at: ./helpers/nowplaying/nowplaying`
2. Either artwork appears via MediaRemote OR
3. `Attempting to fetch artwork via AppleScript from Music` (or Spotify)
4. `AppleScript artwork saved to /tmp/goplaying-artwork-*.png`
5. Album artwork displays in the TUI

## Next Steps

After running the debug build, share:
1. Contents of `/tmp/goplaying_debug.log`
2. Output of `./nowplaying artwork` (or `./helpers/nowplaying/nowplaying artwork`)
3. Terminal emulator being used (Ghostty or Kitty)
4. Music player being used (Music.app, Spotify, browser, etc.)
