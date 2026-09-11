package services

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// FFmpegRendererService handles all video editing operations: cutting,
// cropping to 9:16 vertical, concatenating clips, and adding subtitles.
type FFmpegRendererService struct{}

// NewFFmpegRendererService creates a new FFmpegRendererService.
func NewFFmpegRendererService() *FFmpegRendererService {
	return &FFmpegRendererService{}
}

// normalizeTimestamp ensures FFmpeg accepts HH:MM:SS by normalizing seconds > 59.
func normalizeTimestamp(ts string) string {
	parts := strings.Split(ts, ":")
	if len(parts) == 2 {
		// Format is MM:SS, convert to HH:MM:SS
		ts = "00:" + ts
		parts = strings.Split(ts, ":")
	}
	if len(parts) != 3 {
		return ts
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])

	totalSeconds := hours*3600 + minutes*60 + seconds
	hours = totalSeconds / 3600
	minutes = (totalSeconds % 3600) / 60
	seconds = totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// CutAndCropClipWithBlur cuts a segment and creates a blurred background layout.
// The source video fills 1080x1920 as a blurred backdrop, with the main video
// centered on top at 1080x1350 (leaving space for title/subtitle).
func (f *FFmpegRendererService) CutAndCropClipWithBlur(inputFile, outputFile, startTime, endTime string) error {
	startTime = normalizeTimestamp(startTime)
	endTime = normalizeTimestamp(endTime)

	if startTime == "" || endTime == "" {
		return fmt.Errorf("start and end timestamps required")
	}

	videoDuration, err := f.GetVideoDuration(inputFile)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	startSec, _ := parseTimestampToSeconds(startTime)
	endSec, _ := parseTimestampToSeconds(endTime)

	if startSec >= videoDuration {
		return fmt.Errorf("start time %.0fs exceeds video duration %.0fs", startSec, videoDuration)
	}
	if endSec > videoDuration {
		endTime = fmt.Sprintf("%02d:%02d:%02d", int(videoDuration)/3600, (int(videoDuration)%3600)/60, int(videoDuration)%60)
		logrus.Warnf("   Clamped end time to %.0fs (video duration)", videoDuration)
	}
	if endSec <= startSec {
		return fmt.Errorf("end time %.0fs must be greater than start time %.0fs", endSec, startSec)
	}

	// Layout: 1080x1920
	// Title area:    y=0    to y=160   (160px)
	// Video area:    y=160  to y=1760  (1600px)
	// Subtitle area: y=1760 to y=1920 (160px)
	//
	// Background: full frame, blurred
	// Foreground: centered video scaled/cropped to 1080x1600
	const (
		outW        = 1080
		outH        = 1920
		videoH      = 1600
		videoYStart = 160
	)

	// Build a split filtergraph:
	// [0] → split into [bg] and [fg]
	// [bg] → scale+crop to fill 1080x1920 + boxblur(20)
	// [fg] → scale+crop center to 1080x1600 (cover / crop center if width matches)
	// overlay [fg] onto [bg] at (0, 160)
	vf := fmt.Sprintf(
		"[0:v]split=2[bg][fg];"+
			"[bg]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,boxblur=20:20[blurred];"+
			"[fg]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d[main];"+
			"[blurred][main]overlay=0:%d",
		outW, outH, outW, outH,
		outW, videoH, outW, videoH,
		videoYStart,
	)

	args := []string{
		"-ss", startTime,
		"-to", endTime,
		"-i", inputFile,
		"-vf", vf,
		"-an",
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "medium",
		"-r", "30",
		"-y",
		outputFile,
	}

	logrus.Debugf("🔪 Cutting clip with blur: %s (%s → %s)", filepath.Base(inputFile), startTime, endTime)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg cut-blur failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	info, err := os.Stat(outputFile)
	if err != nil {
		return fmt.Errorf("output file not created: %w", err)
	}
	if info.Size() < 1000 {
		return fmt.Errorf("output file too small (%d bytes), likely invalid clip", info.Size())
	}

	return nil
}

