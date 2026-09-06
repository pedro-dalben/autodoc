package runreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRecipe(hash string) string {
	return `{"storyboard_hash":"` + hash + `",
"redact":{"selectors":["[data-testid=material-secret]"],"mask_password_inputs":true},
"scenes":[
 {"id":"scene-001","scene_hash":"s1","beats":[{"id":"beat-01","steps":[{"kind":"speech"},{"kind":"action"},{"kind":"wait"},{"kind":"speech"},{"kind":"hold"}]}]},
 {"id":"scene-002","scene_hash":"s2","beats":[{"id":"beat-01","steps":[{"kind":"speech"},{"kind":"action"}]}]}
],
"speech_segments":[{"id":"a"},{"id":"b"},{"id":"c"}]}`
}

func TestCollectFullRun(t *testing.T) {
	work := t.TempDir()
	run := "20260101-120000"
	writeFile(t, filepath.Join(work, run, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, run, "tts-report.json"),
		`{"segments_total":3,"cache_hits":2,"cache_misses":1,"synthesized_duration_s":4.5}`)
	writeFile(t, filepath.Join(work, run, "raw", "scene-001.webm"), "fakevideo")
	writeFile(t, filepath.Join(work, run, "raw", "scene-002.webm"), "fakevideo")
	writeFile(t, filepath.Join(work, run, "events-scene-001.jsonl"), "{\"kind\":\"action\"}\n{\"kind\":\"wait\"}\n")
	writeFile(t, filepath.Join(work, run, "final_timeline.json"),
		`{"storyboard_hash":"hash1","sync":{"items":[{"drift_ms":10,"pass":true},{"drift_ms":-200,"pass":false}],"max_drift_ms":-200,"pass":false}}`)
	writeFile(t, filepath.Join(work, run, "cinematic_report.json"),
		`{"storyboard_hash":"hash1","pass":true,"score":88}`)

	r := Collect(work, run, "0.1.0-rc.3", "demo")
	if r.Version != "v1" {
		t.Errorf("version = %q, want v1", r.Version)
	}
	if r.Pipeline.Scenes != 2 || r.Pipeline.Beats != 2 || r.Pipeline.Actions != 2 ||
		r.Pipeline.Waits != 1 || r.Pipeline.Holds != 1 || r.Pipeline.Speech != 3 {
		t.Errorf("pipeline counts wrong: %+v", r.Pipeline)
	}
	if !r.TTS.Available || r.TTS.Segments != 3 || r.TTS.Hits != 2 || r.TTS.Misses != 1 {
		t.Errorf("tts wrong: %+v", r.TTS)
	}
	if r.TTS.HitRatio < 0.666 || r.TTS.HitRatio > 0.667 {
		t.Errorf("hit ratio = %v", r.TTS.HitRatio)
	}
	if r.Capture.Recorded != 2 || r.Capture.Reused != 0 || r.Capture.Missing != 0 {
		t.Errorf("capture wrong: %+v", r.Capture)
	}
	if r.Capture.EventsTotal != 2 {
		t.Errorf("events = %d", r.Capture.EventsTotal)
	}
	if !r.Sync.Available || r.Sync.MaxDriftMs != -200 || r.Sync.OutsideThreshold != 1 || r.Sync.Pass {
		t.Errorf("sync wrong: %+v", r.Sync)
	}
	if r.Sync.MeanDriftMs < 104.9 || r.Sync.MeanDriftMs > 105.1 {
		t.Errorf("mean drift = %v", r.Sync.MeanDriftMs)
	}
	if !r.Cinematic.Available || !r.Cinematic.QAPass || r.Cinematic.QAScore != 88 {
		t.Errorf("cinematic wrong: %+v", r.Cinematic)
	}
	if r.Security.SecretScan == "" || r.Security.SecretScan == "unknown" {
		t.Errorf("secret scan should record compile-time pass: %q", r.Security.SecretScan)
	}
	if r.Security.RedactSelectors != 1 || !r.Security.MaskPassword {
		t.Errorf("security redaction wrong: %+v", r.Security)
	}
	if len(r.Recovery.FailedScenes) != 0 {
		t.Errorf("failed scenes = %v", r.Recovery.FailedScenes)
	}
}

