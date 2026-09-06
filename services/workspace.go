package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Workspace manages all directory/file paths used during the 7-step pipeline.
type Workspace struct {
	EpisodeID    string
	WorkspaceDir string
	InputDir     string
	TempDir      string
	OutputDir    string
	EpisodeTemp  string // workspace/temp/<episodeID>/
}

// NewWorkspace creates a Workspace instance for a given episode ID.
func NewWorkspace(workspaceDir, episodeID string) *Workspace {
	w := &Workspace{
		EpisodeID:    episodeID,
		WorkspaceDir: workspaceDir,
		InputDir:     filepath.Join(workspaceDir, "input"),
		TempDir:      filepath.Join(workspaceDir, "temp"),
		OutputDir:    filepath.Join(workspaceDir, "output"),
	}
	w.EpisodeTemp = filepath.Join(w.TempDir, episodeID)
	return w
}

// InitWorkspace creates: input/, temp/<episodeID>/, output/
func (w *Workspace) InitWorkspace() error {
	dirs := []string{
		w.WorkspaceDir,
		w.InputDir,
		w.EpisodeTemp,
		w.OutputDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		logrus.Infof("Directory ready: %s", dir)
	}
	return nil
}

// --- Path helpers ---

// InputFile returns: workspace/input/<episodeID>.mp4
func (w *Workspace) InputFile() string {
	return filepath.Join(w.InputDir, fmt.Sprintf("%s.mp4", w.EpisodeID))
}

// AudioFile returns: workspace/temp/<episodeID>/<episodeID>_audio.wav
func (w *Workspace) AudioFile() string {
	return filepath.Join(w.EpisodeTemp, fmt.Sprintf("%s_audio.wav", w.EpisodeID))
}

// TranscriptFile returns: workspace/temp/<episodeID>/transcript.json
func (w *Workspace) TranscriptFile() string {
	return filepath.Join(w.EpisodeTemp, "transcript.json")
}

// SceneDataFile returns: workspace/temp/<episodeID>/scenes.json
func (w *Workspace) SceneDataFile() string {
	return filepath.Join(w.EpisodeTemp, "scenes.json")
}

// FramesDir returns: workspace/temp/<episodeID>/frames/
func (w *Workspace) FramesDir() string {
	return filepath.Join(w.EpisodeTemp, "frames")
}

// FrameFile returns: workspace/temp/<episodeID>/frames/frame_<index>.jpg
func (w *Workspace) FrameFile(index int) string {
	return filepath.Join(w.FramesDir(), fmt.Sprintf("frame_%04d.jpg", index))
}

// MasterPlanFile returns: workspace/temp/<episodeID>/master_plan.json
func (w *Workspace) MasterPlanFile() string {
	return filepath.Join(w.EpisodeTemp, "master_plan.json")
}

// VoiceoverFile returns: workspace/temp/<episodeID>/voiceover.mp3
func (w *Workspace) VoiceoverFile() string {
	return filepath.Join(w.EpisodeTemp, "voiceover.mp3")
}

// ClipFile returns: workspace/temp/<episodeID>/clip_<index>.mp4
func (w *Workspace) ClipFile(index int) string {
	return filepath.Join(w.EpisodeTemp, fmt.Sprintf("clip_%02d.mp4", index))
}

// ClipListFile returns: workspace/temp/<episodeID>/list.txt
func (w *Workspace) ClipListFile() string {
	return filepath.Join(w.EpisodeTemp, "list.txt")
}

// ConcatenatedVideo returns: workspace/temp/<episodeID>/concatenated.mp4
func (w *Workspace) ConcatenatedVideo() string {
	return filepath.Join(w.EpisodeTemp, "concatenated.mp4")
}

// SubtitleFile returns: workspace/temp/<episodeID>/subtitles.srt
func (w *Workspace) SubtitleFile() string {
	return filepath.Join(w.EpisodeTemp, "subtitles.srt")
}

// FinalOutputFile returns: workspace/output/FINAL_SHORTS_<episodeID>.mp4
func (w *Workspace) FinalOutputFile() string {
	return filepath.Join(w.OutputDir, fmt.Sprintf("FINAL_SHORTS_%s.mp4", w.EpisodeID))
}

// EnsureInputVideoExists checks whether the input video file exists.
func (w *Workspace) EnsureInputVideoExists() error {
	if _, err := os.Stat(w.InputFile()); os.IsNotExist(err) {
		return fmt.Errorf("input video not found: %s — please place %s.mp4 in the input directory", w.InputFile(), w.EpisodeID)
	}
	return nil
}
