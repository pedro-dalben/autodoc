package pipeline_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/pipeline"
)

const lookupSB = `version: 1
meta:
  title: "T"
  language: "pt-BR"
config:
  base_url: "http://localhost:8099"
setup:
  start_url: "/"
scenes:
  - id: scene-001
    title: S1
    beats: [{id: b1, sequence: [{speech: {text: "Oi"}}]}]
  - id: scene-002
    title: S2
    beats: [{id: b1, sequence: [{speech: {text: "Tchau"}}]}]
`

func writeLookupSB(t *testing.T, root string) string {
	t.Helper()
	p := filepath.Join(root, "storyboard.yml")
	if err := os.WriteFile(p, []byte(lookupSB), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func compileRunWithID(t *testing.T, root, sbPath, runID string) *pipeline.Run {
	t.Helper()
	run, err := pipeline.NewRun(root, sbPath, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	run.RunID = runID
	if err := run.Compile(); err != nil {
		t.Fatal(err)
	}
	return run
}

func putRaw(t *testing.T, run *pipeline.Run, sceneID string) string {
	t.Helper()
	p := filepath.Join(run.WorkDir, run.RunID, "raw", sceneID+".webm")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("fake-raw"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Reproduces the black-MP4 bug: record ran under RunID A, render runs under a
// fresh RunID B with the same storyboard hash, and an older tts-only run dir
// (same hash, no raw/) sorts before the dir holding the raw capture.
// Render must still find A's raw instead of falling back to solid color.
func TestRenderFindsRawAcrossRunIDs(t *testing.T) {
	root := t.TempDir()
	sbPath := writeLookupSB(t, root)

	ttsOnly := compileRunWithID(t, root, sbPath, "20200101-000001")
	_ = ttsOnly // same hash, no raw/ — the old first-glob-match trap
	recorded := compileRunWithID(t, root, sbPath, "20200101-000002")
	wantRaw := putRaw(t, recorded, "scene-001")
	renderRun := compileRunWithID(t, root, sbPath, "20200101-000003")

	if got := renderRun.LatestRunDirWithHash(); got != filepath.Join(renderRun.WorkDir, "20200101-000002") {
		t.Fatalf("LatestRunDirWithHash = %q, want most recent same-hash dir", got)
	}
	if got := renderRun.LatestRunDirWithSceneVideo("scene-001"); got != filepath.Join(renderRun.WorkDir, "20200101-000002") {
		t.Fatalf("LatestRunDirWithSceneVideo = %q, want dir holding raw", got)
	}
	gotRaw, srcDir := renderRun.FindSceneVideo("scene-001")
	if gotRaw != wantRaw {
		t.Fatalf("FindSceneVideo = %q, want %q", gotRaw, wantRaw)
	}
	if srcDir != filepath.Join(renderRun.WorkDir, "20200101-000002") {
		t.Fatalf("FindSceneVideo srcDir = %q", srcDir)
	}
}

// Per-scene fallback: each scene may come from a different recorded run
// (e.g. after --retake), and missing scenes must report "" instead of a
// wrong dir.
func TestFindSceneVideoPerScene(t *testing.T) {
	root := t.TempDir()
	sbPath := writeLookupSB(t, root)

	runA := compileRunWithID(t, root, sbPath, "20200101-000001")
	rawA := putRaw(t, runA, "scene-001")
	runB := compileRunWithID(t, root, sbPath, "20200101-000002")
	rawB := putRaw(t, runB, "scene-002")
	renderRun := compileRunWithID(t, root, sbPath, "20200101-000003")

	if p, _ := renderRun.FindSceneVideo("scene-001"); p != rawA {
		t.Fatalf("scene-001 = %q, want %q", p, rawA)
	}
	if p, _ := renderRun.FindSceneVideo("scene-002"); p != rawB {
		t.Fatalf("scene-002 = %q, want %q", p, rawB)
	}
	if p, d := renderRun.FindSceneVideo("scene-999"); p != "" || d != "" {
		t.Fatalf("unknown scene = %q %q, want empty", p, d)
	}
	if got := renderRun.LatestRunDirWithSceneVideo("scene-999"); got != "" {
		t.Fatalf("LatestRunDirWithSceneVideo(unknown) = %q, want empty", got)
	}
}

// A newer same-hash run without raw must not shadow an older run with raw,
// and the current run dir itself must be skipped.
func TestLatestRunDirSkipsCurrentAndRawless(t *testing.T) {
	root := t.TempDir()
	sbPath := writeLookupSB(t, root)

	recorded := compileRunWithID(t, root, sbPath, "20200101-000001")
	putRaw(t, recorded, "scene-001")
	compileRunWithID(t, root, sbPath, "20200101-000002") // newer, tts-only
	renderRun := compileRunWithID(t, root, sbPath, "20200101-000003")

	if got := renderRun.LatestRunDirWithSceneVideo("scene-001"); got != filepath.Join(renderRun.WorkDir, "20200101-000001") {
		t.Fatalf("LatestRunDirWithSceneVideo = %q, want oldest dir holding raw", got)
	}
	// LatestRunDirWithHash returns the most recent same-hash dir, not the
	// current run and not the first alphabetical match (000001 < 000002).
	if got := renderRun.LatestRunDirWithHash(); got != filepath.Join(renderRun.WorkDir, "20200101-000002") {
		t.Fatalf("LatestRunDirWithHash = %q, want 20200101-000002", got)
	}
}
