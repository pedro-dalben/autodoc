package recipe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/version"
)

type Recipe struct {
	SchemaVersion  int               `json:"schema_version"`
	StoryboardHash string            `json:"storyboard_hash"`
	Meta           storyboard.Meta   `json:"meta"`
	Config         storyboard.Config `json:"config"`
	Redact         storyboard.Redact `json:"redact"`
	Scenes         []ScenePlan       `json:"scenes"`
	SpeechSegments []SpeechSegment   `json:"speech_segments"`
}

type ScenePlan struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	URL         string              `json:"url"`
	Screenshot  bool                `json:"screenshot"`
	SceneHash   string              `json:"scene_hash,omitempty"`
	Beats       []BeatPlan          `json:"beats"`
	SetupPrefix []storyboard.Action `json:"setup_prefix,omitempty"`
}

type BeatPlan struct {
	ID    string     `json:"id"`
	Steps []StepPlan `json:"steps"`
}

type StepKind string

const (
	StepSpeech StepKind = "speech"
	StepAction StepKind = "action"
	StepWait   StepKind = "wait"
	StepHold   StepKind = "hold"
)

type StepPlan struct {
	Kind     StepKind `json:"kind"`
	Index    int      `json:"index"`
	SpeechID string   `json:"speech_id,omitempty"`
	Text     string   `json:"text,omitempty"`
	// PauseBeforeMs/PauseAfterMs orchestrate pacing without rewriting text.
	PauseBeforeMs int                   `json:"pause_before_ms,omitempty"`
	PauseAfterMs  int                   `json:"pause_after_ms,omitempty"`
	SpeechAnchor  string                `json:"speech_anchor,omitempty"`
	Action        *storyboard.Action    `json:"action,omitempty"`
	Wait          *storyboard.WaitEvent `json:"wait,omitempty"`
	HoldMs        int                   `json:"hold_ms,omitempty"`
}

type SpeechSegment struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Hash string `json:"hash"`
}

func SpeechHash(text, voice, model, language string, speed float64) string {
	h := sha256.New()
	fmt.Fprintf(h, "tts-v1|%s|%s|%s|%s|%.3f", text, voice, model, language, speed)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func Compile(sb *storyboard.Storyboard, ttsModel, ttsVoice, ttsLang string, ttsSpeed float64) *Recipe {
	r := &Recipe{
		SchemaVersion:  version.StoryboardSchemaVer,
		StoryboardHash: sb.SourceHash(),
		Meta:           sb.Meta,
		Config:         sb.Config,
		Redact:         sb.Redact,
	}
	globalIdx := 0
	for _, sc := range sb.Scenes {
		sp := ScenePlan{ID: sc.ID, Title: sc.Title, URL: sc.URL, Screenshot: sc.Screenshot, SceneHash: sc.SceneHash()}
		if sc.URL != "" && len(sp.SetupPrefix) == 0 {
			_ = globalIdx
		}
		for _, b := range sc.Beats {
			bp := BeatPlan{ID: b.ID}
			speechIdx := 0
			for _, ev := range b.Sequence {
				switch {
				case ev.Speech != nil:
					sid := fmt.Sprintf("%s-%s-speech-%03d", sc.ID, b.ID, speechIdx+1)
					speechIdx++
					bp.Steps = append(bp.Steps, StepPlan{Kind: StepSpeech, Index: len(bp.Steps), SpeechID: sid, Text: ev.Speech.Text, PauseBeforeMs: ev.Speech.PauseBeforeMs, PauseAfterMs: ev.Speech.PauseAfterMs, SpeechAnchor: ev.Speech.Anchor})
					r.SpeechSegments = append(r.SpeechSegments, SpeechSegment{
						ID:   sid,
						Text: ev.Speech.Text,
						Hash: SpeechHash(ev.Speech.Text, firstNonEmpty(ev.Speech.Voice, ttsVoice), ttsModel, ttsLang, ttsSpeed),
					})
				case ev.Action != nil:
					a := *ev.Action
					bp.Steps = append(bp.Steps, StepPlan{Kind: StepAction, Index: len(bp.Steps), Action: &a})
				case ev.Wait != nil:
					w := *ev.Wait
					bp.Steps = append(bp.Steps, StepPlan{Kind: StepWait, Index: len(bp.Steps), Wait: &w})
				case ev.Hold != nil:
					bp.Steps = append(bp.Steps, StepPlan{Kind: StepHold, Index: len(bp.Steps), HoldMs: ev.Hold.DurationMs})
				}
			}
			sp.Beats = append(sp.Beats, bp)
		}
		r.Scenes = append(r.Scenes, sp)
	}
	return r
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (r *Recipe) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadJSON(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse recipe %s: %w", path, err)
	}
	return &r, nil
}

func (r *Recipe) SceneIDs() []string {
	out := make([]string, 0, len(r.Scenes))
	for _, s := range r.Scenes {
		out = append(out, s.ID)
	}
	return out
}

func (r *Recipe) FindScene(id string) *ScenePlan {
	for i := range r.Scenes {
		if r.Scenes[i].ID == id {
			return &r.Scenes[i]
		}
	}
	return nil
}
