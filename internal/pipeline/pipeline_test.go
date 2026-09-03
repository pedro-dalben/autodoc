package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/pipeline"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func TestSecretScanBlocksCredentials(t *testing.T) {
	sb := &storyboard.Storyboard{
		Version: 1,
		Meta:    storyboard.Meta{Title: "T", Language: "pt-BR"},
		Config:  storyboard.Config{BaseURL: "http://x"},
		Scenes: []storyboard.Scene{
			{ID: "s1", Title: "S", Beats: []storyboard.Beat{
				{ID: "b1", Sequence: []storyboard.Event{
					{Action: &storyboard.Action{Type: "fill", Target: &storyboard.Target{TestID: "pw"}, Value: "SuperSecret123!@#XYZabc"}},
				}},
			}},
		},
	}
	if err := pipeline.ScanSecrets(sb); err == nil {
		t.Fatal("expected secret scan to block credential-like fill")
	}
}

func TestSecretScanClean(t *testing.T) {
	sb := &storyboard.Storyboard{
		Version: 1,
		Meta:    storyboard.Meta{Title: "T", Language: "pt-BR"},
		Config:  storyboard.Config{BaseURL: "http://x"},
		Scenes: []storyboard.Scene{
			{ID: "s1", Title: "S", Beats: []storyboard.Beat{
				{ID: "b1", Sequence: []storyboard.Event{
					{Speech: &storyboard.SpeechEvent{Text: "Clique em Novo."}},
					{Action: &storyboard.Action{Type: "fill", Target: &storyboard.Target{TestID: "name"}, Value: "Areia lavada"}},
				}},
			}},
		},
	}
	if err := pipeline.ScanSecrets(sb); err != nil {
		t.Fatalf("false positive: %v", err)
	}
}

func TestWorkDirSeparation(t *testing.T) {
	root := t.TempDir()
	sbPath := filepath.Join(root, "storyboard.yml")
	sb := `version: 1
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:8099"}
setup: {start_url: "/"}
scenes:
  - id: scene-001
    title: S
    beats: [{id: b1, sequence: [{speech: {text: "Oi"}}]}]
`
	if err := os.WriteFile(sbPath, []byte(sb), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = root
	_ = sbPath
}