// CutAndCropClip cuts a segment from the input video and crops it to 9:16 vertical format.
func (f *FFmpegRendererService) CutAndCropClip(inputFile, outputFile, startTime, endTime string) error {
	startTime = normalizeTimestamp(startTime)
	endTime = normalizeTimestamp(endTime)

	if startTime == "" || endTime == "" {
		return fmt.Errorf("start and end timestamps required")
	}

	// Validate timestamps are within video duration
	videoDuration, err := f.GetVideoDuration(inputFile)
	if err != nil {
		return fmt.Errorf("failed to get video duration: %w", err)
	}

	startSec, _ := parseTimestampToSeconds(startTime)
	endSec, _ := parseTimestampToSeconds(endTime)

	if startSec >= videoDuration {
		return fmt.Errorf("start time %.0fs exceeds video duration %.0fs", startSec, videoDuration)
	}
	if endSec > videoDuration {
		endTime = fmt.Sprintf("%02d:%02d:%02d", int(videoDuration)/3600, (int(videoDuration)%3600)/60, int(videoDuration)%60)
		logrus.Warnf("   Clamped end time to %.0fs (video duration)", videoDuration)
	}
	if endSec <= startSec {
		return fmt.Errorf("end time %.0fs must be greater than start time %.0fs", endSec, startSec)
	}

	// For horizontal video (16:9), crop to 9:16 and scale to 1080x1920
	// This handles both horizontal and vertical source videos
	args := []string{
		"-ss", startTime,
		"-to", endTime,
		"-i", inputFile,
		"-vf", "crop=ih*(9/16):ih,scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2",
		"-an",
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "medium",
		"-r", "30",
		"-y",
		outputFile,
	}

	logrus.Debugf("🔪 Cutting clip: %s (%s → %s)", filepath.Base(inputFile), startTime, endTime)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg cut-crop failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	// Validate output file was created and has content
	info, err := os.Stat(outputFile)
	if err != nil {
		return fmt.Errorf("output file not created: %w", err)
	}
	if info.Size() < 1000 { // Less than 1KB is suspicious
		return fmt.Errorf("output file too small (%d bytes), likely invalid clip", info.Size())
	}

	return nil
}

// parseTimestampToSeconds converts HH:MM:SS to total seconds
func parseTimestampToSeconds(ts string) (float64, error) {
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid timestamp format: %s", ts)
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds, _ := strconv.Atoi(parts[2])
	return float64(hours*3600 + minutes*60 + seconds), nil
}

// GenerateClipListFile writes an FFmpeg concat-compatible list.txt file.
func (f *FFmpegRendererService) GenerateClipListFile(clipFiles []string, listFile string) error {
	var lines []string
	for _, clip := range clipFiles {
		absPath, err := filepath.Abs(clip)
		if err != nil {
			return fmt.Errorf("failed to get abs path for %s: %w", clip, err)
		}
		lines = append(lines, fmt.Sprintf("file '%s'", absPath))
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(listFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write list file %s: %w", listFile, err)
	}

	logrus.Infof("📋 Clip list written: %s (%d clips)", listFile, len(clipFiles))
	return nil
}

// ConcatClips concatenates multiple video clips using FFmpeg concat demuxer.
func (f *FFmpegRendererService) ConcatClips(listFile, outputFile string) error {
	if _, err := os.Stat(listFile); os.IsNotExist(err) {
		return fmt.Errorf("clip list file not found: %s", listFile)
	}

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "medium",
		"-an",
		"-y",
		outputFile,
	}

	logrus.Infof("🔗 Concatenating clips → %s", outputFile)
	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// GetVideoDuration returns the duration of a video file using ffprobe.
func (f *FFmpegRendererService) GetVideoDuration(videoFile string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoFile,
	}

	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w — output: %s", err, string(output))
	}

	var duration float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// RenderWithSubtitles combines video, audio, and subtitles into final output using FFmpeg.
