package cinematic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// ReadableResultHold returns a deterministic reading hold. Result text is the
// evidence available before rendering; richer real UI evidence can raise this
// value later without ever shortening below the configured safety floor.
func ReadableResultHold(result string, cfg visual.CinematicConfig) int {
	hold := cfg.Results.MinHoldMs + len([]rune(result))*28
	if hold < cfg.Results.MinHoldMs {
		return cfg.Results.MinHoldMs
	}
	if hold > 2600 {
		return 2600
	}
	return hold
}

// WaitClass classifies dead time for the automatic editor.
type WaitClass string

const (
	WaitSemantic         WaitClass = "semantic"
	WaitTechnical        WaitClass = "technical"
	WaitLoading          WaitClass = "loading"
	WaitTransition       WaitClass = "transition"
	WaitUserReadable     WaitClass = "user-readable"
	WaitNarrationCovered WaitClass = "narration-covered"
)

// EditDecision decides what happens to a wait/action window.
type EditDecision string

const (
	EditKeep      EditDecision = "keep"
	EditCompress  EditDecision = "compress"
	EditSpeedRamp EditDecision = "speed_ramp"
	EditCut       EditDecision = "cut"
	EditFreeze    EditDecision = "freeze"
	EditCrossfade EditDecision = "cross_transition"
)

// ClipType is one EDL clip in the final video.
type ClipType string

const (
	ClipEstablish    ClipType = "establish"
	ClipAnticipation ClipType = "anticipation"
	ClipAction       ClipType = "action"
	ClipTransition   ClipType = "transition"
	ClipResult       ClipType = "result"
	ClipHold         ClipType = "hold"
	ClipNarration    ClipType = "narration"
)

// Clip is a single edit-decision-list entry.
type Clip struct {
	SceneID    string       `json:"scene_id"`
	BeatID     string       `json:"beat_id,omitempty"`
	Type       ClipType     `json:"type"`
	DurationMs int          `json:"duration_ms,omitempty"`
	HoldMs     int          `json:"hold_ms,omitempty"`
	Strategy   EditDecision `json:"strategy,omitempty"`
	Speed      float64      `json:"speed,omitempty"`
	Label      string       `json:"label,omitempty"`
}

// EditPlan is the explicit intermediate representation separating
// "what happened" (final timeline) from "how it should be edited".
type EditPlan struct {
	StoryboardHash string      `json:"storyboard_hash"`
	SceneEdits     []SceneEdit `json:"scenes"`
	Clips          []Clip      `json:"clips"`
	WaitsSavedMs   int         `json:"waits_saved_ms"`
	HoldsAddedMs   int         `json:"holds_added_ms"`
}

// SceneEdit groups clips per scene.
type SceneEdit struct {
	SceneID string `json:"scene_id"`
	Clips   []Clip `json:"clips"`
}

// ClassifyWait maps a reconciled wait segment to its semantic class.
func ClassifyWait(label string, compressible bool, actualS float64, speechOverlap bool) WaitClass {
	l := strings.ToLower(label)
	if speechOverlap {
		return WaitNarrationCovered
	}
	switch l {
	case "url", "load":
		return WaitTransition
	case "networkidle":
		return WaitTechnical
	case "visible", "attached":
		if actualS > 1.5 {
			return WaitLoading
		}
		return WaitSemantic
	case "hidden", "detached":
		return WaitTransition
	case "timeout", "settle":
		return WaitUserReadable
	default:
		if actualS > 2.0 {
			return WaitLoading
		}
		return WaitSemantic
	}
}

// DecideWait maps class + duration to an edit decision. Never compresses
// narration (callers only pass waits/holds), never invents causality:
// cuts apply only to redundant settle padding, never across an action.
func DecideWait(class WaitClass, actualS float64, compressible bool, cfg visual.CinematicConfig) (EditDecision, float64) {
	if !compressible || !cfg.CompressOn() {
		return EditKeep, 1
	}
	switch class {
	case WaitLoading, WaitTechnical:
		if actualS <= 1.5 {
			return EditKeep, 1
		}
		return EditCompress, 1
	case WaitTransition:
		if actualS <= 1.0 {
			return EditKeep, 1
		}
		return EditSpeedRamp, 1
	case WaitUserReadable:
		if actualS <= 1.0 {
			return EditFreeze, 1
		}
		return EditKeep, 1
	case WaitSemantic, WaitNarrationCovered:
		return EditKeep, 1
	default:
		return EditKeep, 1
	}
}

