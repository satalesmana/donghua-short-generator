package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// TTSService generates voiceover audio via Fish Audio TTS through OpenRouter API.
// Fish Audio returns raw PCM data (audio/pcm format) which we convert to WAV.
type TTSService struct {
	APIKey     string
	BaseURL    string // e.g. "https://openrouter.ai/api/v1"
	Model      string // e.g. "fish-audio/s2.1-pro-free:free"
	Voice      string // Fish Audio voice reference_id (hex string)
	SampleRate int    // output sample rate
	Timeout    time.Duration
}

// NewTTSService creates a new TTSService.
func NewTTSService(apiKey, baseURL, model, voice string, timeout time.Duration) *TTSService {
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	return &TTSService{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		Voice:      voice,
		SampleRate: 44100,
		Timeout:    timeout,
	}
}

// ttsRequest represents the OpenAI-compatible /audio/speech request body.
type ttsRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

// GenerateTTS generates a TTS audio file from the given text using Fish Audio API.
// The API returns raw PCM data which we wrap in a WAV header.
func (s *TTSService) GenerateTTS(text, outputPath string) error {
	logrus.Infof("🔊 Generating TTS (Fish Audio via OpenRouter): model=%s, voice=%s, output=%s", s.Model, s.Voice, outputPath)

	payload := ttsRequest{
		Model: s.Model,
		Input: text,
		Voice: s.Voice,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	req, err := http.NewRequest("POST", s.BaseURL+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create TTS request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.APIKey))
	// Request raw binary response
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: s.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("TTS API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS API error %d: %s", resp.StatusCode, string(respData))
	}

	// Read raw PCM data
	pcmData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TTS audio data: %w", err)
	}

	// Write as WAV file with proper header
	if err := s.writeWAV(pcmData, outputPath); err != nil {
		return fmt.Errorf("failed to write WAV file: %w", err)
	}

	fileSize, _ := os.Stat(outputPath)
	logrus.Infof("✅ TTS complete: %s (%d bytes)", outputPath, fileSize.Size())
	return nil
}

// writeWAV converts raw PCM data to WAV format.
func (s *TTSService) writeWAV(pcmData []byte, outputPath string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outFile.Close()

	// WAV parameters
	channels := 1
	bitsPerSample := 16
	byteRate := s.SampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := len(pcmData)
	fileSize := 44 + dataSize // 44 bytes WAV header + data

	// Write WAV header
	wavHeader := make([]byte, 44)
	copy(wavHeader[0:4], "RIFF")
	putUint32(wavHeader[4:8], uint32(fileSize))
	copy(wavHeader[8:12], "WAVE")
	copy(wavHeader[12:16], "fmt ")
	putUint32(wavHeader[16:20], 16) // chunk size
	putUint16(wavHeader[20:22], 1)  // audio format (PCM)
	putUint16(wavHeader[22:24], uint16(channels))
	putUint32(wavHeader[24:28], uint32(s.SampleRate))
	putUint32(wavHeader[28:32], uint32(byteRate))
	putUint16(wavHeader[32:34], uint16(blockAlign))
	putUint16(wavHeader[34:36], uint16(bitsPerSample))
	copy(wavHeader[36:40], "data")
	putUint32(wavHeader[40:44], uint32(dataSize))

	if _, err := outFile.Write(wavHeader); err != nil {
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	if _, err := outFile.Write(pcmData); err != nil {
		return fmt.Errorf("failed to write WAV data: %w", err)
	}

	return nil
}

// putUint16 writes a uint16 in little-endian format.
func putUint16(p []byte, v uint16) {
	p[0] = byte(v)
	p[1] = byte(v >> 8)
}

// putUint32 writes a uint32 in little-endian format.
func putUint32(p []byte, v uint32) {
	p[0] = byte(v)
	p[1] = byte(v >> 8)
	p[2] = byte(v >> 16)
	p[3] = byte(v >> 24)
}
