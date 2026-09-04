package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/pipeline"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/tts"
)

func TestCacheKeyScopesProviderAndFormat(t *testing.T) {
	a := tts.CacheKey("abcdef1234567890", "openai-compatible", "wav")
	b := tts.CacheKey("abcdef1234567890", "other-provider", "wav")
	c := tts.CacheKey("abcdef1234567890", "openai-compatible", "mp3")
	if a == b || a == c {
		t.Fatal("provider/format change must change TTS cache key")
	}
	if d := tts.CacheKey("abcdef1234567890", "openai-compatible", "wav"); d != a {
		t.Fatal("identical backend must key identically")
	}
}

func TestLoadTimelinePrefersMostRecentMatch(t *testing.T) {
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
	mkRun := func(id string, total float64) {
		run, err := pipeline.NewRun(root, sbPath, config.Default())
		if err != nil {
			t.Fatal(err)
		}
		run.RunID = id
		if err := run.Compile(); err != nil {
			t.Fatal(err)
		}
		tl, err := timeline.Build(run.Recipe, func(sid string) (float64, bool, string) {
			return total, false, "/tmp/x.wav"
		}, 600)
		if err != nil {
			t.Fatal(err)
		}
		if err := tl.WriteJSON(filepath.Join(root, ".autodoc", "_work", id, "timeline.json")); err != nil {
			t.Fatal(err)
		}
	}
	// Older run is slower; a max-duration pick would choose it. Recency wins.
	mkRun("20200101-000001", 30.0)
	mkRun("20200101-000002", 5.0)
	run, err := pipeline.NewRun(root, sbPath, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	run.RunID = "20200101-000003"
	if err := run.Compile(); err != nil {
		t.Fatal(err)
	}
	if err := run.LoadTimeline(); err != nil {
		t.Fatal(err)
	}
	if run.Timeline.TotalS != 5.0 {
		t.Fatalf("LoadTimeline = %.2fs, want most recent (5.0s), not slowest", run.Timeline.TotalS)
	}
}
