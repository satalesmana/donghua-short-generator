# Dao-Clip: Donghua Shorts Generator CLI

CLI tool untuk menghasilkan video Shorts (9:16) dari episode Donghua/Anime Mandarin menggunakan pipeline AI-based.

## 🎯 Features

- **Transcription**: Local Whisper.cpp untuk transkripsi audio Mandarin
- **LLM Analysis**: 9router (Gemini 1.5 Pro) untuk analisis cerita & generation master plan
- **TTS Voiceover**: Fish Audio via OpenRouter untuk voiceover Bahasa Indonesia
- **Video Rendering**: FFmpeg untuk clipping, cropping, concatenation, dan subtitle burning
- **Output**: 1080x1920 vertical video dengan subtitle Indonesia

## 📊 Pipeline Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         DAO-CLIP PIPELINE FLOW                               │
└─────────────────────────────────────────────────────────────────────────────┘

INPUT: workspace/input/episode_01.mp4
         │
         ▼
┌──────────────────┐
│  STEP 1          │  Audio Extraction (FFmpeg)
│  Audio Extract   │  - Extract audio → 16kHz mono WAV
│                  │  - Command: ffmpeg -i input.mp4 -vn -acodec pcm_s16le -ar 16000 -ac 1
│                  │  Output: workspace/temp/episode_01/episode_01_audio.wav
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 2          │  Transcription (Whisper.cpp CLI)
│  Transcribe      │  - Local Whisper.cpp (whisper-cli)
│                  │  - Model: ggml-base.bin
│                  │  - Language: zh (Mandarin)
│                  │  - Output: whisper_result.json
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 3          │  LLM Analysis (9router / Gemini)
│  Master Plan     │  - Analyze transcript + video
│                  │  - Generate: Title, Script Indonesia, Clips, Subtitles
│                  │  - Output: master_plan.json
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 4          │  TTS Voiceover (Fish Audio via OpenRouter)
│  TTS Generate    │  - Model: fish-audio/s2.1-pro-free:free
│                  │  - Voice: Indonesian female
│                  │  - Output: voiceover.mp3
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 5          │  Video Clipping & Cropping
│  Cut & Crop      │  - Cut segments based on Master Plan clips
│                  │  - Crop to 9:16 (1080x1920)
│                  │  - Output: clip_00.mp4, clip_01.mp4, ...
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 6          │  Concatenate Clips
│  Merge Clips     │  - Merge all clips into single video
│                  │  - Output: concatenated.mp4
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  STEP 7          │  Final Render (FFmpeg)
│  Render          │  - Burn subtitles (ASS format)
│                  │  - Add title overlay (drawtext)
│                  │  - Merge audio voiceover
│                  │  - Output: FINAL_SHORTS_episode_01.mp4
└────────┬─────────┘
         │
         ▼
   OUTPUT: workspace/output/FINAL_SHORTS_episode_01.mp4
           (1080x1920, H264+AAC, ~20-30 seconds)
```

## 📁 Struktur File & Directory

```
workspace/
├── input/
│   └── episode_01.mp4              # Input video (source)
├── temp/
│   └── episode_01/
│       ├── episode_01_audio.wav    # Extracted audio (Step 1)
│       ├── whisper_result.json     # Raw transcription (Step 2)
│       ├── transcript.json         # Formatted transcript (Step 2)
│       ├── master_plan.json        # LLM output (Step 3)
│       ├── voiceover.mp3           # TTS audio (Step 4)
│       ├── clip_00.mp4 ... clip_N.mp4  # Clipped segments (Step 5)
│       ├── list.txt                # FFmpeg concat list
│       └── concatenated.mp4        # Merged clips (Step 6)
└── output/
    └── FINAL_SHORTS_episode_01.mp4 # Final output (Step 7)
