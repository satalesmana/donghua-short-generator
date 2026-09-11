package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MasterPlan represents the structured output from the deep LLM analysis.
type MasterPlan struct {
	Title         string     `json:"title"`          // Judul video (maksimal 2 baris)
	ScriptIndonesia string   `json:"script_indonesia"`
	Clips         []PlanClip `json:"clips"`
	Subtitles     []Subtitle `json:"subtitles"`
}

// PlanClip represents a recommended clip segment from the Master Plan.
type PlanClip struct {
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	VisualFocus string `json:"visual_focus"`
}

// Subtitle represents a subtitle line with timing.
type Subtitle struct {
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Text      string  `json:"text"`
}

// LLMAnalyzerService performs deep multimodal analysis using Gemini 1.5 Pro.
type LLMAnalyzerService struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int
	Temp      float64
	Timeout   time.Duration
}

// NewLLMAnalyzerService creates a new LLMAnalyzerService.
func NewLLMAnalyzerService(apiKey, baseURL, model string, maxTokens int, temp float64, timeout time.Duration) *LLMAnalyzerService {
	return &LLMAnalyzerService{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Model:     model,
		MaxTokens: maxTokens,
		Temp:      temp,
		Timeout:   timeout,
	}
}

// GenerateMasterPlan sends the transcription to the 9router LLM endpoint.
// The LLM performs deep analysis, story summarization, and produces a Master Plan JSON.
func (s *LLMAnalyzerService) GenerateMasterPlan(transcriptPath string, promptTemplatePath string, videoPath string) (*MasterPlan, error) {
	// 1. Load system prompt
	systemPrompt, err := s.loadSystemPrompt(promptTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}

	// 2. Load & format transcript
	transcriptText, err := s.loadTranscript(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load transcript: %w", err)
	}

	// 3. Build user prompt
	userPrompt := s.buildUserPrompt(transcriptText, videoPath)

	// 5. Send request to LLM
	logrus.Info("🤖 Sending multimodal request to 9router...")
	rawResponse, err := s.sendChatCompletionRequest(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 6. Parse JSON response
	masterPlan, err := s.parseMasterPlan(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse master plan: %w", err)
	}

	logrus.Infof("✅ Master plan generated: %d clips, %d subtitles, script %d chars",
		len(masterPlan.Clips), len(masterPlan.Subtitles), len(masterPlan.ScriptIndonesia))
	logrus.Infof("📌 Title: %s", masterPlan.Title)
	return masterPlan, nil
}

// SaveMasterPlan writes the master plan to a JSON file.
func (s *LLMAnalyzerService) SaveMasterPlan(plan *MasterPlan, outputPath string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal master plan: %w", err)
	}
	return os.WriteFile(outputPath, data, 0644)
}

// loadSystemPrompt reads the system prompt template.
func (s *LLMAnalyzerService) loadSystemPrompt(templatePath string) (string, error) {
	if templatePath == "" {
		return "", fmt.Errorf("system prompt template path is required")
	}

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	return string(data), nil
}

// loadTranscript reads and formats the transcript JSON for LLM consumption.
func (s *LLMAnalyzerService) loadTranscript(transcriptPath string) (string, error) {
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript file: %w", err)
	}

	var transcript struct {
		Text     string `json:"text"`
		Segments []struct {
			Text  string  `json:"text"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"segments"`
	}

	if err := json.Unmarshal(data, &transcript); err != nil {
		return string(data), nil // fallback: return raw if not JSON
	}

	// Format into readable timestamped text
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FULL TEXT:\n%s\n\nTIMESTAMPED SEGMENTS:\n", transcript.Text))
	for _, seg := range transcript.Segments {
		sb.WriteString(fmt.Sprintf("[%.2f→%.2f] %s\n", seg.Start, seg.End, seg.Text))
	}

	return sb.String(), nil
}

