package cinematic

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// TestBackwardAnchorTransitionalNarration proves dogfood finding F1b:
// narration that follows the state it describes (action, then targeted
// wait, then speech, then hold) anchors on the awaited UI instead of
// drifting to AnchorNone and failing the visual-anchors gate.
func TestBackwardAnchorTransitionalNarration(t *testing.T) {
	r := &recipe.Recipe{
		StoryboardHash: "test",
		Scenes: []recipe.ScenePlan{{
			ID: "s1",
			Beats: []recipe.BeatPlan{{
				ID: "b1",
				Steps: []recipe.StepPlan{
					{Kind: recipe.StepSpeech, Index: 0, SpeechID: "s1-b1-speech-001", Text: "Click enter."},
					{Kind: recipe.StepAction, Index: 1, Action: &storyboard.Action{
						Type:   "click",
						Target: &storyboard.Target{TestID: "login-btn"},
					}},
					{Kind: recipe.StepWait, Index: 2, Wait: &storyboard.WaitEvent{
						State:  "visible",
						Target: &storyboard.Target{TestID: "new-material-btn"},
					}},
					{Kind: recipe.StepSpeech, Index: 3, SpeechID: "s1-b1-speech-002", Text: "Here are the materials."},
					{Kind: recipe.StepHold, Index: 4, HoldMs: 600},
				},
			}},
		}},
	}
	doc := PlanScenes(r, &storyboard.Storyboard{})
	var trailing *BeatPlan
	for i := range doc.Beats {
		b := &doc.Beats[i]
		if b.SpeechID == "s1-b1-speech-002" {
			trailing = b
		}
	}
	if trailing == nil {
		t.Fatal("trailing speech beat not planned")
	}
	if trailing.Anchor.Kind == AnchorNone {
		t.Errorf("transitional narration must anchor backward, got none")
	}
	if trailing.Anchor.Kind != AnchorContainer {
		t.Errorf("backward wait anchor kind = %q, want %q", trailing.Anchor.Kind, AnchorContainer)
	}
}

// TestBackwardAnchorKeepsForwardPriority proves the fallback never
// overrides the forward action neighbor.
func TestBackwardAnchorKeepsForwardPriority(t *testing.T) {
	r := &recipe.Recipe{
		StoryboardHash: "test",
		Scenes: []recipe.ScenePlan{{
			ID: "s1",
			Beats: []recipe.BeatPlan{{
				ID: "b1",
				Steps: []recipe.StepPlan{
					{Kind: recipe.StepWait, Index: 0, Wait: &storyboard.WaitEvent{
						State:  "visible",
						Target: &storyboard.Target{TestID: "old-btn"},
					}},
					{Kind: recipe.StepSpeech, Index: 1, SpeechID: "s1-b1-speech-001", Text: "Fill the name."},
					{Kind: recipe.StepAction, Index: 2, Action: &storyboard.Action{
						Type:   "fill",
						Target: &storyboard.Target{TestID: "material-name"},
					}},
				},
			}},
		}},
	}
	doc := PlanScenes(r, &storyboard.Storyboard{})
	for i := range doc.Beats {
		b := &doc.Beats[i]
		if b.SpeechID == "s1-b1-speech-001" {
			if b.Anchor.Kind == AnchorNone {
				t.Fatal("speech between wait and action must anchor forward")
			}
			if b.Anchor.Kind != AnchorForm {
				t.Errorf("forward fill anchor kind = %q, want form", b.Anchor.Kind)
			}
		}
	}
}

// TestQAIgnoresAttentionBudget proves dogfood finding F1a: beats the
// director deliberately throttled (attention-budget) must not fail the
// action-anticipation gate, exactly like explicit "none" overrides.
func TestQAIgnoresAttentionBudget(t *testing.T) {
	scenes := &ScenePlanDoc{Beats: []BeatPlan{
		{Index: 0, Type: SceneSubmit, NeedsAnticipation: true},
		{Index: 1, Type: SceneSubmit, NeedsAnticipation: true},
	}}
	att := &AttentionPlan{Decisions: []AttentionDecision{
		{BeatIndex: 0, Anticipate: true},
		{BeatIndex: 1, Anticipate: false, Reason: "attention-budget"},
	}}
	ft := &timeline.FinalTimeline{Sync: &timeline.SyncReport{Pass: true}}
	rep := RunQA(ft, scenes, att, &CameraPlan{}, &EditPlan{}, nil, nil, visual.DefaultCinematic())
	found := false
	for _, g := range rep.Gates {
		if g.Name == "action-anticipation" {
			found = true
			if !g.Pass {
				t.Errorf("budget-throttled beat must not fail gate: %s", g.Detail)
			}
		}
	}
	if !found {
		t.Error("action-anticipation gate missing")
	}
}

// TestQAStillFailsGenuinelyMissingAnticipation guards the fix: a beat
// the director dropped without reason still fails the gate.
func TestQAStillFailsGenuinelyMissingAnticipation(t *testing.T) {
	scenes := &ScenePlanDoc{Beats: []BeatPlan{
		{Index: 0, Type: SceneSubmit, NeedsAnticipation: true},
	}}
	att := &AttentionPlan{Decisions: []AttentionDecision{
		{BeatIndex: 0, Anticipate: false, Reason: "interaction-anchor"},
	}}
	ft := &timeline.FinalTimeline{Sync: &timeline.SyncReport{Pass: true}}
	rep := RunQA(ft, scenes, att, &CameraPlan{}, &EditPlan{}, nil, nil, visual.DefaultCinematic())
	for _, g := range rep.Gates {
		if g.Name == "action-anticipation" && g.Pass {
			t.Error("unexplained missing anticipation must still fail the gate")
		}
	}
}

// TestOpeningNarrationAnchorsViewport proves the base-case tutorial
// (scene opens with context-free narration) anchors the establishing
// viewport instead of failing visual-anchors/static-narration.
func TestOpeningNarrationAnchorsViewport(t *testing.T) {
	r := &recipe.Recipe{
		StoryboardHash: "test",
		Scenes: []recipe.ScenePlan{{
			ID: "s1",
			Beats: []recipe.BeatPlan{{
				ID: "b1",
				Steps: []recipe.StepPlan{
					{Kind: recipe.StepSpeech, Index: 0, SpeechID: "s1-b1-speech-001", Text: "This is the home panel."},
					{Kind: recipe.StepHold, Index: 1, HoldMs: 800},
				},
			}},
		}},
	}
	doc := PlanScenes(r, &storyboard.Storyboard{})
	if len(doc.Beats) == 0 {
		t.Fatal("no beats planned")
	}
	if doc.Beats[0].Anchor.Kind != AnchorViewport {
		t.Errorf("opening narration anchor = %q, want viewport", doc.Beats[0].Anchor.Kind)
	}
}
