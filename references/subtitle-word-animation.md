# Subtitle Word-by-Word Animation Implementation

## Overview
Changed subtitle rendering from "whole sentence" to "per-word" with karaoke-style highlight animation, matching the reference video style.

## Changes Made

### 1. `writeASSFile()` - Core Logic Rewrite
- **Before**: Generated one `Dialogue` per subtitle block (whole sentence at once)
- **After**: Generates one `Dialogue` per **word** within each subtitle block

### 2. New: `buildWordAnimationText()`
Creates ASS-formatted text with per-word color codes:
- **Previous words** (already spoken): `\c&H808080&` (dimmed gray)
- **Current word** (being spoken): `\c&H00FFFF&` (bright cyan highlight)
- **Future words** (not yet spoken): `\c&H000000&` (hidden/black)

### 3. Style Updates
- Font size: `44` → `72` (larger, more prominent like reference)
- `MarginV`: `118` → `150` (repositioned for visual center)
- Outline: `5` → `6` (thicker for readability)

## How It Works

```
Original subtitle: "Xiao Yan Kembali Bertemu"
Duration: 4 seconds

Word-by-word generation:
- t=0s-1s:  \c&H00FFFF&Xiao \c&H000000&Yan \c&H000000&Kembali \c&H000000&Bertemu
- t=1s-2s:  \c&H808080&Xiao \c&H00FFFF&Yan \c&H000000&Kembali \c&H000000&Bertemu
- t=2s-3s:  \c&H808080&Xiao \c&H808080&Yan \c&H00FFFF&Kembali \c&H000000&Bertemu
- t=3s-4s:  \c&H808080&Xiao \c&H808080&Yan \c&H808080&Kembali \c&H00FFFF&Bertemu
- t=4s-5s:  \c&H808080&Xiao \c&H808080&Yan \c&H808080&Kembali \c&H808080&Bertemu (all shown)
```

## Files Modified
- `services/ffmpeg_renderer.go` (lines 283-378)

## Build Status
✅ `go build` - Success
✅ `go vet` - No issues

## Next Steps
To test with real data:
```bash
go run main.go -ep=episode_01 -clean
```

## Notes
- ASS `\c` color tags are supported by FFmpeg's `subtitles` filter
- The hide effect (`\c&H000000&`) works by matching text color to background (black on dark bar)
- Word duration is calculated as: `totalDuration / wordCount`
- Previous word highlighting is cumulative (words stay gray after being spoken)
