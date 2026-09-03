package e2e

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type finalSeg struct {
	SceneID     string  `json:"scene_id"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	StartS      float64 `json:"start_s"`
	DurS        float64 `json:"dur_s"`
	VideoStartS float64 `json:"video_start_s"`
	VideoEndS   float64 `json:"video_end_s"`
	ActionAtS   float64 `json:"action_at_s"`
	Zoom        float64 `json:"zoom"`
	NormBBox    *struct {
		X, Y, Width, Height float64 `json:","`
	} `json:"norm_bbox"`
	Estimated bool `json:"estimated"`
}

type finalTL struct {
	TotalS   float64    `json:"total_s"`
	Segments []finalSeg `json:"segments"`
	Sync     *struct {
		Pass       bool    `json:"pass"`
		MaxDriftMs float64 `json:"max_drift_ms"`
		Items      []struct {
			Scope string `json:"scope"`
			Kind  string `json:"kind"`
			Pass  bool   `json:"pass"`
		} `json:"items"`
	} `json:"sync"`
}

func globOne(t *testing.T, pattern string) string {
	t.Helper()
	m, err := filepath.Glob(pattern)
	if err != nil || len(m) == 0 {
		t.Fatalf("no match for %s", pattern)
	}
	return m[len(m)-1]
}

func extractFrame(t *testing.T, src, dst string, atS float64) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-y", "-v", "error", "-ss", fmt.Sprintf("%.3f", atS), "-i", src, "-frames:v", "1", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extract frame @%.2f: %v %s", atS, err, out)
	}
}

type finalSegBox struct{ X, Y, Width, Height float64 }

func decodePNG(t *testing.T, path string) (w, h int, gray []float64) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	w, h = b.Dx(), b.Dy()
	gray = make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 257.0
		}
	}
	return w, h, gray
}

func meanAbsDiffRegion(t *testing.T, fa, fb string, nb *finalSegBox, vw, vh int) float64 {
	t.Helper()
	wa, ha, ga := decodePNG(t, fa)
	_, _, gb := decodePNG(t, fb)
	_ = wa
	x0 := int(nb.X * float64(vw))
	y0 := int(nb.Y * float64(vh))
	x1 := int((nb.X + nb.Width) * float64(vw))
	y1 := int((nb.Y + nb.Height) * float64(vh))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > wa {
		x1 = wa
	}
	if y1 > ha {
		y1 = ha
	}
	var sum float64
	var n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sum += math.Abs(ga[y*wa+x] - gb[y*wa+x])
			n++
		}
	}
	if n == 0 {
		t.Fatal("empty bbox region")
	}
	return sum / float64(n)
}

func TestCinematicVisualCuesAndSync(t *testing.T) {
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

	for _, f := range []string{"storyboard.yml", "autodoc.toml"} {
		sb, err := os.ReadFile(filepath.Join(root, "test", "fixture", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, f), sb, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, work, autodoc, "storyboard", "validate", "--storyboard", "storyboard.yml")
	run(t, work, autodoc, "tts", "--storyboard", "storyboard.yml")
	out := run(t, work, autodoc, "record", "--storyboard", "storyboard.yml", "--headless")
	if !strings.Contains(out, "scene-002") {
		t.Fatalf("record must cover scene-002: %s", out)
	}

	// 1. Capture evidence: bbox + progressive typing + speech windows.
	eventsRaw, err := os.ReadFile(globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "events-scene-002.jsonl")))
	if err != nil {
		t.Fatal(err)
	}
	events := string(eventsRaw)
	for _, want := range []string{`interaction`, `bbox`, `"kind":"speech_start"`, `"kind":"speech_end"`, `typing_chars`, `progressive`} {
		if !strings.Contains(events, want) {
			t.Fatalf("events-scene-002.jsonl missing %s", want)
		}
	}
	if strings.Contains(events, "Areia lavada") {
		t.Fatal("typed value must not leak into events (labels only)")
	}

	// 2. Final timeline + sync PASS.
	finalRaw, err := os.ReadFile(filepath.Join(work, ".autodoc", "_work", "final_timeline.json"))
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
	if math.Abs(ft.Sync.MaxDriftMs) > 250 {
		t.Fatalf("max drift %.0fms exceeds budget", ft.Sync.MaxDriftMs)
	}

	var clickNovo, fillName *finalSeg
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind == "action" && strings.Contains(sg.Label, "new-material-btn") {
			clickNovo = sg
		}
		if sg.Kind == "action" && strings.Contains(sg.Label, "material-name") {
			fillName = sg
		}
	}
	if clickNovo == nil || fillName == nil {
		t.Fatalf("action segments missing (click=%v fill=%v)", clickNovo != nil, fillName != nil)
	}
	if clickNovo.Zoom <= 1.01 || clickNovo.NormBBox == nil {
		t.Fatalf("click must carry camera zoom + bbox: %+v", clickNovo)
	}
	for _, sg := range ft.Segments {
		if sg.Estimated {
			t.Fatalf("no segment may be estimated after fresh record: %+v", sg)
		}
	}

	// 3. Render cinematic + debug cues.
	out = run(t, work, autodoc, "render", "--storyboard", "storyboard.yml", "--debug-cues", "--debug-timeline")
	if !strings.Contains(out, "AutoDoc Sync") {
		t.Fatalf("render must print sync report: %s", out)
	}
	mp4 := globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "tutorial.mp4"))
	probe := func(args ...string) string {
		cmd := exec.Command("ffprobe", args...)
		o, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ffprobe: %v %s", err, o)
		}
		return string(o)
	}
	format := probe("-v", "error", "-show_entries", "format=duration", "-of", "json", mp4)
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

	// 4. Frame evidence on the RAW capture (cues baked in).
	raw := globOne(t, filepath.Join(work, ".autodoc", "_work", "*", "raw", "scene-002.webm"))
	frames := t.TempDir()
	nb := &finalSegBox{X: clickNovo.NormBBox.X, Y: clickNovo.NormBBox.Y, Width: clickNovo.NormBBox.Width, Height: clickNovo.NormBBox.Height}
	pre := filepath.Join(frames, "click-pre.png")
	rip := filepath.Join(frames, "click-ripple.png")
	extractFrame(t, raw, pre, clickNovo.ActionAtS-0.3)
	extractFrame(t, raw, rip, clickNovo.ActionAtS+0.2)
	if d := meanAbsDiffRegion(t, pre, rip, nb, 1280, 720); d < 1.5 {
		t.Fatalf("click ripple invisible in bbox region (mean diff %.2f)", d)
	}
	fnb := &finalSegBox{X: fillName.NormBBox.X, Y: fillName.NormBBox.Y, Width: fillName.NormBBox.Width, Height: fillName.NormBBox.Height}
	win := fillName.VideoEndS - fillName.VideoStartS
	f1, f2, f3 := filepath.Join(frames, "t1.png"), filepath.Join(frames, "t2.png"), filepath.Join(frames, "t3.png")
	extractFrame(t, raw, f1, fillName.VideoStartS+win*0.2)
	extractFrame(t, raw, f2, fillName.VideoStartS+win*0.45)
	extractFrame(t, raw, f3, fillName.VideoStartS+win*0.7)
	d12 := meanAbsDiffRegion(t, f1, f2, fnb, 1280, 720)
	d23 := meanAbsDiffRegion(t, f2, f3, fnb, 1280, 720)
	if d12 < 0.3 || d23 < 0.3 {
		t.Fatalf("typing not progressive (diffs %.2f %.2f)", d12, d23)
	}

	// 5. Bundle still complete and secret-free.
	bundle := filepath.Join(work, "bundle")
	run(t, work, autodoc, "export", "--storyboard", "storyboard.yml", "--out", bundle)
	assertBundle(t, bundle)
	assertNoSecrets(t, bundle)
	if _, err := os.Stat(filepath.Join(bundle, "final_timeline.json")); err != nil {
		t.Fatal("bundle must ship final_timeline.json")
	}
}
