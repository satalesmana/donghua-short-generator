package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/nousresearch/donghua-shorts-generator/config"
	"github.com/nousresearch/donghua-shorts-generator/services"
)

// Auto-load .env file at startup
func init() {
	// Manual .env loading (more reliable than godotenv)
	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				// Remove quotes if present
				value = strings.Trim(value, "\"'")
				os.Setenv(key, value)
			}
		}
		logrus.Info("📄 .env file loaded manually")
	} else {
		logrus.Warnf("Warning: failed to load .env file: %v", err)
	}
	// Debug: log loaded values
	logrus.Infof("📄 NINEROUTER_API_KEY loaded: %q", os.Getenv("NINEROUTER_API_KEY"))
	logrus.Infof("📄 OPENAI_API_KEY loaded: %q", os.Getenv("OPENAI_API_KEY"))
	logrus.Infof("📄 TTS_API_KEY loaded: %q", os.Getenv("TTS_API_KEY"))
}

func main() {
	// Parse CLI flags
	episodeID := flag.String("ep", "", "Episode ID (e.g., episode_01)")
	clean := flag.Bool("clean", false, "Clean up temp files after generation")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Setup logging
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Validate required flag
	if *episodeID == "" {
		fmt.Fprintln(os.Stderr, "Error: -ep flag is required (e.g., -ep=episode_01)")
		os.Exit(1)
	}

	logrus.Info("🚀 Donghua Shorts Generator CLI (Dialogue-Based Pipeline)")
	logrus.Infof("Episode ID: %s", *episodeID)
	logrus.Infof("Clean temp: %v", *clean)

	// Load configuration
	cfg := config.LoadConfig()

	// Validate API keys
	if cfg.NinerouterAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: NINEROUTER_API_KEY environment variable is not set")
		fmt.Fprintf(os.Stderr, "Current env NINEROUTER_API_KEY: %q\n", os.Getenv("NINEROUTER_API_KEY"))
		os.Exit(1)
	}
	if cfg.TTSAPIKey == "" && cfg.OpenAIAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: TTS_API_KEY or OPENAI_API_KEY environment variable is not set")
		os.Exit(1)
	}

	// Initialize workspace
	ws := services.NewWorkspace(cfg.WorkspaceDir, *episodeID)
	if err := ws.InitWorkspace(); err != nil {
		logrus.Fatalf("Failed to initialize workspace: %v", err)
	}
	logrus.Info("✅ Workspace initialized")

	// Verify input video exists
	if err := ws.EnsureInputVideoExists(); err != nil {
		logrus.Fatalf("❌ %v", err)
	}

	// Resolve system prompt path
	promptPath := filepath.Join(".", "templates", "system_prompt.txt")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		promptPath = filepath.Join("templates", "system_prompt.txt")
	}

	// Initialize services
	audioExtractor := services.NewAudioExtractorService()
	transcriber := services.NewTranscriberService(cfg.OpenAIAPIKey, cfg.TranscriptionModel, cfg.OpenAIBaseURL, cfg.TranscriptionLang, cfg.HTTPTimeout, cfg.WhisperBinary, cfg.WhisperModelPath, ws.EpisodeTemp)
	llmAnalyzer := services.NewLLMAnalyzerService(cfg.NinerouterAPIKey, cfg.NinerouterBaseURL, cfg.NinerouterModel, cfg.LLMMaxTokens, cfg.LLMTemperature, cfg.HTTPTimeout)
	tts := services.NewTTSService(cfg.TTSAPIKey, cfg.TTSBaseURL, cfg.TTSModel, cfg.TTSVoice, cfg.HTTPTimeout)
	ffmpegRenderer := services.NewFFmpegRendererService()

	// =========================================================
	// STEP 1: Audio Extraction (FFmpeg)
	// =========================================================
	logrus.Info("=== STEP 1: Audio Extraction (FFmpeg) ===")
	audioFile := ws.AudioFile()
	if err := audioExtractor.ExtractAudio(ws.InputFile(), audioFile); err != nil {
		logrus.Fatalf("❌ Audio extraction failed: %v", err)
	}
	logrus.Infof("✅ Audio extracted: %s", audioFile)

	// =========================================================
	// STEP 2: Transcription (Whisper.cpp CLI)
	// =========================================================
	logrus.Info("=== STEP 2: Audio Transcription (Whisper.cpp) ===")
	transcript, err := transcriber.TranscribeAudio(audioFile)
	if err != nil {
		logrus.Fatalf("❌ Transcription failed: %v", err)
	}
	if err := transcriber.SaveTranscriptJSON(transcript, ws.TranscriptFile()); err != nil {
		logrus.Fatalf("❌ Failed to save transcript: %v", err)
	}
	logrus.Infof("✅ Transcription complete: %d segments", len(transcript.Segments))

	// =========================================================
	// STEP 3: LLM Analysis & Master Plan Generation
	// =========================================================
	logrus.Info("=== STEP 3: LLM Analysis & Master Plan ===")
	masterPlan, err := llmAnalyzer.GenerateMasterPlan(ws.TranscriptFile(), promptPath, ws.InputFile())
	if err != nil {
		logrus.Fatalf("❌ Master plan generation failed: %v", err)
	}

	// Generate subtitle segments from script to ensure 100% match with voiceover
	// This ensures text and voiceover are always synchronized
	masterPlan.Subtitles = generateSubtitlesFromScript(masterPlan.ScriptIndonesia)
	logrus.Infof("📝 Subtitles regenerated from script: %d segments", len(masterPlan.Subtitles))

	if err := llmAnalyzer.SaveMasterPlan(masterPlan, ws.MasterPlanFile()); err != nil {
		logrus.Fatalf("❌ Failed to save master plan: %v", err)
	}

	// VALIDASI: Pastikan teks subtitle dan script sama
	scriptText := strings.Join(strings.Fields(masterPlan.ScriptIndonesia), "")
	subtitleText := ""
	for _, s := range masterPlan.Subtitles {
		subtitleText += s.Text
	}
	subtitleText = strings.Join(strings.Fields(subtitleText), "")
	if len(subtitleText) > 0 && !strings.Contains(scriptText, subtitleText) {
		logrus.Warn("⚠️  PERINGATAN: Teks subtitle berbeda dengan script! Silakan periksa kembali.")
	}

	logrus.Infof("✅ Master plan saved: %s", ws.MasterPlanFile())
	logrus.Infof("   Title: %s", masterPlan.Title)
	logrus.Infof("   Script length: %d chars, %d words", len(masterPlan.ScriptIndonesia), len(strings.Fields(masterPlan.ScriptIndonesia)))
	logrus.Infof("   Clips: %d, Subtitles: %d", len(masterPlan.Clips), len(masterPlan.Subtitles))

	// =========================================================
	// STEP 4: TTS Voiceover Generation
	// =========================================================
	logrus.Info("=== STEP 4: TTS Voiceover Generation ===")
	voiceoverFile := ws.VoiceoverFile()
	if err := tts.GenerateTTS(masterPlan.ScriptIndonesia, voiceoverFile); err != nil {
		logrus.Fatalf("❌ TTS generation failed: %v", err)
	}
	logrus.Infof("✅ Voiceover: %s", voiceoverFile)

	// Get voiceover duration for timing synchronization
	voiceoverDuration, err := ffmpegRenderer.GetAudioDuration(voiceoverFile)
	if err != nil {
		logrus.Warnf("⚠️  Could not get voiceover duration: %v, will use video duration", err)
		voiceoverDuration = 0
	} else {
		logrus.Infof("   Voiceover duration: %.2f seconds", voiceoverDuration)
	}

	// =========================================================
	// STEP 5: Video Clipping (Cut & Crop to 9:16)
	// =========================================================
	logrus.Info("=== STEP 5: Video Clipping ===")
	clipFiles := make([]string, 0, len(masterPlan.Clips))
	for i, clip := range masterPlan.Clips {
		clipFile := ws.ClipFile(i)
		err := ffmpegRenderer.CutAndCropClip(ws.InputFile(), clipFile, clip.StartTime, clip.EndTime)
		if err != nil {
			logrus.Warnf("⚠️  Failed to cut clip %d (%s → %s): %v (skipping)", i, clip.StartTime, clip.EndTime, err)
			continue
		}
		clipFiles = append(clipFiles, clipFile)
		logrus.Infof("   ✅ Clip %d/%d: %s → %s (%s)", i+1, len(masterPlan.Clips), clip.StartTime, clip.EndTime, clip.VisualFocus)
	}
	logrus.Infof("✅ %d/%d clips cut and cropped successfully", len(clipFiles), len(masterPlan.Clips))

	// =========================================================
	// STEP 6: Concatenate Clips
	// =========================================================
	logrus.Info("=== STEP 6: Concatenating Clips ===")
	if len(clipFiles) == 0 {
	logrus.Fatalf("❌ No clips generated, cannot proceed")
	}

	listFile := ws.ClipListFile()
	if err := ffmpegRenderer.GenerateClipListFile(clipFiles, listFile); err != nil {
	logrus.Fatalf("❌ Failed to generate clip list: %v", err)
	}
	concatedFile := ws.ConcatenatedVideo()
	if err := ffmpegRenderer.ConcatClips(listFile, concatedFile); err != nil {
	logrus.Fatalf("❌ Failed to concatenate clips: %v", err)
	}
	logrus.Infof("✅ Clips concatenated: %s", concatedFile)

	// =========================================================
	// STEP 7: Render with FFmpeg (Video + Audio + Subtitles + Title)
	// Note: Remotion Video component produces black output on macOS < 15
	// =========================================================
	logrus.Info("=== STEP 7: FFmpeg Render (Video + Audio + Subtitles + Title) ===")
	finalFile := ws.FinalOutputFile()

	// Use voiceover duration if available, otherwise fall back to video duration
	renderDuration := voiceoverDuration
	if renderDuration <= 0 {
		renderDuration, err = ffmpegRenderer.GetVideoDuration(concatedFile)
		if err != nil {
			logrus.Warnf("⚠️  Could not get video duration: %v", err)
			renderDuration = 30.0
		}
		logrus.Infof("   Using video duration: %.2f seconds (voiceover duration unavailable)", renderDuration)
	} else {
		logrus.Infof("   Using voiceover duration: %.2f seconds", renderDuration)
	}

	if err := ffmpegRenderer.RenderWithSubtitles(concatedFile, voiceoverFile, masterPlan.Subtitles, finalFile, masterPlan.Title, renderDuration); err != nil {
		logrus.Fatalf("❌ FFmpeg render failed: %v", err)
	}

	// Verify final output
	finalInfo, err := os.Stat(finalFile)
	if err != nil {
		logrus.Fatalf("❌ Final output file not created: %v", err)
	}

	finalDuration, err := ffmpegRenderer.GetVideoDuration(finalFile)
	if err != nil {
		logrus.Warnf("Could not determine duration: %v", err)
	} else {
		logrus.Infof("   Duration: %.2f seconds", finalDuration)
	}

	logrus.Infof("✅ Final output: %s (%.1f MB)", finalFile, float64(finalInfo.Size())/1024/1024)

	// Clean up if requested
	if *clean {
		logrus.Info("🧹 Cleaning up temp files...")
		if err := os.RemoveAll(ws.EpisodeTemp); err != nil {
			logrus.Warnf("Warning: failed to clean temp directory: %v", err)
		} else {
			logrus.Info("✅ Temp files cleaned")
		}
	}

	logrus.Infof("🎉 Done! Shorts video generated at: %s", finalFile)
}

