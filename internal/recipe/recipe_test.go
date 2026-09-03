package recipe_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func sampleSB() *storyboard.Storyboard {
	return &storyboard.Storyboard{
		Version: 1,
		Meta:    storyboard.Meta{Title: "T", Language: "pt-BR"},
		Config:  storyboard.Config{BaseURL: "http://x"},
		Scenes: []storyboard.Scene{
			{ID: "scene-001", Title: "S", URL: "/a", Beats: []storyboard.Beat{
				{ID: "beat-01", Sequence: []storyboard.Event{
					{Speech: &storyboard.SpeechEvent{Text: "Olá mundo"}},
					{Action: &storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "btn"}}},
					{Speech: &storyboard.SpeechEvent{Text: "Feito"}},
					{Hold: &storyboard.HoldEvent{DurationMs: 500}},
				}},
			}},
		},
	}
}

func TestCompilePreservesOrder(t *testing.T) {
	r := recipe.Compile(sampleSB(), "kokoro", "v", "pt-BR", 1.0)
	if len(r.Scenes) != 1 || len(r.Scenes[0].Beats) != 1 {
		t.Fatalf("bad scenes: %+v", r.Scenes)
	}
	steps := r.Scenes[0].Beats[0].Steps
	kinds := []string{string(steps[0].Kind), string(steps[1].Kind), string(steps[2].Kind), string(steps[3].Kind)}
	want := []string{"speech", "action", "speech", "hold"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("order broken: %v", kinds)
		}
	}
	if len(r.SpeechSegments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(r.SpeechSegments))
	}
	if r.SpeechSegments[0].ID == r.SpeechSegments[1].ID {
		t.Fatal("segment ids must differ")
	}
}

func TestSpeechHashStable(t *testing.T) {
	a := recipe.SpeechHash("hello", "v", "m", "pt-BR", 1.0)
	b := recipe.SpeechHash("hello", "v", "m", "pt-BR", 1.0)
	c := recipe.SpeechHash("hello!", "v", "m", "pt-BR", 1.0)
	d := recipe.SpeechHash("hello", "v2", "m", "pt-BR", 1.0)
	if a != b || a == c || a == d {
		t.Fatalf("hash unstable: %s %s %s %s", a, b, c, d)
	}
}
