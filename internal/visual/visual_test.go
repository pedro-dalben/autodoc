package visual

import (
	"strings"
	"testing"
)

func TestDefaultsAreCinematic(t *testing.T) {
	c := Default()
	c.ApplyDefaults()
	if !c.CursorOn() || !c.RippleOn() || !c.HiliteOn() || !c.CameraOn() {
		t.Fatal("cinematic cues must be on by default")
	}
	if c.Typing.CharDelayMs < 30 || c.Typing.CharDelayMs > 80 {
		t.Fatalf("char delay %d outside 30..80ms", c.Typing.CharDelayMs)
	}
	if c.Cursor.MaxMs > 450 || c.Cursor.MinMs < 150 {
		t.Fatalf("cursor motion out of 150..450ms envelope: %+v", c.Cursor)
	}
}

func TestCursorMoveScalesWithDistance(t *testing.T) {
	cfg := Default().Cursor
	near := CursorMoveMs(Point{0, 0}, Point{10, 0}, cfg)
	far := CursorMoveMs(Point{0, 0}, Point{1000, 800}, cfg)
	if near <= 0 || far <= 0 {
		t.Fatal("moves must take positive time")
	}
	if far < near {
		t.Fatal("longer distance must not be faster")
	}
	if far > cfg.MaxMs || near < cfg.MinMs {
		t.Fatalf("motion outside envelope: near=%d far=%d", near, far)
	}
	if ms := CursorMoveMs(Point{5, 5}, Point{5.5, 5}, cfg); ms != 0 {
		t.Fatalf("teleport-adjacent points must not animate, got %d", ms)
	}
}

func TestCursorInterpolationEases(t *testing.T) {
	from, to := Point{0, 0}, Point{100, 0}
	mid := CursorAt(from, to, 50, 100)
	if mid.X <= 0 || mid.X >= 100 {
		t.Fatalf("midpoint outside path: %+v", mid)
	}
	q1 := CursorAt(from, to, 25, 100)
	q3 := CursorAt(from, to, 75, 100)
	if q3.X-q1.X <= 0 {
		t.Fatal("cursor must advance monotonically")
	}
	if got := CursorAt(from, to, 0, 100); got != from {
		t.Fatal("t=0 must be origin")
	}
	if got := CursorAt(from, to, 200, 100); got != to {
		t.Fatal("t>=total must be target")
	}
}

func TestCameraSafeArea(t *testing.T) {
	cfg := Default().Camera
	shot := PlanCamera(BBox{X: 820, Y: 412, Width: 180, Height: 42}, 1280, 720, cfg.ClickZoom, cfg)
	if !shot.Apply || shot.Zoom <= 1.01 {
		t.Fatalf("small button must zoom: %+v", shot)
	}
	if shot.CropX < 0 || shot.CropY < 0 || shot.CropX+shot.CropW > 1280 || shot.CropY+shot.CropH > 720 {
		t.Fatalf("crop escapes frame: %+v", shot)
	}
	big := PlanCamera(BBox{X: 100, Y: 100, Width: 1100, Height: 600}, 1280, 720, cfg.ClickZoom, cfg)
	if big.Apply {
		t.Fatal("near-fullscreen modal must not zoom")
	}
	tiny := PlanCamera(BBox{X: 5, Y: 5, Width: 4, Height: 4}, 1280, 720, cfg.ClickZoom, cfg)
	if tiny.Apply {
		t.Fatal("noise-sized target must not zoom")
	}
	if s := PlanCamera(BBox{}, 1280, 720, cfg.ClickZoom, cfg); s.Apply {
		t.Fatal("missing bbox must not zoom")
	}
}

func TestOverlayIsIsolated(t *testing.T) {
	for _, want := range []string{"pointer-events:none", "aria-hidden", "data-autodoc-overlay", "2147483646"} {
		if !strings.Contains(OverlayJS, want) {
			t.Fatalf("overlay missing isolation marker %q", want)
		}
	}
	for _, fn := range []string{CursorMoveJS(Point{}, Point{X: 1}, 10), HighlightJS(BBox{Width: 1, Height: 1}, 5), RippleJS(Point{}, 5)} {
		if !strings.Contains(fn, "__autodocCues") {
			t.Fatalf("helper does not target overlay root: %s", fn)
		}
	}
}
