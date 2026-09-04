// Package cinematic implements the AutoDoc Cinematic V2 AI Director /
// automatic tutorial editor.
//
// The pipeline is:
//
//	recipe (what to teach)
//	  -> ScenePlan (semantic beats: intent, anchor, action, result)
//	  -> browser execution (actual events, bboxes discovered at replay)
//	  -> AttentionDirector (how to present each beat)
//	  -> CameraDirector V2 (temporal camera with continuity)
//	  -> EditPlan / EDL (how raw footage becomes final video)
//	  -> Cinematic QA (objective gates + diagnostic score)
//
// Every decision is deterministic and testable: no LLM calls, no random
// timing, no hidden state. Rendering, timing, camera, safe areas,
// collision handling and QA reproduce byte-for-byte from the same inputs.
package cinematic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

// SceneType classifies a semantic beat. The planner derives it from the
// storyboard sequence; overrides stay available for special cases.
type SceneType string

const (
	SceneNarration      SceneType = "narration-only"
	SceneExplanation    SceneType = "explanation"
	SceneNavigation     SceneType = "navigation"
	SceneClickAction    SceneType = "click_action"
	SceneTypingAction   SceneType = "typing_action"
	SceneSelection      SceneType = "selection"
	SceneSubmit         SceneType = "submit"
	SceneResult         SceneType = "result"
	SceneConfirmation   SceneType = "confirmation"
	SceneLoading        SceneType = "loading"
	SceneModal          SceneType = "modal"
	ScenePageTransition SceneType = "page_transition"
	SceneCompletion     SceneType = "completion"
)

// AnchorKind names what justifies the narration visually.
type AnchorKind string

const (
	AnchorNone      AnchorKind = "none"
	AnchorTarget    AnchorKind = "target"
	AnchorContainer AnchorKind = "container"
	AnchorResult    AnchorKind = "result"
	AnchorViewport  AnchorKind = "viewport"
	AnchorModal     AnchorKind = "modal"
	AnchorForm      AnchorKind = "form"
	AnchorGroup     AnchorKind = "group"
)

// VisualAnchor binds narration to something on screen.
type VisualAnchor struct {
	Kind     AnchorKind `json:"kind"`
	Describe string     `json:"describe"`
	// TargetKey is the storyboard target describe string when known.
	TargetKey string `json:"target_key,omitempty"`
}

// BeatPlan is one semantic beat: intent + anchor + action + result.
type BeatPlan struct {
	SceneID     string       `json:"scene_id"`
	BeatID      string       `json:"beat_id"`
	Index       int          `json:"index"`
	Type        SceneType    `json:"type"`
	Intent      string       `json:"intent"`
	Narration   string       `json:"narration,omitempty"`
	SpeechID    string       `json:"speech_id,omitempty"`
	Anchor      VisualAnchor `json:"anchor"`
	ActionType  string       `json:"action_type,omitempty"`
	ActionLabel string       `json:"action_label,omitempty"`
	// ExpectedResult carries the declared or inferred result target.
	ExpectedResult string `json:"expected_result,omitempty"`
	// NeedsAnticipation marks beats where pre-action direction applies.
	NeedsAnticipation bool `json:"needs_anticipation"`
	// NeedsConfirmation marks beats whose result must be held/shown.
	NeedsConfirmation bool   `json:"needs_confirmation"`
	CameraOverride    string `json:"camera_override,omitempty"`
	AttentionOverride string `json:"attention_override,omitempty"`
	Callout           string `json:"callout,omitempty"`
}

// ScenePlanDoc is the serializable scene_plan.json artifact.
type ScenePlanDoc struct {
	StoryboardHash string     `json:"storyboard_hash"`
	Beats          []BeatPlan `json:"beats"`
}

// ClassifyBeat derives the semantic type from neighboring steps.
// Pure function of (prevHasSpeech, step, nextStep): deterministic.
func ClassifyBeat(prevIsSpeech bool, cur recipe.StepPlan, next *recipe.StepPlan, hasModalTarget bool) SceneType {
	switch cur.Kind {
	case recipe.StepSpeech:
		if next != nil && next.Kind == recipe.StepAction {
			return SceneExplanation
		}
		if prevIsSpeech {
			return SceneNarration
		}
		return SceneExplanation
	case recipe.StepAction:
		t := ""
		if cur.Action != nil {
			t = cur.Action.Type
		}
		switch t {
		case "goto", "reload", "goback":
			return SceneNavigation
		case "fill", "type":
			return SceneTypingAction
		case "select", "check", "uncheck":
			return SceneSelection
		case "press":
			if next != nil && next.Kind == recipe.StepWait {
				return SceneSubmit
			}
			return SceneClickAction
		case "click":
			if hasModalTarget {
				return SceneModal
			}
			if next != nil && next.Kind == recipe.StepWait {
				return SceneSubmit
			}
			return SceneClickAction
		case "hover":
			return SceneExplanation
		default:
			return SceneClickAction
		}
	case recipe.StepWait:
		st := ""
		if cur.Wait != nil {
			st = strings.ToLower(cur.Wait.State)
		}
		if st == "url" || st == "load" {
			return ScenePageTransition
		}
		return SceneLoading
	case recipe.StepHold:
		return SceneConfirmation
	}
	return SceneNarration
}

