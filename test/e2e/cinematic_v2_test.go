package e2e

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCinematicV2Director exercises the full AI Director pipeline on a
// real browser: semantic planning, anticipation, spotlight, camera
// continuity, typing, wait compression, result confirmation, context
// restore, sync and cinematic QA — with pixel-level frame evidence.
func TestCinematicV2Director(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e needs browsers+ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg missing")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe missing")
	}
	root := repoRoot(t)
	binDir := t.TempDir()
	autodoc, fixtureBin, fakettsBin := buildBins(t, binDir)
	work := t.TempDir()
	startBin(t, fakettsBin)
	waitHTTP(t, "http://localhost:8880/healthz", 20*time.Second)
	startBin(t, fixtureBin)
	waitHTTP(t, "http://localhost:8099/healthz", 20*time.Second)
	waitHTTP(t, "http://localhost:8099/cinematic", 20*time.Second)

	sb, err := os.ReadFile(filepath.Join(root, "test", "fixture", "cinematic-v2.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "storyboard.yml"), sb, 0o644); err != nil {
		t.Fatal(err)
	}
	toml, err := os.ReadFile(filepath.Join(root, "test", "fixture", "autodoc.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "autodoc.toml"), toml, 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, work, autodoc, "storyboard", "validate", "--storyboard", "storyboard.yml")
	run(t, work, autodoc, "tts", "--storyboard", "storyboard.yml")
	out := run(t, work, autodoc, "record", "--storyboard", "storyboard.yml", "--headless")
	if !strings.Contains(out, "scene-talk") {
		t.Fatalf("record must cover scene-talk: %s", out)
	}

	// 1. Direction evidence in capture events.
	eventsRaw, err := os.ReadFile(globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "events-scene-talk.jsonl")))
	if err != nil {
		t.Fatal(err)
	}
	events := string(eventsRaw)
	for _, want := range []string{`anticipated`, `spotlight`, `ripple`, `"kind":"result"`, `callout`, `"kind":"speech_start"`, `"kind":"speech_end"`, `typing_chars`, `progressive`} {
		if !strings.Contains(events, want) {
			t.Fatalf("events-scene-talk.jsonl missing %s", want)
		}
	}

	// 2. Director workspace artifacts exist (debug, never published).
	runDir := filepath.Dir(globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "final_timeline.json")))
	for _, f := range []string{"scene_plan.json", "cinematic_plan.json", "edit_plan.json", "cinematic_report.json", "final_timeline.json"} {
		st, err := os.Stat(filepath.Join(runDir, f))
		if err != nil || st.Size() == 0 {
			t.Fatalf("workspace artifact missing: %s", f)
		}
	}

	// 3. Final timeline + sync PASS, no estimates.
	finalRaw, err := os.ReadFile(filepath.Join(runDir, "final_timeline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ft finalTL
	if err := json.Unmarshal(finalRaw, &ft); err != nil {
		t.Fatal(err)
	}
	if ft.Sync == nil || !ft.Sync.Pass {
		t.Fatalf("sync must PASS: %+v", ft.Sync)
	}
	for _, sg := range ft.Segments {
		if sg.Estimated {
			t.Fatalf("no segment may be estimated after fresh record: %+v", sg)
		}
	}

	// 4. Cinematic QA gates PASS.
	out = run(t, work, autodoc, "validate", "--storyboard", "storyboard.yml", "--cinematic")
	if !strings.Contains(out, "AUTODOC_CINEMATIC_QA: PASS") {
		t.Fatalf("cinematic QA must PASS:\n%s", out)
	}
	for _, gate := range []string{"visual-anchors", "action-anticipation", "result-confirmation", "camera-continuity", "context-restoration", "overlay-collisions"} {
		if !strings.Contains(out, gate) {
			t.Fatalf("QA report missing gate %s:\n%s", gate, out)
		}
	}

	// 5. Camera continuity: at least one focused move, zero applied
	// oscillation beyond the plan, final restore to full context.
	var clickConv *finalSeg
	var zooms int
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind == "action" && strings.Contains(sg.Label, "conv-alice") {
			clickConv = sg
		}
		if sg.Zoom > 1.01 {
			zooms++
		}
	}
	if clickConv == nil {
		t.Fatal("click conv-alice segment missing")
	}
	if clickConv.Zoom <= 1.01 || clickConv.NormBBox == nil {
		t.Fatalf("conversation click must carry camera zoom + bbox: %+v", clickConv)
	}
	if zooms == 0 {
		t.Fatal("no camera moves directed")
	}
	last := ft.Segments[len(ft.Segments)-1]
	if last.Zoom > 1.01 {
		t.Fatalf("final beat must restore full context (zoom=1), got %.2f", last.Zoom)
	}

	// 6. Result confirmation: send action holds success state.
	var sendSeg *finalSeg
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind == "action" && strings.Contains(sg.Label, "cinematic-send") {
			sendSeg = sg
		}
	}
	if sendSeg == nil {
		t.Fatal("send action segment missing")
	}
	if sendSeg.DurS < 0.8 {
		t.Fatalf("result must hold >= 800ms, got %.2fs", sendSeg.DurS)
	}

	// 7. Dead-time editing: the delayed receipt compresses honestly.
	compressed := false
	for _, sg := range ft.Segments {
		if sg.Kind == "wait" && strings.Contains(sg.Label, "visible") {
			t.Logf("wait seg: label=%s dur=%.2f", sg.Label, sg.DurS)
		}
	}
	editRaw, _ := os.ReadFile(filepath.Join(runDir, "edit_plan.json"))
	var edit struct {
		WaitsSavedMs int `json:"waits_saved_ms"`
		Clips        []struct {
			Type     string  `json:"type"`
			Strategy string  `json:"strategy"`
			Label    string  `json:"label"`
			Speed    float64 `json:"speed"`
		} `json:"clips"`
	}
	_ = json.Unmarshal(editRaw, &edit)
	for _, c := range edit.Clips {
		if c.Type == "transition" && (c.Strategy == "compress" || c.Strategy == "speed_ramp") {
			compressed = true
		}
	}
	if !compressed {
		t.Fatalf("artificial loading wait must compress; edit plan: %+v", edit.Clips)
	}
	if edit.WaitsSavedMs <= 0 {
		t.Fatalf("expected saved wait ms > 0, got %d", edit.WaitsSavedMs)
	}
	clipTypes := map[string]bool{}
	for _, c := range edit.Clips {
		clipTypes[c.Type] = true
	}
	for _, want := range []string{"establish", "anticipation", "action", "transition", "result", "narration"} {
		if !clipTypes[want] {
			t.Fatalf("edit plan missing clip type %s", want)
		}
	}

	// 8. Render + duration coherence + QA line.
	out = run(t, work, autodoc, "render", "--storyboard", "storyboard.yml", "--debug-cues", "--debug-timeline")
	if !strings.Contains(out, "AutoDoc Sync") || !strings.Contains(out, "AUTODOC_CINEMATIC_QA:") {
		t.Fatalf("render must print sync + cinematic QA: %s", out)
	}
	mp4 := globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "tutorial.mp4"))
	format := func() string {
		cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "json", mp4)
		o, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ffprobe: %v %s", err, o)
		}
		return string(o)
	}()
	var fj struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	_ = json.Unmarshal([]byte(format), &fj)
	var dur float64
	fmt.Sscanf(fj.Format.Duration, "%f", &dur)
	if d := math.Abs(dur - ft.TotalS); d > 0.6 {
		t.Fatalf("mp4 %.2fs vs final timeline %.2fs", dur, ft.TotalS)
	}

	// 9. Frame evidence on the RAW capture. Same reasoning as the V1 suite:
	// a 550ms transient cannot be proven visible on a lossy recording under
	// host lag, so dispatch is proven by the `ripple` capture event
	// (section 1) and pixels prove sustained change (typing, result).
	raw := globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "raw", "scene-talk.webm"))
	frames := t.TempDir()

	// Typing progression on the message input.
	var fillInput *finalSeg
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind == "action" && strings.Contains(sg.Label, "cinematic-input") {
			fillInput = sg
		}
	}
	if fillInput == nil || fillInput.NormBBox == nil {
		t.Fatal("message input fill segment missing")
	}
	fnb := &finalSegBox{X: fillInput.NormBBox.X, Y: fillInput.NormBBox.Y, Width: fillInput.NormBBox.Width, Height: fillInput.NormBBox.Height}
	win := fillInput.VideoEndS - fillInput.VideoStartS
	f1, f2, f3 := filepath.Join(frames, "m1.png"), filepath.Join(frames, "m2.png"), filepath.Join(frames, "m3.png")
	extractFrame(t, raw, f1, fillInput.VideoStartS+win*0.2)
	extractFrame(t, raw, f2, fillInput.VideoStartS+win*0.45)
	extractFrame(t, raw, f3, fillInput.VideoStartS+win*0.7)
	if d12 := meanAbsDiffRegion(t, f1, f2, fnb, 1280, 720); d12 < 0.3 {
		t.Fatalf("typing not progressive (first diff %.2f)", d12)
	} else if d23 := meanAbsDiffRegion(t, f2, f3, fnb, 1280, 720); d23 < 0.3 {
		t.Fatalf("typing not progressive (second diff %.2f)", d23)
	}

	// Result appearance: success state differs from pre-send frame.
	res := filepath.Join(frames, "result.png")
	extractFrame(t, raw, res, sendSeg.VideoEndS-0.3)
	before := filepath.Join(frames, "before-send.png")
	extractFrame(t, raw, before, sendSeg.VideoStartS+0.2)
	if d := meanAbsDiffFull(t, before, res); d < 0.5 {
		t.Fatalf("result confirmation invisible (full-frame diff %.2f)", d)
	}

	// 10. Bundle still complete and secret-free.
	bundle := filepath.Join(work, "bundle")
	run(t, work, autodoc, "export", "--storyboard", "storyboard.yml", "--out", bundle)
	assertBundle(t, bundle)
	assertNoSecrets(t, bundle)
}

func meanAbsDiffFull(t *testing.T, fa, fb string) float64 {
	t.Helper()
	wa, ha, ga := decodePNG(t, fa)
	wb, hb, gb := decodePNG(t, fb)
	if wa != wb || ha != hb {
		t.Fatalf("frame size mismatch %dx%d vs %dx%d", wa, ha, wb, hb)
	}
	var sum float64
	for i := range ga {
		d := ga[i] - gb[i]
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum / float64(len(ga))
}
