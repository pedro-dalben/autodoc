package media_test

import (
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func mockUI(t *testing.T, dir string) string {
	t.Helper()
	raw := filepath.Join(dir, "raw-login.mp4")
	filters := "drawbox=x=410:y=200:w=460:h=320:color=#e8ecf4:t=fill," +
		"drawbox=x=410:y=200:w=460:h=320:color=#9aa4b5:t=3," +
		"drawbox=x=450:y=280:w=380:h=56:color=white:t=fill," +
		"drawbox=x=450:y=280:w=380:h=56:color=#4f8cff:t=3," +
		"drawbox=x=450:y=352:w=380:h=56:color=white:t=fill," +
		"drawbox=x=450:y=352:w=380:h=56:color=#4f8cff:t=3," +
		"drawbox=x=450:y=440:w=380:h=56:color=#16a34a:t=fill," +
		"drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='Sign in':fontsize=34:fontcolor=#1a2030:x=450:y=225," +
		"drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf:text='demo@example.com':fontsize=22:fontcolor=#333333:x=462:y=296," +
		"drawtext=fontfile=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf:text='ENTRAR':fontsize=24:fontcolor=white:x=590:y=456"
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=#2b3345:s=1280x720:d=14:r=30",
		"-vf", filters, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-r", "30", raw)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mock UI: %v\n%s", err, out)
	}
	return raw
}

func dirSegments() []timeline.AVSegment {
	field := &visual.BBox{X: 450.0 / 1280, Y: 280.0 / 720, Width: 380.0 / 1280, Height: 56.0 / 720}
	button := &visual.BBox{X: 450.0 / 1280, Y: 440.0 / 720, Width: 380.0 / 1280, Height: 56.0 / 720}
	form := &visual.BBox{X: 410.0 / 1280, Y: 200.0 / 720, Width: 460.0 / 1280, Height: 320.0 / 720}
	return []timeline.AVSegment{
		{SceneID: "login", BeatID: "b1", Kind: "speech", Label: "opening", StartS: 0, DurS: 2, VideoStartS: 0, VideoEndS: 2, Speed: 1, Zoom: 1},
		{SceneID: "login", BeatID: "b1", Kind: "action", Label: "fill email", StartS: 2, DurS: 2, VideoStartS: 2, VideoEndS: 4, Speed: 1, NormBBox: field},
		{SceneID: "login", BeatID: "b1", Kind: "action", Label: "press Enter", StartS: 4, DurS: 2, VideoStartS: 4, VideoEndS: 6, Speed: 1, NormBBox: form},
		{SceneID: "login", BeatID: "b1", Kind: "action", Label: "click entrar", StartS: 6, DurS: 2, VideoStartS: 6, VideoEndS: 8, Speed: 1, NormBBox: button},
		{SceneID: "home", BeatID: "b1", Kind: "speech", Label: "result", StartS: 8, DurS: 2, VideoStartS: 8, VideoEndS: 10, Speed: 1, Zoom: 1},
	}
}

func withVariant(segs []timeline.AVSegment, variant string) []timeline.AVSegment {
	out := make([]timeline.AVSegment, len(segs))
	copy(out, segs)
	for i := range out {
		sg := &out[i]
		switch variant {
		case "auto":
			if sg.Kind == "action" {
				sg.Zoom = 1.12
			}
		case "directed":
			if sg.Kind == "action" {
				sg.Zoom = 1.25
				sg.CameraTransition = "zoom_isolated"
			}
			if sg.Label == "fill email" {
				sg.Outline = true
			}
			if sg.Label == "press Enter" {
				sg.Keyboard = true
				sg.KeyLabel = "ENTER"
			}
			if sg.Label == "click entrar" {
				sg.ResultFlash = true
			}
		case "zoomonly":
			if sg.Kind == "action" {
				sg.Zoom = 1.25
				sg.CameraTransition = "zoom_isolated"
			}
		case "minimal":
			sg.Zoom = 1
		}
	}
	return out
}