// InferAnchor derives the visual anchor for a beat.
func InferAnchor(cur recipe.StepPlan, neighborAction *recipe.StepPlan, speechAnchor string) VisualAnchor {
	if speechAnchor != "" {
		if speechAnchor == "viewport" {
			return VisualAnchor{Kind: AnchorViewport, Describe: "viewport"}
		}
		return VisualAnchor{Kind: AnchorGroup, Describe: speechAnchor}
	}
	pick := cur
	if cur.Kind == recipe.StepSpeech && neighborAction != nil {
		pick = *neighborAction
	}
	if pick.Action != nil && pick.Action.Target != nil && !pick.Action.Target.Empty() {
		t := pick.Action.Target
		desc := t.Describe()
		kind := AnchorTarget
		if pick.Action.Type == "fill" || pick.Action.Type == "type" || pick.Action.Type == "select" {
			kind = AnchorForm
		}
		if pick.Action.ResultTarget != nil && !pick.Action.ResultTarget.Empty() {
			return VisualAnchor{Kind: AnchorResult, Describe: pick.Action.ResultTarget.Describe(), TargetKey: desc}
		}
		if cur.Kind == recipe.StepWait && pick.Kind == recipe.StepWait {
			return VisualAnchor{Kind: AnchorContainer, Describe: desc, TargetKey: desc}
		}
		return VisualAnchor{Kind: kind, Describe: desc, TargetKey: desc}
	}
	if pick.Action != nil && pick.Action.Type == "goto" {
		u := pick.Action.URL
		if u == "" {
			u = "page"
		}
		return VisualAnchor{Kind: AnchorViewport, Describe: u}
	}
	if cur.Kind == recipe.StepWait {
		return VisualAnchor{Kind: AnchorContainer, Describe: "loading state"}
	}
	if cur.Kind == recipe.StepHold {
		return VisualAnchor{Kind: AnchorViewport, Describe: "result hold"}
	}
	return VisualAnchor{Kind: AnchorNone, Describe: "none"}
}

// beatKey joins scene+beat for segment/beat correlation. The reconciler
// preserves recipe beat IDs on every segment, so direction matches on
// keys — never on free-text labels (selector vs describe forms differ).
func beatKey(scene, beat string) string { return scene + "|" + beat }

// indexBeats groups beat plans by scene|beat, preserving step order.
func indexBeats(beats []BeatPlan) map[string][]BeatPlan {
	m := map[string][]BeatPlan{}
	for _, b := range beats {
		k := beatKey(b.SceneID, b.BeatID)
		m[k] = append(m[k], b)
	}
	return m
}

// beatForAction resolves the semantic beat for an action segment.
func beatForAction(idx map[string][]BeatPlan, scene, beat, actionType string) BeatPlan {
	for _, b := range idx[beatKey(scene, beat)] {
		if b.ActionType == actionType {
			return b
		}
	}
	for _, b := range idx[beatKey(scene, beat)] {
		if b.ActionType != "" {
			return b
		}
	}
	return BeatPlan{}
}

// beatForSpeech resolves the semantic beat for a speech segment.
func beatForSpeech(idx map[string][]BeatPlan, scene, beat, speechID string) BeatPlan {
	for _, b := range idx[beatKey(scene, beat)] {
		if b.SpeechID == speechID {
			return b
		}
	}
	return BeatPlan{}
}

// IntentFor renders a short human intent line for a beat.
func IntentFor(bt SceneType, narration, actionLabel string) string {
	n := strings.TrimSpace(narration)
	if len(n) > 90 {
		n = n[:87] + "..."
	}
	switch bt {
	case SceneNarration:
		return fmt.Sprintf("explain without interaction (%s)", n)
	case SceneExplanation:
		return fmt.Sprintf("explain anchored to UI (%s)", n)
	case SceneNavigation:
		return fmt.Sprintf("navigate to %s", actionLabel)
	case SceneClickAction:
		return fmt.Sprintf("teach click on %s", actionLabel)
	case SceneTypingAction:
		return fmt.Sprintf("teach typing into %s", actionLabel)
	case SceneSelection:
		return fmt.Sprintf("teach selection on %s", actionLabel)
	case SceneSubmit:
		return fmt.Sprintf("submit %s and await result", actionLabel)
	case SceneLoading:
		return "bridge loading time without losing attention"
	case ScenePageTransition:
		return "reorient after page transition"
	case SceneModal:
		return fmt.Sprintf("operate modal via %s", actionLabel)
	case SceneConfirmation:
		return "hold result so the viewer can read it"
	case SceneCompletion:
		return "close with full context"
	default:
		return fmt.Sprintf("present %s", actionLabel)
	}
}

