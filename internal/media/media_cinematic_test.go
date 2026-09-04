package media_test

import (
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func TestZoomChainTransitions(t *testing.T) {
	sg := timeline.AVSegment{
		DurS:     2.0,
		Zoom:     1.2,
		NormBBox: &visual.BBox{X: 0.2, Y: 0.3, Width: 0.2, Height: 0.1},
	}

	// 1. zoom_stay
	sg.CameraTransition = "zoom_stay"
	chainStay := media.ZoomChain(sg, 1280, 720)
	if !strings.Contains(chainStay, "1.2000") {
		t.Errorf("zoom_stay should contain constant zoom 1.2000, got: %s", chainStay)
	}
	if strings.Contains(chainStay, "pow") {
		t.Errorf("zoom_stay should NOT contain dynamic pow easing, got: %s", chainStay)
	}

	// 2. zoom_in
	sg.CameraTransition = "zoom_in"
	chainIn := media.ZoomChain(sg, 1280, 720)
	if !strings.Contains(chainIn, "lt(t,") && !strings.Contains(chainIn, "lt(t\\,") {
		t.Errorf("zoom_in should contain lt(t, got: %s", chainIn)
	}
	if strings.Contains(chainIn, "gt(t") {
		t.Errorf("zoom_in should NOT contain gt(t (no zoom out), got: %s", chainIn)
	}

	// 3. zoom_out
	sg.CameraTransition = "zoom_out"
	chainOut := media.ZoomChain(sg, 1280, 720)
	if !strings.Contains(chainOut, "gt(t") {
		t.Errorf("zoom_out should contain gt(t, got: %s", chainOut)
	}
	if strings.Contains(chainOut, "lt(t") {
		t.Errorf("zoom_out should NOT contain lt(t (no zoom in), got: %s", chainOut)
	}

	// 4. zoom_isolated
	sg.CameraTransition = "zoom_isolated"
	chainIso := media.ZoomChain(sg, 1280, 720)
	if !strings.Contains(chainIso, "lt(t") || !strings.Contains(chainIso, "gt(t") {
		t.Errorf("zoom_isolated should contain both lt(t and gt(t, got: %s", chainIso)
	}
}