func TestCollectPartialRun(t *testing.T) {
	work := t.TempDir()
	run := "20260101-120000"
	writeFile(t, filepath.Join(work, run, "recipe.json"), fixtureRecipe("hash1"))

	r := Collect(work, run, "0.1.0-rc.3", "demo")
	if !r.Pipeline.Available {
		t.Error("pipeline should be available")
	}
	if r.TTS.Available {
		t.Error("tts must be unavailable without tts-report.json")
	}
	if r.Sync.Available {
		t.Error("sync must be unavailable without final timeline")
	}
	if r.Cinematic.Available {
		t.Error("cinematic must be unavailable without qa report")
	}
	if r.Output.Available {
		t.Error("output must be unavailable without mp4")
	}
	// Both scenes missing video and events: failed, not crashed.
	if len(r.Recovery.FailedScenes) != 2 {
		t.Errorf("failed scenes = %v", r.Recovery.FailedScenes)
	}
	if r.Capture.Missing != 2 {
		t.Errorf("missing = %d", r.Capture.Missing)
	}
}

func TestCollectNoRecipe(t *testing.T) {
	work := t.TempDir()
	r := Collect(work, "20260101-120000", "0.1.0-rc.3", "demo")
	if r.Pipeline.Available {
		t.Error("pipeline must be unavailable without recipe")
	}
	if r.StoryboardHash != "" {
		t.Errorf("hash must be empty, got %q", r.StoryboardHash)
	}
}

func TestReuseAndRetake(t *testing.T) {
	work := t.TempDir()
	old := "20260101-110000"
	newer := "20260101-120000"
	writeFile(t, filepath.Join(work, old, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, old, "raw", "scene-001.webm"), "v")
	writeFile(t, filepath.Join(work, old, "raw", "scene-002.webm"), "v")
	writeFile(t, filepath.Join(work, newer, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, newer, "raw", "scene-002.webm"), "v2")

	r := Collect(work, newer, "0.1.0-rc.3", "demo")
	if r.Capture.Reused != 1 || r.Capture.Recorded != 1 || r.Capture.Retaken != 1 {
		t.Errorf("reuse/retake wrong: %+v", r.Capture)
	}
	if len(r.Capture.ReusedIDs) != 1 || r.Capture.ReusedIDs[0] != "scene-001" {
		t.Errorf("reused ids = %v", r.Capture.ReusedIDs)
	}
	if len(r.Recovery.RetakenScenes) != 1 || r.Recovery.RetakenScenes[0] != "scene-002" {
		t.Errorf("retaken = %v", r.Recovery.RetakenScenes)
	}
	if r.Capture.ReuseRatio != 0.5 {
		t.Errorf("reuse ratio = %v", r.Capture.ReuseRatio)
	}
}

func TestChangedSceneInvalidatesReuse(t *testing.T) {
	work := t.TempDir()
	old := "20260101-110000"
	newer := "20260101-120000"
	writeFile(t, filepath.Join(work, old, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, old, "raw", "scene-001.webm"), "v")
	changed := strings.Replace(fixtureRecipe("hash2"), `"scene_hash":"s1"`, `"scene_hash":"s1-changed"`, 1)
	writeFile(t, filepath.Join(work, newer, "recipe.json"), changed)

	r := Collect(work, newer, "0.1.0-rc.3", "demo")
	// scene-001 hash changed: old raw must NOT count as reuse.
	if r.Capture.Reused != 0 || r.Capture.Missing != 2 {
		t.Errorf("scene change must invalidate reuse: %+v", r.Capture)
	}
}

