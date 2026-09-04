package cinematic

import (
	"github.com/pedro-dalben/autodoc/internal/recipe"
)

// PausePlan models narration pacing separately from TTS text.
// Content is never rewritten: pauses are orchestration events the
// recorder sleeps through on the narration clock.
type PausePlan struct {
	// BeforeMs maps speech_id -> pause before the speech starts.
	BeforeMs map[string]int `json:"before_ms"`
	// AfterMs maps speech_id -> pause after the speech ends.
	AfterMs map[string]int `json:"after_ms"`
	// Gaps lists human-readable concept/action/result pauses.
	Gaps []PauseGap `json:"gaps"`
}

// PauseGap is one planned silence with its rationale.
type PauseGap struct {
	SpeechID string `json:"speech_id"`
	Position string `json:"position"` // before | after
	Ms       int    `json:"ms"`
	Reason   string `json:"reason"`
}

// PlanPauses derives the pause plan from the recipe: explicit storyboard
// pauses win; otherwise concept->action gets a beat gap, and results get
// a breath before the confirmation lands. Capped at 1200ms per gap.
func PlanPauses(r *recipe.Recipe, speechGapMs, actionGapMs int) *PausePlan {
	p := &PausePlan{BeforeMs: map[string]int{}, AfterMs: map[string]int{}}
	if speechGapMs <= 0 {
		speechGapMs = 250
	}
	if actionGapMs <= 0 {
		actionGapMs = 450
	}
	for _, sc := range r.Scenes {
		for _, b := range sc.Beats {
			for i, st := range b.Steps {
				if st.Kind != recipe.StepSpeech {
					continue
				}
				before, after := st.PauseBeforeMs, st.PauseAfterMs
				var prev, next *recipe.StepPlan
				if i > 0 {
					prev = &b.Steps[i-1]
				}
				if i+1 < len(b.Steps) {
					next = &b.Steps[i+1]
				}
				// Default concept/action/result rhythm when the author
				// did not hand-tune the pause.
				if before == 0 && prev != nil && prev.Kind == recipe.StepAction {
					before = actionGapMs
					p.Gaps = append(p.Gaps, PauseGap{SpeechID: st.SpeechID, Position: "before", Ms: before, Reason: "action-speech-breath"})
				}
				if after == 0 && next != nil && next.Kind == recipe.StepAction {
					after = speechGapMs
					p.Gaps = append(p.Gaps, PauseGap{SpeechID: st.SpeechID, Position: "after", Ms: after, Reason: "concept-action-gap"})
				}
				if before > 1200 {
					before = 1200
				}
				if after > 1200 {
					after = 1200
				}
				if before > 0 {
					p.BeforeMs[st.SpeechID] = before
					if st.PauseBeforeMs > 0 {
						p.Gaps = append(p.Gaps, PauseGap{SpeechID: st.SpeechID, Position: "before", Ms: before, Reason: "authored"})
					}
				}
				if after > 0 {
					p.AfterMs[st.SpeechID] = after
					if st.PauseAfterMs > 0 {
						p.Gaps = append(p.Gaps, PauseGap{SpeechID: st.SpeechID, Position: "after", Ms: after, Reason: "authored"})
					}
				}
			}
		}
	}
	return p
}
