package timeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedro-dalben/autodoc/internal/recipe"
)

type Segment struct {
	SpeechID  string  `json:"speech_id"`
	Text      string  `json:"text"`
	StartS    float64 `json:"start_s"`
	EndS      float64 `json:"end_s"`
	DurationS float64 `json:"duration_s"`
	Cached    bool    `json:"cached"`
	WavPath   string  `json:"wav_path"`
}

type ClipMarker struct {
	SceneID string  `json:"scene_id"`
	BeatID  string  `json:"beat_id,omitempty"`
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	AtS     float64 `json:"at_s"`
}

type Timeline struct {
	StoryboardHash string       `json:"storyboard_hash"`
	TotalS         float64      `json:"total_s"`
	Segments       []Segment    `json:"segments"`
	Markers        []ClipMarker `json:"markers"`
	SceneClips     []SceneClip  `json:"scene_clips"`
	HoldsS         float64      `json:"holds_s"`
	SpeechS        float64      `json:"speech_s"`
}

type SceneClip struct {
	SceneID  string  `json:"scene_id"`
	StartS   float64 `json:"start_s"`
	EndS     float64 `json:"end_s"`
	Duration float64 `json:"duration_s"`
}

const (
	actionPadS   = 0.0
	settleDefS   = 0.6
	holdMinGapMs = 0
)

type DurationFunc func(speechID string) (float64, bool, string)

func Build(r *recipe.Recipe, durFn DurationFunc, defaultSettleMs int) (*Timeline, error) {
	tl := &Timeline{StoryboardHash: r.StoryboardHash}
	if defaultSettleMs <= 0 {
		defaultSettleMs = 600
	}
	t := 0.0
	segIdx := map[string]int{}
	for si := range r.Scenes {
		sc := &r.Scenes[si]
		clipStart := t
		for bi := range sc.Beats {
			b := &sc.Beats[bi]
			for _, st := range b.Steps {
				switch st.Kind {
				case recipe.StepSpeech:
					dur, cached, wav := durFn(st.SpeechID)
					if dur <= 0 {
						return nil, fmt.Errorf("no duration for speech %s", st.SpeechID)
					}
					seg := Segment{SpeechID: st.SpeechID, Text: st.Text, StartS: t, EndS: t + dur, DurationS: dur, Cached: cached, WavPath: wav}
					tl.Segments = append(tl.Segments, seg)
					segIdx[st.SpeechID] = len(tl.Segments) - 1
					tl.Markers = append(tl.Markers, ClipMarker{SceneID: sc.ID, BeatID: b.ID, Kind: "speech", Label: st.SpeechID, AtS: t})
					t += dur
					tl.SpeechS += dur
				case recipe.StepAction:
					label := st.Action.Type
					if st.Action.Target != nil {
						label += " " + st.Action.Target.Describe()
					}
					tl.Markers = append(tl.Markers, ClipMarker{SceneID: sc.ID, BeatID: b.ID, Kind: "action", Label: label, AtS: t})
					t += actionPadS
				case recipe.StepWait:
					settle := float64(st.Wait.SettleMs) / 1000.0
					if st.Wait.SettleMs == 0 {
						settle = float64(defaultSettleMs) / 1000.0
					}
					tl.Markers = append(tl.Markers, ClipMarker{SceneID: sc.ID, BeatID: b.ID, Kind: "wait", Label: st.Wait.State, AtS: t})
					t += settle
				case recipe.StepHold:
					hold := float64(st.HoldMs) / 1000.0
					tl.Markers = append(tl.Markers, ClipMarker{SceneID: sc.ID, BeatID: b.ID, Kind: "hold", Label: fmt.Sprintf("%dms", st.HoldMs), AtS: t})
					t += hold
					tl.HoldsS += hold
				}
			}
		}
		clip := SceneClip{SceneID: sc.ID, StartS: clipStart, EndS: t, Duration: t - clipStart}
		tl.SceneClips = append(tl.SceneClips, clip)
	}
	tl.TotalS = t
	return tl, nil
}

func (t *Timeline) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadJSON(path string) (*Timeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Timeline
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (t *Timeline) SpeechAt(timeS float64) *Segment {
	for i := range t.Segments {
		if timeS >= t.Segments[i].StartS && timeS < t.Segments[i].EndS {
			return &t.Segments[i]
		}
	}
	return nil
}