// generateSubtitlesFromScript splits the script into subtitle segments
// to ensure text and voiceover are always synchronized.
func generateSubtitlesFromScript(script string) []services.Subtitle {
	// Split by sentence boundaries (period, exclamation, question mark)
	words := strings.Fields(script)
	var subtitles []services.Subtitle
	var current string
	segIndex := 0

	for _, word := range words {
		// Split on sentence boundaries
		if strings.HasSuffix(word, ".") || strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?") {
			current += word
			text := strings.TrimSpace(current)
			if text != "" {
				subtitles = append(subtitles, services.Subtitle{
					StartTime: float64(segIndex) * 7.0, // Perpanjang durasi segmen agar tidak terlalu cepat
					EndTime:   float64(segIndex+1) * 7.0,
					Text:      text,
				})
				segIndex++
			}
			current = ""
		} else {
			current += " " + word
		}
	}

	// Handle last segment
	if strings.TrimSpace(current) != "" {
		text := strings.TrimSpace(current)
		subtitles = append(subtitles, services.Subtitle{
			StartTime: float64(segIndex) * 7.0,
			EndTime:   float64(segIndex+1) * 7.0,
			Text:      text,
		})
	}

	// If no segments were created, create a single segment from the whole script
	if len(subtitles) == 0 {
		subtitles = append(subtitles, services.Subtitle{
			StartTime: 0,
			EndTime:   7.0,
			Text:      script,
		})
	}

	return subtitles
}
