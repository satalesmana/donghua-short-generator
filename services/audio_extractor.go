package services

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// AudioExtractorService wraps FFmpeg CLI to extract high-quality audio from video.
type AudioExtractorService struct{}

// NewAudioExtractorService creates a new instance.
func NewAudioExtractorService() *AudioExtractorService {
	return &AudioExtractorService{}
}

// ExtractAudio extracts mono 16kHz PCM WAV audio from the input video file.
// Command: ffmpeg -i input.mp4 -vn -acodec pcm_s16le -ar 16000 -ac 1 output.wav
func (s *AudioExtractorService) ExtractAudio(videoPath, outputAudioPath string) error {
	logrus.Infof("Extracting audio from %s → %s", videoPath, outputAudioPath)

	args := []string{
		"-i", videoPath,
		"-vn",                // no video
		"-acodec", "pcm_s16le", // PCM signed 16-bit little-endian
		"-ar", "16000",       // 16 kHz sample rate
		"-ac", "1",           // mono
		"-y",                 // overwrite
		outputAudioPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg audio extraction failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	logrus.Infof("✅ Audio extracted: %s", outputAudioPath)
	return nil
}
