package recipe_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func voiceSample() *storyboard.Storyboard {
	return &storyboard.Storyboard{
		Version: 2,
		Meta:    storyboard.Meta{Title: "T", Language: "pt-BR"},
		Config:  storyboard.Config{BaseURL: "http://x"},
		Scenes: []storyboard.Scene{
			{ID: "s1", Title: "S", Beats: []storyboard.Beat{
				{ID: "b1", Sequence: []storyboard.Event{
					{Speech: &storyboard.SpeechEvent{Text: "Global voice."}},
					{Speech: &storyboard.SpeechEvent{Text: "Override voice.", Voice: "custom"}},
				}},
			}},
		},
	}
}

func TestCompileResolvesPerBeatVoice(t *testing.T) {
	r := recipe.Compile(voiceSample(), "m", "global", "pt-BR", 1.0)
	if len(r.SpeechSegments) != 2 {
		t.Fatalf("want 2 segments, got %d", len(r.SpeechSegments))
	}
	if r.SpeechSegments[0].Voice != "global" {
		t.Fatalf("segment 0 voice = %q, want global", r.SpeechSegments[0].Voice)
	}
	if r.SpeechSegments[1].Voice != "custom" {
		t.Fatalf("segment 1 voice = %q, want custom", r.SpeechSegments[1].Voice)
	}
	if r.SpeechSegments[0].Hash == r.SpeechSegments[1].Hash {
		t.Fatal("distinct voice/text must hash distinctly")
	}
}

func TestExecKeyBindsTTSIdentity(t *testing.T) {
	sb := voiceSample()
	a := recipe.ExecKey(sb, "m", "v1", "pt-BR", 1.0)
	b := recipe.ExecKey(sb, "m", "v2", "pt-BR", 1.0)
	if a == b {
		t.Fatal("voice change must change exec key")
	}
	ra := recipe.Compile(sb, "m", "v1", "pt-BR", 1.0)
	rb := recipe.Compile(sb, "m", "v2", "pt-BR", 1.0)
	if ra.StoryboardHash == rb.StoryboardHash {
		t.Fatal("voice change must invalidate run-level reuse key")
	}
	if ra.Scenes[0].SceneHash == rb.Scenes[0].SceneHash {
		t.Fatal("voice change must invalidate per-scene reuse key (narration clock)")
	}
	rc := recipe.Compile(sb, "m", "v1", "pt-BR", 1.0)
	if ra.StoryboardHash != rc.StoryboardHash || ra.Scenes[0].SceneHash != rc.Scenes[0].SceneHash {
		t.Fatal("identical TTS identity must hash identically")
	}
}
