package visual

import (
	"strings"
	"testing"
)

func TestDirectionMerge(t *testing.T) {
	base := DirectionConfig{Zoom: "off", Cursor: "show"}
	over := DirectionConfig{Zoom: "strong"}
	got := base.Merge(&over)
	if got.Zoom != "strong" || got.Cursor != "show" {
		t.Errorf("merge must overlay per-field, got %+v", got)
	}
	if got.Merge(nil) != got {
		t.Errorf("nil merge must be identity")
	}
}

func TestDirectionPreset(t *testing.T) {
	d := DirectionConfig{Zoom: "off"}.WithPreset("dynamic")
	if d.Zoom != "off" {
		t.Error("explicit zoom must survive preset fill")
	}
	if d.Preset != "dynamic" {
		t.Errorf("preset must be recorded, got %q", d.Preset)
	}
	if d.Keyboard == "" || d.Keyboard == "auto" {
		// dynamic/training presets set keyboard explicitly.
		t.Errorf("dynamic preset should set keyboard, got %q", d.Keyboard)
	}
	min := DirectionConfig{}.WithPreset("minimal")
	if min.Zoom != "off" || min.Spotlight != "off" || min.Camera != "static" {
		t.Errorf("minimal preset wrong: %+v", min)
	}
}

func TestDirectionEmpty(t *testing.T) {
	var nilDir *DirectionConfig
	if !nilDir.Empty() {
		t.Error("nil direction must be empty")
	}
	zero := DirectionConfig{}
	if !zero.Empty() {
		t.Error("zero direction must be empty")
	}
	set := DirectionConfig{Zoom: "off"}
	if set.Empty() {
		t.Error("set direction must not be empty")
	}
}

func TestCapabilitiesCompact(t *testing.T) {
	caps := Capabilities()
	for _, need := range []string{"zoom:", "cursor:", "keyboard:", "spotlight:", "precedence:"} {
		if !strings.Contains(caps, need) {
			t.Errorf("capabilities missing %q", need)
		}
	}
	if len(caps) > 1200 {
		t.Errorf("capabilities must stay compact, got %d bytes", len(caps))
	}
}

func TestZoomScaleMap(t *testing.T) {
	if ZoomScale("off") != 1.0 {
		t.Error("off must map to 1.0")
	}
	if ZoomScale("extreme") <= ZoomScale("strong") {
		t.Error("extreme must exceed strong")
	}
	if ZoomScale("auto") != 0 || ZoomScale("") != 0 {
		t.Error("auto/unset must map to 0 (director decides)")
	}
}