func renderVariant(t *testing.T, dir, raw, variant string) string {
	t.Helper()
	segs := withVariant(dirSegments(), variant)
	ft := &timeline.FinalTimeline{
		StoryboardHash: "direction-" + variant,
		Segments:       segs, TotalS: 10,
		SceneClips: []timeline.SceneClip{{SceneID: "login", StartS: 0, EndS: 8, Duration: 8}, {SceneID: "home", StartS: 8, EndS: 10, Duration: 2}},
	}
	out := filepath.Join(dir, variant+".mp4")
	err := media.RenderCinematic(ft, media.CinematicOptions{
		Width: 1280, Height: 720, FPS: 30,
		SceneVideo: map[string]string{"login": raw, "home": raw},
		WorkDir:    dir, OutputMP4: out,
	})
	if err != nil {
		t.Fatalf("render %s: %v", variant, err)
	}
	return out
}

func grabFrame(t *testing.T, dir, mp4 string, at float64, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	cmd := exec.Command("ffmpeg", "-y", "-ss", fmt.Sprintf("%.2f", at), "-i", mp4, "-frames:v", "1", p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("frame %s: %v\n%s", name, err, out)
	}
	return p
}

func regionPixels(t *testing.T, pngPath string) ([][]float64, int, int) {
	t.Helper()
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	px := make([][]float64, h)
	for y := 0; y < h; y++ {
		px[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			px[y][x] = float64(r+g+bl) / 3 / 257
		}
	}
	return px, w, h
}

func regionMean(px [][]float64, x0, y0, x1, y1 int) float64 {
	var sum float64
	n := 0
	for y := y0; y < y1 && y < len(px); y++ {
		for x := x0; x < x1 && x < len(px[y]); x++ {
			sum += px[y][x]
			n++
		}
	}
	return sum / float64(n)
}

func countBright(px [][]float64, x0, y0, x1, y1 int, thresh float64) int {
	n := 0
	for y := y0; y < y1 && y < len(px); y++ {
		for x := x0; x < x1 && x < len(px[y]); x++ {
			if px[y][x] >= thresh {
				n++
			}
		}
	}
	return n
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func TestRenderDirectionVariants(t *testing.T) {
	if _, err := media.CheckFFmpeg(); err != nil {
		t.Skip("ffmpeg missing")
	}
	dir := t.TempDir()
	raw := mockUI(t, dir)
	auto := renderVariant(t, dir, raw, "auto")
	directed := renderVariant(t, dir, raw, "directed")
	minimal := renderVariant(t, dir, raw, "minimal")
	zoomonly := renderVariant(t, dir, raw, "zoomonly")

	for _, mp4 := range []string{auto, directed, minimal, zoomonly} {
		probe, err := media.ProbeFile(mp4)
		if err != nil {
			t.Fatal(err)
		}
		if !probe.HasVideo || probe.VCodec != "h264" || probe.Width != 1280 || probe.Height != 720 {
			t.Fatalf("bad video %+v for %s", probe, mp4)
		}
		if fileSize(t, mp4) < 10_000 {
			t.Fatalf("suspiciously small render %s", mp4)
		}
	}

	dFill := grabFrame(t, dir, directed, 3.0, "directed-fill.png")
	mFill := grabFrame(t, dir, minimal, 3.0, "minimal-fill.png")
	d, _ := os.ReadFile(dFill)
	m, _ := os.ReadFile(mFill)
	if string(d) == string(m) {
		t.Error("directed zoom must change the fill frame vs minimal")
	}

	dPress := grabFrame(t, dir, directed, 5.0, "directed-press.png")
	zPress := grabFrame(t, dir, zoomonly, 5.0, "zoomonly-press.png")
	dpx, _, _ := regionPixels(t, dPress)
	zpx, _, _ := regionPixels(t, zPress)
	if got, want := countBright(dpx, 520, 560, 760, 640, 200), countBright(zpx, 520, 560, 760, 640, 200); got < 300 || got < 3*want {
		t.Errorf("keyboard ENTER text must appear in the pill zone: directed=%d zoomonly=%d", got, want)
	}

	dOut := grabFrame(t, dir, directed, 3.0, "directed-outline.png")
	zOut := grabFrame(t, dir, zoomonly, 3.0, "zoomonly.png")
	od, _ := os.ReadFile(dOut)
	oz, _ := os.ReadFile(zOut)
	if string(od) == string(oz) {
		t.Error("outline must change pixels vs zoom-only at identical zoom")
	}

	dRes := grabFrame(t, dir, directed, 7.0, "directed-result.png")
	mRes := grabFrame(t, dir, minimal, 7.0, "minimal-result.png")
	rd, _ := os.ReadFile(dRes)
	rm, _ := os.ReadFile(mRes)
	if string(rd) == string(rm) {
		t.Error("result flash must change the result frame vs minimal")
	}
}
