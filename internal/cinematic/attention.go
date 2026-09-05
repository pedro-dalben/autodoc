package cinematic

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/pedro-dalben/autodoc/internal/visual"
)

// AttentionStrategy decides HOW a beat is presented visually.
// The director never performs business actions.
type AttentionStrategy string

const (
	AttStay           AttentionStrategy = "stay"
	AttFocus          AttentionStrategy = "focus"
	AttSoftZoom       AttentionStrategy = "soft_zoom"
	AttPan            AttentionStrategy = "pan"
	AttPanZoom        AttentionStrategy = "pan_zoom"
	AttSpotlight      AttentionStrategy = "spotlight"
	AttHighlight      AttentionStrategy = "highlight"
	AttFollow         AttentionStrategy = "follow"
	AttContextRestore AttentionStrategy = "context_restore"
	AttNone           AttentionStrategy = "none"
)

// AttentionDecision is one beat's presentation verdict.
type AttentionDecision struct {
	BeatIndex  int               `json:"beat_index"`
	SceneID    string            `json:"scene_id"`
	BeatID     string            `json:"beat_id"`
	BeatType   SceneType         `json:"beat_type"`
	Strategy   AttentionStrategy `json:"strategy"`
	Reason     string            `json:"reason"`
	Spotlight  bool              `json:"spotlight"`
	Anticipate bool              `json:"anticipate"`
}

// AttentionPlan is the serializable cinematic_plan.json attention section.
type AttentionPlan struct {
	Decisions []AttentionDecision `json:"decisions"`
}

// DirectAttention maps every semantic beat to a presentation strategy.
// Deterministic rules, in priority order:
//
//  1. explicit per-action/scene override wins ("none" disables direction);
//  2. narration-only without anchor -> stay (flagged later by QA, never
//     masked with gratuitous motion);
//  3. explanation with anchor -> focus (camera holds, no travel);
//  4. click/typing/selection/submit -> spotlight+highlight with
//     anticipation; far targets upgrade to pan/pan_zoom by the camera
//     director, not here;
//  5. loading/page transition -> stay (temporal editor owns the cut);
//  6. confirmation/completion -> context_restore when the next beat needs
//     full context, else stay.
func DirectAttention(beats []BeatPlan, cfg visual.CinematicConfig) *AttentionPlan {
	p := &AttentionPlan{}
	if !cfg.AttentionOn() {
		for _, b := range beats {
			p.Decisions = append(p.Decisions, AttentionDecision{
				BeatIndex: b.Index, SceneID: b.SceneID, BeatID: b.BeatID,
				BeatType: b.Type, Strategy: AttNone, Reason: "attention-disabled",
			})
		}
		return p
	}
	for i, b := range beats {
		if ov := AttentionStrategy(b.AttentionOverride); ov != "" {
			p.Decisions = append(p.Decisions, AttentionDecision{
				BeatIndex: b.Index, SceneID: b.SceneID, BeatID: b.BeatID,
				BeatType: b.Type, Strategy: ov, Reason: "explicit-override",
				Spotlight:  ov == AttSpotlight && cfg.SpotlightOn(),
				Anticipate: b.NeedsAnticipation && ov != AttNone,
			})
			continue
		}
		dec := AttentionDecision{
			BeatIndex: b.Index, SceneID: b.SceneID, BeatID: b.BeatID, BeatType: b.Type,
		}
		switch b.Type {
		case SceneNarration:
			if b.Anchor.Kind == AnchorNone {
				dec.Strategy, dec.Reason = AttStay, "narration-without-anchor-hold"
			} else {
				dec.Strategy, dec.Reason = AttFocus, "anchored-explanation"
			}
		case SceneExplanation:
			dec.Strategy, dec.Reason = AttFocus, "anchor-holds-camera"
		case SceneClickAction, SceneTypingAction, SceneSelection:
			dec.Strategy, dec.Reason = AttSpotlight, "interaction-anchor"
			dec.Spotlight = cfg.SpotlightOn()
			dec.Anticipate = cfg.AnticipationOn() && b.NeedsAnticipation
		case SceneSubmit, SceneModal:
			dec.Strategy, dec.Reason = AttFollow, "track-action-to-result"
			dec.Spotlight = cfg.SpotlightOn()
			dec.Anticipate = cfg.AnticipationOn() && b.NeedsAnticipation
		case SceneNavigation, ScenePageTransition, SceneLoading, SceneResult:
			dec.Strategy, dec.Reason = AttStay, "temporal-editor-owns-transition"
		case SceneConfirmation:
			if nextNeedsContext(beats, i) && cfg.ContextRestoreOn() {
				dec.Strategy, dec.Reason = AttContextRestore, "restore-after-focus"
			} else {
				dec.Strategy, dec.Reason = AttStay, "intentional-static-read"
			}
		case SceneCompletion:
			dec.Strategy, dec.Reason = AttContextRestore, "final-context-restore"
		default:
			dec.Strategy, dec.Reason = AttStay, "default-hold"
		}
		// Explicit spotlight direction overrides the automatic choice:
		// off kills the dim (strategy untouched), an intensity forces it.
		if b.SpotlightMode == "off" {
			dec.Spotlight = false
			dec.Reason += "+spotlight-off"
		} else if b.SpotlightMode == "subtle" || b.SpotlightMode == "medium" || b.SpotlightMode == "strong" {
			if cfg.AttentionOn() {
				dec.Spotlight = true
			}
			dec.Reason += "+spotlight-" + b.SpotlightMode
		}
		p.Decisions = append(p.Decisions, dec)
	}
	applyAttentionBudget(p, beats)
	return p
}

