package media_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/tts"
)

func testTimeline(t *testing.T, dir string) *timeline.Timeline {
	t.Helper()
	wavs := []string{}
	texts := []string{"No menu lateral, acesse Materiais.", "Nesta tela encontramos os materiais cadastrados."}
	durs := []float64{}
	for i := range texts {
		p := filepath.Join(dir, string(rune('a'+i))+".wav")
		if err := os.WriteFile(p, tts.SineWAV(1.0+float64(i), 22050, 220), 0o644); err != nil {
			t.Fatal(err)
		}
		wavs = append(wavs, p)
		d, _ := tts.WavDurationSeconds(mustRead(t, p))
		durs = append(durs, d)
	}
	r := &recipe.Recipe{
		StoryboardHash: "h",
		Scenes:         []recipe.ScenePlan{{ID: "scene-001", Beats: []recipe.BeatPlan{{ID: "b1", Steps: []recipe.StepPlan{}}}}},
	}
	_ = r
	tl := &timeline.Timeline{
		StoryboardHash: "h",
		Segments: []timeline.Segment{
			{SpeechID: "s1", Text: texts[0], StartS: 0, EndS: durs[0], DurationS: durs[0], WavPath: wavs[0]},
			{SpeechID: "s2", Text: texts[1], StartS: durs[0] + 0.6, EndS: durs[0] + 0.6 + durs[1], DurationS: durs[1], WavPath: wavs[1]},
		},
	}
	tl.TotalS = durs[0] + 0.6 + durs[1] + 0.6
	return tl
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFinalMP4Properties(t *testing.T) {
	if _, err := media.CheckFFmpeg(); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	tl := testTimeline(t, dir)
	out := filepath.Join(dir, "tutorial.mp4")
	if err := media.BuildFinalMP4(tl, media.RenderOptions{Width: 1280, Height: 720, FPS: 30, OutputMP4: out}); err != nil {
		t.Fatalf("render: %v", err)
	}
	probe, err := media.ProbeFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !probe.HasVideo || probe.VCodec != "h264" {
		t.Fatalf("bad video: %+v", probe)
	}
	if !probe.HasAudio || probe.ACodec != "aac" {
		t.Fatalf("bad audio: %+v", probe)
	}
	if probe.Width != 1280 || probe.Height != 720 {
		t.Fatalf("bad resolution: %+v", probe)
	}
	if diff := probe.Duration - tl.TotalS; diff > 1.0 || diff < -1.0 {
		t.Fatalf("duration %.2f vs timeline %.2f", probe.Duration, tl.TotalS)
	}
}

func TestSubtitlesAndThumbnail(t *testing.T) {
	if _, err := media.CheckFFmpeg(); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	tl := testTimeline(t, dir)
	if err := media.WriteSRT(tl, filepath.Join(dir, "subtitles.srt")); err != nil {
		t.Fatal(err)
	}
	if err := media.WriteVTT(tl, filepath.Join(dir, "subtitles.vtt")); err != nil {
		t.Fatal(err)
	}
	srt := string(mustRead(t, filepath.Join(dir, "subtitles.srt")))
	if len(srt) < 20 || !contains(srt, "00:00:00,000 -->") {
		t.Fatalf("bad srt: %q", srt[:min(200, len(srt))])
	}
	out := filepath.Join(dir, "tutorial.mp4")
	if err := media.BuildFinalMP4(tl, media.RenderOptions{Width: 640, Height: 360, FPS: 15, OutputMP4: out}); err != nil {
		t.Fatal(err)
	}
	if err := media.MakeThumbnail(out, filepath.Join(dir, "thumbnail.png"), 0.5, 320); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "thumbnail.png")); err != nil || st.Size() == 0 {
		t.Fatal("thumbnail missing")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