func TestUnchangedSceneReusedAcrossStoryboardChange(t *testing.T) {
	work := t.TempDir()
	old := "20260101-110000"
	newer := "20260101-120000"
	writeFile(t, filepath.Join(work, old, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, old, "raw", "scene-001.webm"), "v")
	// Only scene-002 changed (s2 -> s2-changed); scene-001 keeps hash s1.
	changed := strings.Replace(fixtureRecipe("hash2"), `"scene_hash":"s2"`, `"scene_hash":"s2-changed"`, 1)
	writeFile(t, filepath.Join(work, newer, "recipe.json"), changed)

	r := Collect(work, newer, "0.1.0-rc.3", "demo")
	// Mirrors the pipeline: unchanged scene-001 reuses across storyboard change.
	if r.Capture.Reused != 1 || len(r.Capture.ReusedIDs) != 1 || r.Capture.ReusedIDs[0] != "scene-001" {
		t.Errorf("unchanged scene must reuse: %+v", r.Capture)
	}
	if r.Capture.Missing != 1 {
		t.Errorf("changed scene-002 must miss: %+v", r.Capture)
	}
}

func TestLatestRunPrefersComplete(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "20260101-100000", "recipe.json"), fixtureRecipe("a"))
	writeFile(t, filepath.Join(work, "20260101-110000", "recipe.json"), fixtureRecipe("b"))
	writeFile(t, filepath.Join(work, "20260101-110000", "final_timeline.json"), `{}`)
	writeFile(t, filepath.Join(work, "20260101-120000", "recipe.json"), fixtureRecipe("c"))
	if got := LatestRun(work); got != "20260101-110000" {
		t.Errorf("latest complete = %q, want 20260101-110000", got)
	}
	writeFile(t, filepath.Join(work, "20260101-120000", "final_timeline.json"), `{}`)
	writeFile(t, filepath.Join(work, "20260101-120000", "tutorial.mp4"), `fake`)
	if got := LatestRun(work); got != "20260101-120000" {
		t.Errorf("latest full = %q, want 20260101-120000", got)
	}
}

func TestTTSFallbackToSameHashSibling(t *testing.T) {
	work := t.TempDir()
	ttsRun := "20260101-110000"
	renderRun := "20260101-120000"
	writeFile(t, filepath.Join(work, ttsRun, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, ttsRun, "tts-report.json"),
		`{"segments_total":3,"cache_hits":3,"cache_misses":0,"synthesized_duration_s":0.5}`)
	writeFile(t, filepath.Join(work, renderRun, "recipe.json"), fixtureRecipe("hash1"))
	r := Collect(work, renderRun, "v", "demo")
	if !r.TTS.Available || r.TTS.Hits != 3 {
		t.Errorf("tts fallback failed: %+v", r.TTS)
	}
}

func TestTTSFallbackSkipsEmptySibling(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "20260101-110000", "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, "20260101-110000", "tts-report.json"),
		`{"segments_total":2,"cache_hits":1,"cache_misses":1,"synthesized_duration_s":0.5}`)
	// Newer same-hash run minted by a read-only command: recipe only.
	writeFile(t, filepath.Join(work, "20260101-120000", "recipe.json"), fixtureRecipe("hash1"))
	r := Collect(work, "20260101-120000", "v", "demo")
	if !r.TTS.Available || r.TTS.Segments != 2 || r.TTS.Hits != 1 {
		t.Errorf("fallback must skip empty sibling: %+v", r.TTS)
	}
}
func TestDeterminism(t *testing.T) {
	work := t.TempDir()
	run := "20260101-120000"
	writeFile(t, filepath.Join(work, run, "recipe.json"), fixtureRecipe("hash1"))
	writeFile(t, filepath.Join(work, run, "tts-report.json"),
		`{"segments_total":3,"cache_hits":3,"cache_misses":0,"synthesized_duration_s":1.0}`)
	writeFile(t, filepath.Join(work, run, "raw", "scene-002.webm"), "v")

	a := Collect(work, run, "0.1.0-rc.3", "demo")
	b := Collect(work, run, "0.1.0-rc.3", "demo")
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("collect is not deterministic")
	}
	var parsed map[string]any
	if err := json.Unmarshal(ja, &parsed); err != nil {
		t.Fatalf("evidence.json must parse: %v", err)
	}
	if parsed["version"] != "v1" {
		t.Errorf("schema version missing: %v", parsed["version"])
	}
}