// BuildEditPlan derives the EDL from the reconciled timeline + beats.
func BuildEditPlan(ft *timeline.FinalTimeline, beats []BeatPlan, cfg visual.CinematicConfig) *EditPlan {
	plan := &EditPlan{StoryboardHash: ft.StoryboardHash}
	byScene := map[string]*SceneEdit{}
	sceneOrder := []string{}
	get := func(scene string) *SceneEdit {
		if se, ok := byScene[scene]; ok {
			return se
		}
		se := &SceneEdit{SceneID: scene}
		byScene[scene] = se
		sceneOrder = append(sceneOrder, scene)
		return se
	}
	idx := indexBeats(beats)
	beatOf := func(sg timeline.AVSegment) BeatPlan {
		switch sg.Kind {
		case "action":
			return beatForAction(idx, sg.SceneID, sg.BeatID, sg.Label)
		case "speech":
			return beatForSpeech(idx, sg.SceneID, sg.BeatID, sg.Label)
		default:
			if bs := idx[beatKey(sg.SceneID, sg.BeatID)]; len(bs) > 0 {
				return bs[0]
			}
			return BeatPlan{}
		}
	}
	for _, sg := range ft.Segments {
		se := get(sg.SceneID)
		b := beatOf(sg)
		switch sg.Kind {
		case "speech":
			se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipNarration, DurationMs: int(sg.DurS * 1000), Label: sg.Label})
		case "action":
			if b.NeedsAnticipation && cfg.AnticipationOn() {
				env := AnticipationFor(cfg, b, 400)
				se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipAnticipation, DurationMs: env.TotalMs, Label: b.ActionLabel})
			}
			se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipAction, DurationMs: int(sg.DurS * 1000), Label: sg.Label})
			if b.NeedsConfirmation && cfg.ConfirmationOn() {
				hold := ReadableResultHold(b.ExpectedResult, cfg)
				se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipResult, HoldMs: hold, DurationMs: hold, Strategy: EditKeep, Label: "result:" + b.ExpectedResult})
			}
		case "wait":
			actual := sg.VideoEndS - sg.VideoStartS
			class := ClassifyWait(sg.Label, sg.Compressible, actual, false)
			dec, _ := DecideWait(class, actual, sg.Compressible, cfg)
			speed := sg.Speed
			if speed < 1 {
				speed = 1
			}
			se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipTransition, DurationMs: int(sg.DurS * 1000), Strategy: dec, Speed: speed, Label: string(class) + ":" + sg.Label})
			if sg.Compressed {
				plan.WaitsSavedMs += int((actual - sg.DurS) * 1000)
			}
		case "hold":
			se.Clips = append(se.Clips, Clip{SceneID: sg.SceneID, BeatID: b.BeatID, Type: ClipHold, DurationMs: int(sg.DurS * 1000), HoldMs: int(sg.DurS * 1000), Strategy: EditKeep, Label: sg.Label})
		}
	}
	// Establish clip: first speech of each scene opens with orientation.
	for _, id := range sceneOrder {
		se := byScene[id]
		if len(se.Clips) > 0 && se.Clips[0].Type == ClipNarration {
			se.Clips[0].Type = ClipEstablish
		}
		plan.SceneEdits = append(plan.SceneEdits, *se)
		plan.Clips = append(plan.Clips, se.Clips...)
	}
	return plan
}

// EnsureResultHolds enforces minimum result visibility by extending
// video-only action windows (never speech: causality and sync preserved).
// Returns added milliseconds.
func EnsureResultHolds(ft *timeline.FinalTimeline, beats []BeatPlan, cfg visual.CinematicConfig) int {
	if !cfg.ConfirmationOn() {
		return 0
	}
	added := 0
	idx := indexBeats(beats)
	need := func(sg timeline.AVSegment) (int, bool) {
		b := beatForAction(idx, sg.SceneID, sg.BeatID, sg.Label)
		if b.NeedsConfirmation {
			return ReadableResultHold(b.ExpectedResult, cfg), true
		}
		return 0, false
	}
	// Walk segments; when an action needs confirmation, guarantee the
	// action window itself plus the immediately following hold covers
	// MinHoldMs by stretching the action DurS (tpad clone in renderer).
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind != "action" {
			continue
		}
		minHold, ok := need(*sg)
		if !ok {
			continue
		}
		cover := sg.DurS
		if i+1 < len(ft.Segments) && (ft.Segments[i+1].Kind == "hold" || ft.Segments[i+1].Kind == "wait") {
			cover += ft.Segments[i+1].DurS
		}
		want := float64(minHold) / 1000.0
		if cover < want {
			sg.DurS += want - cover
			added += int((want - cover) * 1000)
		}
	}
	if added > 0 {
		// Reflow StartS and totals (speech positions shift deterministically).
		t := 0.0
		for i := range ft.Segments {
			ft.Segments[i].StartS = t
			t += ft.Segments[i].DurS
		}
		ft.TotalS = t
		// Reflow speech extents to match their AV segments.
		si := 0
		for _, sg := range ft.Segments {
			if sg.Kind == "speech" && si < len(ft.Speeches) {
				ft.Speeches[si].StartS = sg.StartS
				ft.Speeches[si].EndS = sg.StartS + sg.DurS
				si++
			}
		}
		for i := range ft.SceneClips {
			var s0, s1 float64
			first := true
			for _, sg := range ft.Segments {
				if sg.SceneID == ft.SceneClips[i].SceneID {
					if first {
						s0 = sg.StartS
						first = false
					}
					s1 = sg.StartS + sg.DurS
				}
			}
			ft.SceneClips[i].StartS, ft.SceneClips[i].EndS, ft.SceneClips[i].Duration = s0, s1, s1-s0
		}
	}
	return added
}

func (p *EditPlan) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