// buildUserPrompt constructs the full user message for the LLM.
func (s *LLMAnalyzerService) buildUserPrompt(transcript, videoPath string) string {
	return fmt.Sprintf(`Berikut adalah data analisis dari 1 episode Donghua/Anime panjang 20-30 menit:

[DATA TRANSCRIP MANDARIN DENGAN TIMESTAMP]
%s

[NAMA FILE VIDEO ASAL]
%s

Silakan lakukan Analisis Mendalam & Summarization berikut:
1. Memahami alur cerita penuh dari transkrip.
2. Mengidentifikasi momen paling menarik, emosional, aksi, dan/atau inti cerita.
3. Menerjemahkan isi penting ke Bahasa Indonesia dengan natural.
4. Membuat Master Plan Shorts berdurasi 90–180 detik (1,5–3 menit) dengan:
   - script_indonesia: narasi voiceover (150–200 kata)
   - clips: 15–25 potongan dengan start_time, end_time, visual_focus (5–8 detik per klip)
   - subtitles: data subtitle per frasa (start_time, end_time, text) yang selaras dengan voiceover

Output HARUS berupa JSON valid sesuai skema yang ditentukan, dengan total durasi video antara 90 sampai 180 detik (1,5–3 menit).`, transcript, videoPath)
}

// sendChatCompletionRequest sends a chat completion request to 9router API.
func (s *LLMAnalyzerService) sendChatCompletionRequest(systemPrompt, userPrompt string) (string, error) {
	payload := s.buildPayload(systemPrompt, userPrompt)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.APIKey))

	client := &http.Client{Timeout: s.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respData))
	}

	logrus.Debugf("Raw API response (%d bytes)", len(respData))
	return string(respData), nil
}

// buildPayload constructs the chat completion JSON payload.
type chatCompletionRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (s *LLMAnalyzerService) buildPayload(systemPrompt, userPrompt string) *chatCompletionRequest {
	return &chatCompletionRequest{
		Model:       s.Model,
		MaxTokens:   s.MaxTokens,
		Temperature: s.Temp,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
}

// parseMasterPlan extracts and unmarshals the Master Plan JSON from the API response.
func (s *LLMAnalyzerService) parseMasterPlan(rawResponse string) (*MasterPlan, error) {
	// 0. Handle SSE stream data if present
	if strings.HasPrefix(rawResponse, "data: ") {
		rawResponse = s.flattenSSEStream(rawResponse)
	}

	// 0.5. Fix unescaped newlines in JSON strings (common LLM output issue)
	rawResponse = fixUnescapedNewlines(rawResponse)

	var apiResp struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// Use a streaming Decoder instead of json.Unmarshal: some proxies append
	// trailing data after the main JSON object (e.g. an SSE "data: [DONE]"
	// marker, even in non-stream mode). json.Unmarshal rejects that with
	// "invalid character ... after top-level value"; a Decoder reads only the
	// first top-level value and ignores the rest.
	if err := json.NewDecoder(strings.NewReader(rawResponse)).Decode(&apiResp); err != nil {
		// Raw response is malformed — try to extract content field manually
		content := extractContentFromRaw(rawResponse)
		if content == "" {
			return nil, fmt.Errorf("failed to parse API response: %w — raw: %s", err, truncate(rawResponse, 500))
		}
		return s.parseContent(content)
	}

	if apiResp.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in API response")
	}

	content := apiResp.Choices[0].Message.Content
	if content == "" {
		return nil, fmt.Errorf("empty content in API response")
	}

	return s.parseContent(content)
}

// flattenSSEStream converts a stream of "data: {chunk}" lines into a single complete message content.
func (s *LLMAnalyzerService) flattenSSEStream(raw string) string {
	lines := strings.Split(raw, "\n")

	// We need to build a mock "chat.completion" structure that parseMasterPlan expects
	// or just return a string that extractContentFromRaw can handle.
	// Since parseMasterPlan continues to parse, we'll return a fake full response.

	var fullContent strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			if len(chunk.Choices) > 0 {
				fullContent.WriteString(chunk.Choices[0].Delta.Content)
			}
		}
	}

	// Wrap the collected content in a standard response JSON
	return fmt.Sprintf(`{"choices":[{"message":{"content":%q}}]}`, fullContent.String())
}