// PlanScenes transforms recipe steps into semantic beats.
func PlanScenes(r *recipe.Recipe, sb *storyboard.Storyboard) *ScenePlanDoc {
	doc := &ScenePlanDoc{StoryboardHash: r.StoryboardHash}
	sceneCam := map[string]string{}
	sceneAtt := map[string]string{}
	if sb != nil {
		for _, sc := range sb.Scenes {
			if sc.Camera != "" {
				sceneCam[sc.ID] = sc.Camera
			}
			if sc.Attention != "" {
				sceneAtt[sc.ID] = sc.Attention
			}
		}
	}
	idx := 0
	lastBeatIsCompletion := map[string]bool{}
	for _, sc := range r.Scenes {
		prevIsSpeech := false
		flat := []*recipe.StepPlan{}
		for bi := range sc.Beats {
			for si := range sc.Beats[bi].Steps {
				flat = append(flat, &sc.Beats[bi].Steps[si])
			}
		}
		beatOf := map[*recipe.StepPlan]string{}
		for i := range sc.Beats {
			for j := range sc.Beats[i].Steps {
				beatOf[&sc.Beats[i].Steps[j]] = sc.Beats[i].ID
			}
		}
		for i, st := range flat {
			var next *recipe.StepPlan
			if i+1 < len(flat) {
				next = flat[i+1]
			}
			var neighborAction *recipe.StepPlan
			if st.Kind == recipe.StepSpeech && next != nil && next.Kind == recipe.StepAction {
				neighborAction = next
			}
			if st.Kind != recipe.StepSpeech && st.Kind != recipe.StepAction && st.Kind != recipe.StepWait && st.Kind != recipe.StepHold {
				continue
			}
			hasModal := false
			if st.Action != nil && st.Action.Target != nil && st.Action.Target.Role == "dialog" {
				hasModal = true
			}
			bt := ClassifyBeat(prevIsSpeech, *st, next, hasModal)
			var narration, speechID string
			if st.Kind == recipe.StepSpeech {
				narration, speechID = st.Text, st.SpeechID
			} else if next != nil && next.Kind == recipe.StepSpeech && st.Kind == recipe.StepAction {
				// interaction beats borrow the following narration as intent
				// context without rewriting content.
				narration = next.Text
			}
			actionType, actionLabel := "", ""
			callout := ""
			camOv, attOv := sceneCam[sc.ID], sceneAtt[sc.ID]
			expResult := ""
			needsConf := false
			if st.Action != nil {
				actionType = st.Action.Type
				actionLabel = actionType
				if st.Action.Target != nil {
					actionLabel += " " + st.Action.Target.Describe()
				}
				callout = st.Action.Callout
				if st.Action.Camera != "" {
					camOv = st.Action.Camera
				}
				if st.Action.Attention != "" {
					attOv = st.Action.Attention
				}
				if st.Action.ResultTarget != nil && !st.Action.ResultTarget.Empty() {
					expResult = st.Action.ResultTarget.Describe()
					needsConf = true
				} else if bt == SceneSubmit {
					expResult = "post-submit state"
					needsConf = true
				}
			}
			anchor := InferAnchor(*st, neighborAction, st.SpeechAnchor)
			bp := BeatPlan{
				SceneID: sc.ID, BeatID: beatOf[st], Index: idx,
				Type: bt, Intent: IntentFor(bt, narration, actionLabel),
				Narration: narration, SpeechID: speechID, Anchor: anchor,
				ActionType: actionType, ActionLabel: actionLabel,
				ExpectedResult:    expResult,
				NeedsAnticipation: st.Kind == recipe.StepAction && (actionType == "click" || actionType == "fill" || actionType == "type" || actionType == "select" || actionType == "press"),
				NeedsConfirmation: needsConf,
				CameraOverride:    camOv, AttentionOverride: attOv, Callout: callout,
			}
			doc.Beats = append(doc.Beats, bp)
			idx++
			prevIsSpeech = st.Kind == recipe.StepSpeech
			_ = lastBeatIsCompletion
		}
	}
	// Mark the final hold/beat of the last scene as completion for the
	// context-restore exit.
	if n := len(doc.Beats); n > 0 {
		last := &doc.Beats[n-1]
		if last.Type == SceneConfirmation {
			last.Type = SceneCompletion
			last.Intent = IntentFor(SceneCompletion, last.Narration, last.ActionLabel)
		}
	}
	return doc
}

func (d *ScenePlanDoc) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
