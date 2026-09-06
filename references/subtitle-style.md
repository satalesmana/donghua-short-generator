# Subtitle Style Reference

## Current Implementation (2026-09-02)
Word-by-word karaoke-style animation. Each word highlighted individually.

### ASS Style
```
Style: Default,Arial,72,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,1,0,0,0,100,100,0,0,3,6,3,2,30,30,150,1
```
- Font: Arial Bold **72px**
- PrimaryColour: White (`&H00FFFFFF`)
- OutlineColour: Black (`&H00000000`)
- BackColour: Transparent (`&H00000000`) — background handled by FFmpeg `drawbox`
- BorderStyle: 3 (opaque box)
- Outline: 6, Shadow: 3
- **MarginV: 150** (centered in drawbox at y=1640–1920)

### Word-by-Word Color Override Tags
ASS override tags use curly braces: `{\c&HBBGGRR&}text`

| State | Color Code | Visual |
|-------|-----------|--------|
| Previous (spoken) | `{\c&H808080&}` | Dimmed gray |
| Current (speaking) | `{\c&H00FFFF&}` | Bright yellow* |
| Future (not yet) | `{\c&H000000&}` | Hidden (black = matches background bar) |

*Note: `&H00FFFF&` in ASS BGR format = RGB(FF,FF,00) = yellow. For cyan, use `&HFFFF00&`.

### Layout
| Area | Method | Position | Style |
|------|--------|----------|-------|
| Title | `drawtext` filter | Y=50 from top | 52px, Bold, Center, White |
| Video | source | full frame | — |
| Background bar | `drawbox` filter | y=1640, h=280 | `#000000@0.7` (70% opacity) |
| Subtitles | ASS `subtitles` filter | MarginV=150 | 72px, Bold, centered in bar |

### Margins
- MarginL=30, MarginR=30 (horizontal padding)
- MarginV=150 (from bottom, centers text in drawbox y=1640–1920)

## force_style Warning
NEVER use `force_style` in FFmpeg subtitles filter — it overrides ASS file styles and breaks the drawbox background.

## Historical Styles (Removed)
| Version | Font | MarginV | Notes |
|---------|------|---------|-------|
| v1 (2026-08-31) | 28px | 150 | No bold, thin outline |
| v2 (2026-09-01) | 32px | 280 | Bold, semi-transparent bg |
| v3 (2026-09-01) | 44px | 280→118 | BorderStyle=3, drawbox |
| v4 (2026-09-02) | 72px | 150 | Word-by-word karaoke |