```

## 🔧 Detail Per Step

### **Step 1: Audio Extraction**
- **Service**: `services/audio_extractor.go`
- **Tools**: FFmpeg CLI
- **Output**: WAV mono 16kHz (format optimal untuk Whisper)
- **Duration**: ~1 detik untuk video 20MB

### **Step 2: Transcription**
- **Service**: `services/transcriber.go`
- **Tools**: `whisper-cli` (local) atau API (fallback)
- **Model**: `ggml-base.bin` (~500MB)
- **Output**: JSON dengan segments (text + timestamp)
- **Duration**: ~2 menit untuk 30 Menit video (local CPU)

### **Step 3: LLM Analysis**
- **Service**: `services/llm_analyzer.go`
- **API**: 9router (Gemini 1.5 Pro)
- **Input**: Transcript + video path
- **Output JSON Schema**:
  ```json
  {
    "title": "Judul Video",
    "script_indonesia": "Narasi voiceover",
    "clips": [{"start_time": "00:06:30", "end_time": "00:06:33", "visual_focus": "..."}],
    "subtitles": [{"start_time": 0, "end_time": 4.5, "text": "..."}]
  }
  ```
- **Duration**: ~20 detik (API call)

### **Step 4: TTS Generation**
- **Service**: `services/tts.go`
- **API**: OpenRouter → Fish Audio
- **Model**: `fish-audio/s2.1-pro-free:free`
- **Voice**: Indonesian female reference
- **Output**: MP3 voiceover
- **Duration**: ~10-15 detik

### **Step 5: Video Clipping**
- **Service**: `services/ffmpeg_renderer.go`
- **Function**: `CutAndCropClip()`
- **Filter**: `crop=ih*(9/16):ih,scale=1080:1920,force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2`
- **Output**: 14-17 clips × 3 detik = ~45-50 detik total

### **Step 6: Concatenation**
- **Service**: `services/ffmpeg_renderer.go`
- **Method**: FFmpeg concat demuxer
- **Input**: `list.txt` (daftar semua clip)
- **Output**: `concatenated.mp4` (single video)

### **Step 7: Final Render**
- **Service**: `services/ffmpeg_renderer.go`
- **Function**: `RenderWithSubtitles()`
- **Filters**:
  1. `drawtext` - Title overlay (Y=50, center)
  2. `subtitles` - ASS subtitles (Y=150, bottom-center)
- **Codec**: H264 + AAC
- **Output**: `FINAL_SHORTS_episode_01.mp4` (~20 detik, 8-10 MB)

## ⏱️ Estimasi Waktu Total

| Step | Proses | Durasi |
|------|--------|--------|
| 1 | Audio Extraction | ~1s |
| 2 | Whisper Transcription | ~120s (local) / ~10s (API) |
| 3 | LLM Analysis | ~20s |
| 4 | TTS Generation | ~15s |
| 5 | Video Clipping | ~30s (15 clips × 2s) |
| 6 | Concatenation | ~25s |
| 7 | Final Render | ~15s |
| **Total** | | **~230 detik (3-4 menit)** |

## 🎯 Key Observations

1. **Bottleneck**: Step 2 (Whisper transcription) - 50% total waktu
2. **Parallelization Opportunity**: Steps 3 & 4 bisa running parallel (LLM & TTS independent)
3. **Memory Usage**: Whisper model ~500MB + video processing temp files
4. **API Dependencies**: 9router (LLM), OpenRouter (TTS), local Whisper (optional API fallback)
5. **Output Quality**: 1080x1920, 30fps, H264+AAC, ~3000-4000 kbps

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- FFmpeg & ffprobe
- Whisper.cpp (`whisper-cli` binary)
- ggml-base.bin model
- API keys untuk 9router & OpenRouter

### Installation

```bash
# Clone repository
git clone <repository-url>
cd dao-clip

# Install dependencies
go mod tidy

# Setup environment
cp .env.example .env
# Edit .env dengan API keys Anda
```

### Usage

```bash
# Basic run
go run main.go -ep=episode_01

# With debug logging
go run main.go -ep=episode_01 -debug

# Clean temp files after completion
go run main.go -ep=episode_01 -clean
```

## ⚙️ Configuration (.env)

```bash
# API Keys
NINEROUTER_API_KEY=***
NINEROUTER_BASE_URL=http://localhost:20128/v1
NINEROUTER_MODEL=FreeModel

# TTS (Fish Audio via OpenRouter)
OPENAI_API_KEY=***
OPENAI_BASE_URL=https://openrouter.ai/api/v1
TTS_VOICE=970f714811ad4dd2b7bc2ffae8627901
TTS_MODEL=fish-audio/s2.1-pro-free:free

# Whisper (Local)
WHISPER_LANG=zh
WHISPER_BINARY=whisper-cli
WHISPER_MODEL_PATH=lib/whisper-models/ggml-base.bin
```

## 📝 Output Format

- **Resolution**: 1080×1920 (9:16 vertical)
- **Codec**: H.264 video + AAC audio
- **FPS**: 30
- **Duration**: 20-30 detik
- **File Size**: 8-15 MB
- **Language**: Mandarin source → Indonesian voiceover & subtitle

## 🔧 Troubleshooting

### Video Output Black/Blank
- Pastikan Remotion tidak digunakan di macOS < 15 (gunakan FFmpeg renderer)
- Check input video format (harus MP4/H264)

### Subtitle Position Wrong
- Pastikan `MarginV=150` untuk bottom alignment
- Gunakan `Alignment=2` untuk center-bottom

### Whisper Transcription Slow
- Gunakan API mode (Groq Whisper) untuk kecepatan 10x
- Atau upgrade model ke `ggml-large.bin` untuk akurasi lebih baik

### TTS Failed
- Check OpenRouter API key valid
- Pastikan voice ID valid untuk Fish Audio

## 🚀 Future Improvements

1. **Parallel Processing**: Jalankan Step 3 (LLM) & Step 4 (TTS) secara parallel
2. **API Whisper**: Integrasi Groq Whisper untuk transkripsi 10x lebih cepat
3. **Batch TTS**: Generate TTS per subtitle, bukan semua sekaligus
4. **Streaming Render**: Render per-clip sambil generate, tidak perlu tunggu semua clip selesai
5. **Multi-language**: Support bahasa source lain (Jepang, Korea)
6. **Cloud Deployment**: Dockerize untuk deployment di cloud

## 📄 License

Private Project - DAO-Clip

---

**Developed by**: Hermes Agent  
**Last Updated**: 2026-08-31
