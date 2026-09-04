package cinematic

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// TestIntegrationStoryboardToScenePlan proves storyboard -> scene plan
// with anchors and overrides flowing through.
func TestIntegrationStoryboardToScenePlan(t *testing.T) {
	sb, err := storyboard.LoadFile("../../test/fixture/cinematic-v2.yml")
	if err != nil {
		t.Fatal(err)
	}
	r := recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
	doc := PlanScenes(r, sb)
	if len(doc.Beats) == 0 {
		t.Fatal("no beats planned")
	}
	// Callouts declared in the storyboard survive planning.
	found := 0
	for _, b := range doc.Beats {
		if b.Callout != "" {
			found++
		}
	}
	if found != 3 {
		t.Fatalf("want 3 callouts, got %d", found)
	}
	// Anchored narration present; override on the info icon present.
	anchored := 0
	overridden := 0
	for _, b := range doc.Beats {
		if b.Anchor.Kind != AnchorNone && (b.Type == SceneExplanation || b.Type == SceneNarration) {
			anchored++
		}
		if b.AttentionOverride == "none" {
			overridden++
		}
	}
	if anchored == 0 {
		t.Fatal("no anchored narration")
	}
	if overridden == 0 {
		t.Fatal("attention override lost")
	}
}

// TestIntegrationOldStoryboardCompatibility proves V1 storyboards plan
// cleanly with semantic defaults and no manual migration.
func TestIntegrationOldStoryboardCompatibility(t *testing.T) {
	sb, err := storyboard.LoadFile("../../test/fixture/storyboard.yml")
	if err != nil {
		t.Fatal(err)
	}
	if sb.Cinematic != nil {
		t.Fatal("V1 storyboard must not require a cinematic block")
	}
	cine := sb.CinematicOrDefault()
	if !cine.DirectorOn() || cine.CalloutsOn() || cine.SoundOn() {
		t.Fatalf("safe defaults: %+v", cine)
	}
	r := recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
	doc := PlanScenes(r, nil)
	if len(doc.Beats) == 0 {
		t.Fatal("no beats")
	}
	att := DirectAttention(doc.Beats, cine)
	if len(att.Decisions) != len(doc.Beats) {
		t.Fatal("attention covers every beat")
	}
}

// TestIntegrationEventsToEditPlan proves events -> edit plan -> render
// timeline reflow keeps speech intact and causality ordered.
func TestIntegrationEventsToEditPlan(t *testing.T) {
	cine := visual.DefaultCinematic()
	cine.Callouts.Enabled = boolPtr(true)
	sb, err := storyboard.LoadFile("../../test/fixture/cinematic-v2.yml")
	if err != nil {
		t.Fatal(err)
	}
	r := recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
	planned, err := timeline.Build(r, func(id string) (float64, bool, string) { return 2.0, false, "/tmp/x.wav" }, 600)
	if err != nil {
		t.Fatal(err)
	}
	// Simulated capture: speech windows exact, one long compressible wait.
	_ = planned
	ft := &timeline.FinalTimeline{StoryboardHash: r.StoryboardHash}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "scene-talk", BeatID: "beat-01", Kind: "speech", Label: "scene-talk-beat-01-speech-001", StartS: 0, DurS: 2, VideoStartS: 0, VideoEndS: 2, Speed: 1},
		{SceneID: "scene-talk", BeatID: "beat-01", Kind: "action", Label: "click [data-testid=\"conv-alice\"]", StartS: 2, DurS: 1.2, VideoStartS: 2, VideoEndS: 3.2, Speed: 1, Zoom: 1.08, NormBBox: bbox(0.05, 0.2, 0.12, 0.05)},
		{SceneID: "scene-talk", BeatID: "beat-01", Kind: "wait", Label: "visible", StartS: 3.2, DurS: 0.7, VideoStartS: 3.2, VideoEndS: 7.5, Speed: 6.1, Compressed: true, Compressible: true},
		{SceneID: "scene-talk", BeatID: "beat-03", Kind: "speech", Label: "scene-talk-beat-03-speech-001", StartS: 3.9, DurS: 2, VideoStartS: 7.5, VideoEndS: 9.5, Speed: 1},
	}
	ft.TotalS = 5.9
	doc := PlanScenes(r, sb)
	_ = doc
	vis := visual.Default()
	bundle := Direct(r, sb, ft, nil, vis, cine)
	if bundle.Edit == nil || len(bundle.Edit.Clips) == 0 {
		t.Fatal("empty edit plan")
	}
	// EDL separates what happened from how it is edited.
	types := map[string]bool{}
	for _, c := range bundle.Edit.Clips {
		types[string(c.Type)] = true
	}
	for _, want := range []string{"narration", "action", "transition"} {
		if !types[want] {
			t.Fatalf("EDL missing %s: %v", want, types)
		}
	}
	// Correlated beats produce anticipation + result clips for the
	// confirmed conversation click.
	if !types["anticipation"] || !types["result"] {
		t.Fatalf("EDL missing directed clips: %v", types)
	}
	// Causality: action clip precedes its transition/result clips.
	actionIdx, transIdx := -1, -1
	for i, c := range bundle.Edit.Clips {
		if c.Type == ClipAction && actionIdx < 0 {
			actionIdx = i
		}
		if c.Type == ClipTransition && transIdx < 0 {
			transIdx = i
		}
	}
	if actionIdx < 0 || transIdx < 0 || transIdx < actionIdx {
		t.Fatalf("causality violated: action=%d transition=%d", actionIdx, transIdx)
	}
	// Speech never compressed: narration clips keep full duration.
	for _, c := range bundle.Edit.Clips {
		if c.Type == ClipNarration && c.DurationMs != 2000 {
			t.Fatalf("speech compressed: %+v", c)
		}
	}
	if bundle.Report == nil || len(bundle.Report.Gates) == 0 {
		t.Fatal("missing QA report")
	}
}

func bbox(x, y, w, h float64) *visual.BBox { return &visual.BBox{X: x, Y: y, Width: w, Height: h} }

func boolPtr(v bool) *bool { return &v }
