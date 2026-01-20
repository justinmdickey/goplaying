# 🎵 Vinyl Mode Easter Egg

A hidden feature that makes your album artwork spin like a real vinyl record!

## What is it?

When enabled, vinyl mode transforms your album artwork into a spinning record that:
- **Rotates continuously** when music is playing
- **Stops spinning** when paused or stopped
- Shows a **"33⅓ RPM"** indicator with animated spinner
- Syncs perfectly with playback state

## How to enable

Add `vinyl_mode: true` to your artwork configuration:

```yaml
artwork:
  enabled: true
  vinyl_mode: true  # Enable the spinning vinyl effect!
  padding: 15
  width_pixels: 300
  width_columns: 13
```

## Pro tips

1. **Use with auto color mode** for the best visual experience:
   ```yaml
   ui:
     color_mode: "auto"
   ```

2. **Smooth animation** requires a reasonable UI refresh rate:
   ```yaml
   timing:
     ui_refresh_ms: 100  # Default, works great
   ```

3. **Works best** with square or circular album artwork

## Technical details

- Uses 8-frame rotation animation
- Braille Unicode characters create spinning effect
- Minimal performance impact (just animation frames)
- Respects playback state automatically

## Why is this an easter egg?

Because not everyone wants their terminal to look like a record player! This feature is:
- Not documented in the main README or example config
- Opt-in only (off by default)
- A fun surprise for users who explore the config options
- Perfect for those who appreciate retro aesthetics

Enjoy your spinning vinyl! 🎶
