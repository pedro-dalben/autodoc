package cinematic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainRun(t *testing.T) {
	d := t.TempDir()

	plan := CinematicPlan{
		StoryboardHash: "hash123",
		CameraAdjusted: 1,
		HoldsAddedMs:   500,
		Camera: &CameraPlan{
			Decisions: []CameraDecision{
				{SegmentIdx: 0, SceneID: "scene-1", Kind: "action", Label: "click btn", Move: CamZoom, Zoom: 1.15, Reason: "focused-action"},
			},
		},
		Attention: &AttentionPlan{
			Decisions: []AttentionDecision{
				{BeatIndex: 0, SceneID: "scene-1", Strategy: AttSpotlight, Reason: "click-focus", Anticipate: true},
			},
		},
	}

	planBytes, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "cinematic_plan.json"), planBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	exps, text, err := ExplainRun(d, "")
	if err != nil {
		t.Fatalf("ExplainRun failed: %v", err)
	}
	if len(exps) != 1 || exps[0].SceneID != "scene-1" {
		t.Fatalf("unexpected explanations: %+v", exps)
	}
	if !strings.Contains(text, "focused-action") {
		t.Errorf("expected text to mention focused-action, got:\n%s", text)
	}
	if !strings.Contains(text, "spotlight") {
		t.Errorf("expected text to mention spotlight, got:\n%s", text)
	}
}
