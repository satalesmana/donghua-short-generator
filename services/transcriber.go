package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Segment represents a word-level transcription segment with timestamps.
type Segment struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Transcript represents the full transcription result with word-level timestamps.
type Transcript struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language"`
}

// TranscriberService handles speech-to-text transcription via Whisper.cpp CLI.
type TranscriberService struct {
	APIKey           string
	Model            string
	Language         string
	WhisperBinary    string
	WhisperModelPath string
	OutputDir        string // Whisper output directory (per-episode temp)
	Timeout          time.Duration
}

// NewTranscriberService creates a new TranscriberService instance.
func NewTranscriberService(apiKey, model, baseURL, language string, timeout time.Duration,
	whisperBinary, whisperModelPath, outputDir string) *TranscriberService {

	return &TranscriberService{
		APIKey:           apiKey,
		Model:            model,
		Language:         language,
		WhisperBinary:    whisperBinary,
		WhisperModelPath: whisperModelPath,
		OutputDir:        outputDir,
		Timeout:          timeout,
	}
}

// TranscribeAudio transcribes the audio file using local Whisper.cpp CLI.
func (s *TranscriberService) TranscribeAudio(audioPath string) (*Transcript, error) {
	logrus.Info("📝 Transcribing via local Whisper.cpp CLI...")
	return s.transcribeViaWhisperCPP(audioPath)
}

// transcribeViaWhisperCPP calls the local whisper-cli binary and parses the JSON output.
// whisper-cli outputs JSON in the format:
// {
//   "transcription": [{"text":"...","timestamps":{"from":"HH:MM:SS,mmm","to":"..."},"offsets":{"from":ms,"to":ms}}],
//   "result": {"language":"zh"}
// }
func (s *TranscriberService) transcribeViaWhisperCPP(audioPath string) (*Transcript, error) {
	outputDir := s.OutputDir
	if outputDir == "" {
		outputDir, _ = os.Getwd()
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	outputFile := filepath.Join(outputDir, "whisper_result")

	args := []string{
		"-m", s.WhisperModelPath,
		"-f", audioPath,
		"-oj",
		"-of", outputFile,
		"-l", s.Language,
		"-wt", "0.5",
		"-otxt",
	}

	cmd := exec.Command(s.WhisperBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper-cli failed: %v — output: %s", err, strings.TrimSpace(string(output)))
	}

	jsonFile := outputFile + ".json"
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper JSON output: %w", err)
	}

	// Parse using a generic map first for flexibility with varying formats
	var rawResult map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to parse whisper JSON top-level: %w", err)
	}

	// Extract language
	language := s.Language
	if resultRaw, ok := rawResult["result"]; ok {
		var resultInfo struct {
			Language string `json:"language"`
		}
		_ = json.Unmarshal(resultRaw, &resultInfo)
		if resultInfo.Language != "" {
			language = resultInfo.Language
		}
	}

	// Extract transcription segments
	var segments []Segment
	if transRaw, ok := rawResult["transcription"]; ok {
		var rawSegs []struct {
			Text       string          `json:"text"`
			Timestamps struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"timestamps"`
			Offsets struct {
				From int `json:"from"`
				To   int `json:"to"`
			} `json:"offsets"`
		}
		if err := json.Unmarshal(transRaw, &rawSegs); err != nil {
			return nil, fmt.Errorf("failed to parse transcription array: %w", err)
		}
		for _, rs := range rawSegs {
			startSec := float64(rs.Offsets.From) / 1000.0
			endSec := float64(rs.Offsets.To) / 1000.0
			if rs.Text != "" {
				segments = append(segments, Segment{
					Text:  strings.TrimSpace(rs.Text),
					Start: startSec,
					End:   endSec,
				})
			}
		}
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no transcription segments found in whisper output")
	}

	transcript := &Transcript{
		Language: language,
		Segments: segments,
	}

	var fullText string
	for _, seg := range segments {
		fullText += seg.Text + " "
	}
	transcript.Text = strings.TrimSpace(fullText)

	logrus.Infof("✅ Transcription complete via Whisper.cpp CLI: %d segments, language=%s", len(segments), language)
	return transcript, nil
}

// SaveTranscriptJSON writes the transcript to a structured JSON file.
func (s *TranscriberService) SaveTranscriptJSON(transcript *Transcript, outputPath string) error {
	data, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal transcript JSON: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write transcript file: %w", err)
	}
	logrus.Infof("📄 Transcript saved: %s", outputPath)
	return nil
}