// parseContent extracts and unmarshals the Master Plan JSON from a content string.
func (s *LLMAnalyzerService) parseContent(content string) (*MasterPlan, error) {
	// Strip markdown code fences if present
	content = stripMarkdownFences(content)

	// Extract JSON from the content (may have surrounding text/markers)
	jsonStr, err := extractJSONObject(content)
	if err != nil {
		// Try to repair truncated JSON
		jsonStr, err = repairTruncatedJSON(content)
		if err != nil {
			return nil, fmt.Errorf("failed to extract JSON from content: %w — content: %s", err, truncate(content, 500))
		}
	}

	var masterPlan MasterPlan
	if err := json.Unmarshal([]byte(jsonStr), &masterPlan); err != nil {
		// Final attempt: repair and retry
		repaired, repErr := repairTruncatedJSON(jsonStr)
		if repErr == nil {
			if err2 := json.Unmarshal([]byte(repaired), &masterPlan); err2 == nil {
				return &masterPlan, nil
			}
		}
		return nil, fmt.Errorf("failed to unmarshal master plan JSON: %w — json: %s", err, truncate(jsonStr, 500))
	}

	return &masterPlan, nil
}

// stripMarkdownFences removes surrounding ```json ... ``` wrappers.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	// Remove leading ```json or ```
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 0 {
			s = strings.TrimPrefix(lines[0], "```")
			s = strings.TrimSpace(s)
			if len(lines) > 1 {
				s = lines[1]
			}
		}
	}
	// Remove trailing ```
	for strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

// fixUnescapedNewlines repairs JSON with unescaped newline characters inside strings.
// Some LLMs return malformed JSON where newlines in string values are not escaped.
func fixUnescapedNewlines(s string) string {
	// Fix invalid escape sequences like \N (capital N) which is not valid JSON
	s = strings.ReplaceAll(s, `\N`, `\n`)
	s = strings.ReplaceAll(s, `\n`, "\\n") // properly escape newlines

	var sb strings.Builder
	inString := false
	escape := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escape {
			sb.WriteByte(c)
			escape = false
			continue
		}

		if c == '\\' {
			sb.WriteByte(c)
			escape = true
			continue
		}

		if c == '"' {
			inString = !inString
			sb.WriteByte(c)
			continue
		}

		// If we're inside a string and encounter an unescaped newline, replace with space
		if inString && (c == '\n' || c == '\r') {
			sb.WriteByte(' ')
			continue
		}

		sb.WriteByte(c)
	}

	return sb.String()
}

// extractContentFromRaw tries to pull the content field from a malformed raw API response.
func extractContentFromRaw(raw string) string {
	// Look for "content":"..." pattern, handling escaped quotes
	idx := strings.Index(raw, `"content"`)
	if idx == -1 {
		return ""
	}
	// Find the opening quote after "content":
	quoteIdx := strings.Index(raw[idx:], `"`)
	if quoteIdx == -1 {
		return ""
	}
	quoteIdx += idx
	// Skip the colon and whitespace
	rest := raw[quoteIdx+1:]
	rest = strings.TrimLeft(rest, " 	\n\r")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:] // skip opening quote

	// Now scan for the closing unescaped quote
	var sb strings.Builder
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '\\' && i+1 < len(rest) {
			sb.WriteByte(c)
			i++
			sb.WriteByte(rest[i])
			continue
		}
		if c == '"' {
			return sb.String()
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// repairTruncatedJSON attempts to close open braces/brackets/strings to make JSON parseable.
func repairTruncatedJSON(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") {
		return "", fmt.Errorf("not a JSON object")
	}

	depth := 0
	inString := false
	escape := false
	lastValid := 0

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && inString {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' || c == '[' {
			depth++
		} else if c == '}' || c == ']' {
			depth--
			if depth < 0 {
				depth = 0
			}
		}
		if depth == 0 {
			lastValid = i + 1
		}
	}

	// Truncate to last complete JSON object boundary
	if lastValid == 0 {
		return "", fmt.Errorf("no valid JSON boundary found")
	}
	result := s[:lastValid]
	// Close any remaining open structures
	for depth > 0 {
		if lastValid > 0 && s[lastValid-1] == '[' {
			result += "]"
		} else {
			result += "}"
		}
		depth--
	}

	// Try to parse the repaired JSON
	var tmp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &tmp); err != nil {
		return "", fmt.Errorf("could not repair truncated JSON: %w", err)
	}
	return result, nil
}

// extractJSONObject finds and extracts the first valid JSON object from a string.
func extractJSONObject(s string) (string, error) {
	start := bytes.IndexByte([]byte(s), '{')
	if start == -1 {
		return "", fmt.Errorf("no JSON object found")
	}

	depth := 0
	inString := false
	escape := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escape {
			escape = false
			continue
		}

		if c == '\\' && inString {
			escape = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}

	return "", fmt.Errorf("could not find matching closing brace")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