func TestPrivacy(t *testing.T) {
	work := t.TempDir()
	run := "20260101-120000"
	secret := "s3cr3t-p4ssw0rd-sentinel"
	speech := "narrate the token abc123 in detail"
	recipe := `{"storyboard_hash":"h",
"scenes":[{"id":"s1","beats":[]}],
"speech_segments":[{"id":"x","text":"` + speech + `"}]}`
	writeFile(t, filepath.Join(work, run, "recipe.json"), recipe)

	r := Collect(work, run, "0.1.0-rc.3", "demo")
	data, _ := json.Marshal(r)
	s := string(data)
	for _, forbidden := range []string{secret, speech, "abc123"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("report leaks %q", forbidden)
		}
	}
	if strings.Contains(s, `"text"`) {
		t.Error("report must not carry speech text fields")
	}
	text := r.RenderText()
	for _, forbidden := range []string{speech, "abc123"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("text rendering leaks %q", forbidden)
		}
	}
}

func TestLatestRun(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "20260101-100000", "recipe.json"), fixtureRecipe("a"))
	writeFile(t, filepath.Join(work, "20260101-120000", "recipe.json"), fixtureRecipe("b"))
	if err := os.MkdirAll(filepath.Join(work, "20260101-130000"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := LatestRun(work); got != "20260101-120000" {
		t.Errorf("latest = %q", got)
	}
	if got := LatestRun(t.TempDir()); got != "" {
		t.Errorf("empty workdir latest = %q", got)
	}
}

func TestCompare(t *testing.T) {
	a := &Report{TTS: TTS{Available: true, HitRatio: 0.45}, Capture: Capture{Available: true, ReuseRatio: 0.2, Retaken: 3},
		Sync: Sync{Available: true, MaxDriftMs: 181}, Pipeline: Pipeline{Available: true, Scenes: 4, Speech: 10}}
	b := &Report{TTS: TTS{Available: true, HitRatio: 0.82}, Capture: Capture{Available: true, ReuseRatio: 0.71, Retaken: 1},
		Sync: Sync{Available: true, MaxDriftMs: 92}, Pipeline: Pipeline{Available: true, Scenes: 4, Speech: 10}}
	rows := Compare(a, b)
	byMetric := map[string]Row{}
	for _, r := range rows {
		byMetric[r.Metric] = r
	}
	if byMetric["TTS hit ratio"].Delta != "+37.0pp" {
		t.Errorf("tts delta = %q", byMetric["TTS hit ratio"].Delta)
	}
	if byMetric["capture reuse"].Delta != "+51.0pp" {
		t.Errorf("reuse delta = %q", byMetric["capture reuse"].Delta)
	}
	if byMetric["retakes"].Delta != "-2" {
		t.Errorf("retake delta = %q", byMetric["retakes"].Delta)
	}
	if byMetric["max drift"].Delta != "-89ms" {
		t.Errorf("drift delta = %q", byMetric["max drift"].Delta)
	}
	out := RenderCompare(rows)
	if !strings.Contains(out, "TTS hit ratio") || !strings.Contains(out, "Before") {
		t.Errorf("compare table malformed:\n%s", out)
	}
}

func TestCompareUnavailable(t *testing.T) {
	a := &Report{}
	b := &Report{}
	for _, r := range Compare(a, b) {
		if r.Before != "n/a" || r.After != "n/a" || r.Delta != "—" {
			t.Errorf("unavailable row must be n/a: %+v", r)
		}
	}
}

func TestRenderTextPartial(t *testing.T) {
	r := &Report{Version: "v1", RunID: "x", AutodocVersion: "v", Tutorial: "t",
		Recovery: Recovery{Available: true}, Security: Security{Available: true, SecretScan: "unknown"}}
	out := r.RenderText()
	for _, want := range []string{"no tts report yet", "no final timeline yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("partial text missing %q:\n%s", want, out)
		}
	}
}
