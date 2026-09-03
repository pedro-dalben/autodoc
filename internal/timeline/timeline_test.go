package timeline_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
)

func TestExplicitSequenceTimeline(t *testing.T) {
	r := &recipe.Recipe{
		StoryboardHash: "abc",
		Scenes: []recipe.ScenePlan{
			{ID: "scene-001", Beats: []recipe.BeatPlan{
				{ID: "beat-01", Steps: []recipe.StepPlan{
					{Kind: recipe.StepSpeech, SpeechID: "s1", Text: "A"},
					{Kind: recipe.StepAction, Action: &storyboard.Action{Type: "click"}},
					{Kind: recipe.StepWait, Wait: &storyboard.WaitEvent{State: "visible", SettleMs: 600}},
					{Kind: recipe.StepSpeech, SpeechID: "s2", Text: "B"},
					{Kind: recipe.StepHold, HoldMs: 600},
				}},
			}},
		},
	}
	durs := map[string]float64{"s1": 2.0, "s2": 4.0}
	tl, err := timeline.Build(r, func(id string) (float64, bool, string) {
		return durs[id], false, "/tmp/" + id + ".wav"
	}, 600)
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Segments) != 2 {
		t.Fatalf("expected 2 segments: %+v", tl.Segments)
	}
	s1, s2 := tl.Segments[0], tl.Segments[1]
	if s1.StartS != 0 || s1.EndS != 2.0 {
		t.Fatalf("bad s1: %+v", s1)
	}
	if s2.StartS < 2.0 {
		t.Fatalf("s2 must start after s1+settle: %+v", s2)
	}
	if s2.DurationS != 4.0 {
		t.Fatalf("s2 must keep full 4s: %+v", s2)
	}
	expected := 2.0 + 0.6 + 4.0 + 0.6
	if diff := tl.TotalS - expected; diff > 0.001 || diff < -0.001 {
		t.Fatalf("total %.3f != %.3f", tl.TotalS, expected)
	}
	actionAt := -1.0
	for _, m := range tl.Markers {
		if m.Kind == "action" {
			actionAt = m.AtS
		}
	}
	if actionAt != 2.0 {
		t.Fatalf("action must be at 2.0 (right after s1), got %v", actionAt)
	}
}