func (f *FFmpegRendererService) RenderWithSubtitles(
	concatenatedVideo string,
	voiceoverFile string,
	subtitles []Subtitle,
	outputFile string,
	title string, // Title overlay for the video
	voiceoverDuration float64, // Duration of voiceover in seconds
) error {
	// Get video duration
	// Note: video duration will be overridden by voiceoverDuration parameter

	// Scale subtitle timing based on voiceover duration vs original subtitle duration
	// First, find the earliest start time to normalize subtitles to start at 0
	var minStartTime, maxEndTime float64
	for i, sub := range subtitles {
		if i == 0 || sub.StartTime < minStartTime {
			minStartTime = sub.StartTime
		}
		if i == 0 || sub.EndTime > maxEndTime {
			maxEndTime = sub.EndTime
		}
	}

	// Calculate original subtitle duration (from first to last subtitle)
	originalSubtitleDuration := maxEndTime - minStartTime

	// Calculate scale factor, capped at 1.0 to prevent subtitle stretching.
	// When voiceover is longer than subtitle timings, we don't stretch them
	// (which would make them feel slow). Instead, we extend the last subtitle
	// to fill the remaining time.
	scaleFactor := 1.0
	if originalSubtitleDuration > 0 {
		ratio := voiceoverDuration / originalSubtitleDuration
		if ratio < 1.0 {
			scaleFactor = ratio
		}
	}

	// Extra padding to add to the last subtitle end time (in seconds)
	// This fills the gap when voiceover is longer than subtitle span
	const lastSubtitlePadding = 1.0

	logrus.Infof("📊 Timing scale factor: %.2f (voiceover: %.2fs, subtitles: %.2fs, min_start: %.2fs)", scaleFactor, voiceoverDuration, originalSubtitleDuration, minStartTime)

	// Create subtitle file in ASS format (with word-by-word animation)
	assFile := outputFile + ".ass"
	if err := f.writeASSFile(assFile, subtitles, title, scaleFactor, minStartTime, voiceoverDuration); err != nil {
		return fmt.Errorf("failed to write ASS file: %w", err)
	}
	defer os.Remove(assFile)

	logrus.Infof("📝 ASS subtitles written: %s (%d lines)", assFile, len(subtitles))
	if title != "" {
		logrus.Infof("📌 Title overlay: %s", title)
	}
	logrus.Infof("🎬 Rendering with FFmpeg (video + audio + subtitles)...")

	// FFmpeg command to merge video, audio, and burn subtitles + title
	var vfParts []string

	// Add title overlay using textfile to support multi-line newlines (\n) smoothly without escaping nightmares
	if title != "" {
		title = strings.ReplaceAll(title, "\\n", "\n")
		titleFile := outputFile + ".title.txt"
		if err := os.WriteFile(titleFile, []byte(title), 0644); err == nil {
			// Using Arial Bold.ttf for thicker text, and centering properly via drawtext
			// Note: drawtext by default renders left-aligned per line, but when using textfile with multiple lines,
			// to make each line center-aligned, we can use the text_align option available in newer ffmpeg,
			// or box=1:boxcolor=... or simply rely on x=(w-text_w)/2 for single line / text_align=center.
			// Let's add text_align=center if supported, or ensure proper alignment.
			vfParts = append(vfParts, fmt.Sprintf(
				"drawtext=textfile='%s':fontfile='/System/Library/Fonts/Supplemental/Arial Bold.ttf':fontsize=52:fontcolor=yellow:x=(w-text_w)/2:y=80:text_align=center:line_spacing=12:shadowcolor=black:shadowx=2:shadowy=2",
				strings.ReplaceAll(titleFile, ":", "\\:"),
			))
			defer os.Remove(titleFile)
		}
	}

	// Add subtitle background box and ensure subtitles are rendered LAST (at the very front/top layer)
	// User requested height = 200, y = 1760, color = #2D0000
	vfParts = append(vfParts, "drawbox=x=0:y=1500:width=1080:height=400:color=#3E0F8D:thickness=fill")

	// Add subtitle overlay (placed at the end of vfParts so it renders on top of everything)
	vfParts = append(vfParts, fmt.Sprintf(
		"subtitles='%s'",
		strings.ReplaceAll(assFile, ":", "\\:"), // Escape colon untuk Windows
	))

	vfFilter := strings.Join(vfParts, ",")

	args := []string{
		"-i", concatenatedVideo,
		"-i", voiceoverFile,
		"-vf", vfFilter,
		"-c:v", "libx264",
		"-crf", "23",
		"-preset", "medium",
		"-c:a", "aac",
		"-b:a", "192k",
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-t", fmt.Sprintf("%.2f", voiceoverDuration),
		"-y",
		outputFile,
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg render with subtitles failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	logrus.Infof("✅ FFmpeg render complete: %s", outputFile)
	return nil
}

// writeASSFile writes subtitles in ASS format with word-by-word animation.
// ASS (Advanced SubStation Alpha) supports rich formatting including fade animations.
// minStartTime is subtracted from all times to normalize subtitles to start at 0 before scaling.
// voiceoverDuration is used to extend the last subtitle so it fills the remaining time.
func (f *FFmpegRendererService) writeASSFile(filePath string, subtitles []Subtitle, title string, scaleFactor, minStartTime, voiceoverDuration float64) error {
	const lastSubtitlePadding = 1.0
	var sb strings.Builder

	// ASS header
	sb.WriteString(`[Script Info]
Title: dao-clip subtitles
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,72,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,1,0,0,0,100,100,0,0,3,6,3,2,30,30,60,1
Style: Title,Arial,52,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,3,2,2,30,30,50,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`)

	// Layout positions based on 1080x1920 (9:16)
	// Title area: top (y=0 to y=160)
	// Video area: y=160 to y=1760 (height 1600)
	// Subtitle area: y=1500 to y=1920 (height 420)
	// Alignment=2 (bottom-center), MarginV=60 from bottom
	const titleMarginV = 30     // Title: 30px from top
	const subtitleMarginV = 100 // Subtitle: 60px from bottom (Alignment 2 = bottom-center)

	// Write title if provided (at top of screen) - ONLY via drawtext, NOT in ASS
	// Title is rendered by FFmpeg drawtext filter, not by ASS file

	for i, sub := range subtitles {
		// Normalize to relative time (subtract minStartTime), then scale
		startSec := (sub.StartTime - minStartTime) * scaleFactor
		endSec := (sub.EndTime - minStartTime) * scaleFactor
		// Extend the last non-empty subtitle to fill the gap before voiceover ends
		if i == len(subtitles)-1 || (i < len(subtitles)-1 && subtitles[i+1].Text == "") {
			endSec = math.Min(endSec+lastSubtitlePadding, voiceoverDuration)
		}
		text := sub.Text

		// Skip if text is empty or just noise
		text = strings.TrimSpace(text)
		if text == "" || strings.HasPrefix(text, "(") {
			_ = i
			continue
		}

		// Wrap text into maximum 2 lines
		maxCharsPerLine := 35
		lines := f.wrapText(text, maxCharsPerLine)

		// If text is very long, add font override tag
		fontTag := ""
		if len(text) > 40 {
			fontTag = "{\\fs50}"
		}

		// Join lines with ASS newline tag `\N`
		joinedText := strings.Join(lines, "\\N")

		// Write single subtitle block for all lines
		startASS := f.toASSTime(startSec)
		endASS := f.toASSTime(endSec)
		sb.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,%d,,%s%s\n",
			startASS, endASS, subtitleMarginV, fontTag, joinedText))

		_ = i // suppress unused warning
	}

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}

// GetAudioDuration returns the duration of an audio file using ffprobe.
func (f *FFmpegRendererService) GetAudioDuration(audioFile string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioFile,
	}

	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w — output: %s", err, string(output))
	}

	var duration float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// toASSTime converts seconds to ASS time format (H:MM:SS.CS)
func (f *FFmpegRendererService) toASSTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	csec := int(math.Round((seconds - float64(int(seconds))) * 100))

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, secs, csec)
}

// wrapTitleText splits title text into multiple lines using '\n' for FFmpeg drawtext
func (f *FFmpegRendererService) wrapTitleText(text string, maxChars int) string {
	lines := f.wrapText(text, maxChars)
	return strings.Join(lines, "\n")
}

// wrapText splits text into lines based on max characters per line.
func (f *FFmpegRendererService) wrapText(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder
	currentLen := 0

	for _, word := range words {
		wordLen := len(word)
		if currentLen == 0 {
			// First word on line
			currentLine.WriteString(word)
			currentLen = wordLen
		} else if currentLen+1+wordLen > maxChars {
			// Need new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
			currentLen = wordLen
		} else {
			// Continue current line
			currentLine.WriteString(" " + word)
			currentLen += 1 + wordLen
		}
	}

	// Add last line
	if currentLen > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}
