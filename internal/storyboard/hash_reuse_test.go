package storyboard_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func hashSample() *storyboard.Storyboard {
	return &storyboard.Storyboard{
		Version: 2,
		Meta:    storyboard.Meta{Title: "T", Language: "pt-BR"},
		Config:  storyboard.Config{BaseURL: "http://x"},
		Scenes: []storyboard.Scene{
			{ID: "s1", Title: "Send", URL: "/chat", Beats: []storyboard.Beat{
				{ID: "b1", Sequence: []storyboard.Event{
					{Speech: &storyboard.SpeechEvent{Text: "Envie a mensagem."}},
					{Action: &storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "send"}}},
				}},
			}},
		},
	}
}

func TestSourceHashSensitiveToRedactTitleScreenshot(t *testing.T) {
	base := hashSample().SourceHash()
	redacted := hashSample()
	redacted.Redact = storyboard.Redact{Selectors: []string{"#token"}, MaskPasswordInputs: true}
	if got := redacted.SourceHash(); got == base {
		t.Fatal("redact change must invalidate SourceHash (capture-time masking)")
	}
	retitled := hashSample()
	retitled.Scenes[0].Title = "Renamed"
	if got := retitled.SourceHash(); got == base {
		t.Fatal("scene title change must invalidate SourceHash")
	}
	shot := hashSample()
	shot.Scenes[0].Screenshot = true
	if got := shot.SourceHash(); got == base {
		t.Fatal("screenshot flag change must invalidate SourceHash")
	}
	if got := hashSample().SourceHash(); got != base {
		t.Fatal("identical storyboard must hash identically")
	}
}

func TestSceneHashSensitiveToTitleAndExecKey(t *testing.T) {
	base := hashSample().Scenes[0].SceneHash()
	retitled := hashSample()
	retitled.Scenes[0].Title = "Renamed"
	if got := retitled.Scenes[0].SceneHash(); got == base {
		t.Fatal("scene title change must invalidate SceneHash")
	}
	if got := hashSample().Scenes[0].SceneHash("tts|m|v|pt-BR|1.000|redact|false|"); got == base {
		t.Fatal("exec key (TTS identity) must participate in SceneHash")
	}
	other := hashSample().Scenes[0].SceneHash("tts|m|other|pt-BR|1.000|redact|false|")
	if other == hashSample().Scenes[0].SceneHash("tts|m|v|pt-BR|1.000|redact|false|") {
		t.Fatal("voice change must invalidate SceneHash (narration clock)")
	}
}