// applyAttentionBudget prevents the repeated zoom/spotlight choreography that
// makes a deterministic tutorial feel mechanical. Explicit storyboard
// overrides stay authoritative; ordinary interaction beats get at most about
// 60% visual interventions per scene, with one orientation allowance.
func applyAttentionBudget(p *AttentionPlan, beats []BeatPlan) {
	actions := map[string]int{}
	for _, b := range beats {
		if b.NeedsAnticipation {
			actions[b.SceneID]++
		}
	}
	limit := map[string]int{}
	for scene, n := range actions {
		limit[scene] = int(math.Ceil(float64(n)*0.6)) + 1
	}
	used := map[string]int{}
	for i := range p.Decisions {
		d := &p.Decisions[i]
		if d.Strategy != AttSpotlight && d.Strategy != AttFollow {
			continue
		}
		if d.BeatIndex >= 0 && d.BeatIndex < len(beats) && (beats[d.BeatIndex].AttentionOverride != "" || beats[d.BeatIndex].SpotlightMode != "" || beats[d.BeatIndex].CameraOverride == "stay") {
			continue
		}
		used[d.SceneID]++
		if used[d.SceneID] > limit[d.SceneID] {
			d.Strategy, d.Spotlight, d.Anticipate, d.Reason = AttFocus, false, false, "attention-budget"
		}
	}
}

func nextNeedsContext(beats []BeatPlan, i int) bool {
	if i+1 >= len(beats) {
		return true
	}
	switch beats[i+1].Type {
	case SceneExplanation, SceneNarration, SceneCompletion, SceneConfirmation:
		return true
	}
	// A far jump between two interaction anchors also wants a restore
	// first; the camera director refines this with real geometry.
	if beats[i+1].Anchor.TargetKey != "" && beats[i].Anchor.TargetKey != "" &&
		beats[i+1].Anchor.TargetKey != beats[i].Anchor.TargetKey {
		return true
	}
	return false
}

// AnticipationEnvelope carries the pre-action timing budget in ms.
type AnticipationEnvelope struct {
	RevealMs   int `json:"reveal_ms"`
	ApproachMs int `json:"approach_ms"`
	SettleMs   int `json:"settle_ms"`
	TotalMs    int `json:"total_ms"`
}

// AnticipationFor returns the deterministic envelope for an action beat.
// Values come from config (tested ranges 300-700 reveal, 200-500
// approach, >=200 settle) and never exceed them.
func AnticipationFor(cfg visual.CinematicConfig, beat BeatPlan, cursorDistPx float64) AnticipationEnvelope {
	env := AnticipationEnvelope{
		RevealMs: cfg.Anticipation.RevealMs,
		SettleMs: cfg.Anticipation.SettleMs,
	}
	// Scale the approach by distance, clamped to [200, ApproachMs].
	ms := 180 + cursorDistPx*0.25
	if ms < 200 {
		ms = 200
	}
	if ms > float64(cfg.Anticipation.ApproachMs) {
		ms = float64(cfg.Anticipation.ApproachMs)
	}
	env.ApproachMs = int(math.Round(ms))
	env.TotalMs = env.RevealMs + env.ApproachMs + env.SettleMs
	return env
}

func (p *AttentionPlan) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
