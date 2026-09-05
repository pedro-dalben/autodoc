package cinematic

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// TestDirectedValidationRejectsConflicts proves contradictory direction
// is a compact actionable error, never a silent mis-render.
func TestDirectedValidationRejectsConflicts(t *testing.T) {
	sb, err := storyboard.LoadFile("../../test/fixture/storyboard-directed.yml")
	if err != nil {
		t.Fatal(err)
	}
	sb.Direction.Cursor = "hide"
	sb.Direction.CursorHalo = "on"
	if errs := sb.Validate(); len(errs) == 0 {
		t.Error("cursor=hide + halo=on must fail storyboard validation")
	}
	sb.Direction.CursorHalo = "off"
	sb.Scenes[0].Direction.Camera = "static"
	sb.Scenes[0].Direction.Zoom = "extreme"
	if errs := sb.Validate(); len(errs) == 0 {
		t.Error("camera=static + zoom=extreme must fail storyboard validation")
	}
}

// fixture: NL intent -> storyboard direction -> resolver -> directors ->
// effect timeline -> render annotations -> directive QA. No browser.
func TestDirectedLoginEndToEnd(t *testing.T) {
	sb, err := storyboard.LoadFile("../../test/fixture/storyboard-directed.yml")
	if err != nil {
		t.Fatal(err)
	}
	if errs := sb.Validate(); len(errs) != 0 {
		t.Fatalf("directed fixture must validate: %v", errs)
	}
	if sb.Direction == nil || sb.Scenes[0].Direction == nil {
		t.Fatal("direction blocks must survive YAML round-trip")
	}
	r := recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
	formBox := &visual.BBox{X: 0.32, Y: 0.30, Width: 0.36, Height: 0.30}
	fieldBox := &visual.BBox{X: 0.35, Y: 0.42, Width: 0.30, Height: 0.07}
	ft := &timeline.FinalTimeline{StoryboardHash: r.StoryboardHash}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "login", BeatID: "beat-01", Kind: "speech", Label: "login-beat-01-speech-001", StartS: 0, DurS: 2, VideoStartS: 0, VideoEndS: 2, Speed: 1},
		{SceneID: "login", BeatID: "beat-01", Kind: "action", Label: "fill [data-testid=\"login-email\"]", StartS: 2, DurS: 1.4, VideoStartS: 2, VideoEndS: 3.4, Speed: 1, NormBBox: fieldBox},
		{SceneID: "login", BeatID: "beat-01", Kind: "action", Label: "fill [data-testid=\"login-password\"]", StartS: 3.4, DurS: 1.4, VideoStartS: 3.4, VideoEndS: 4.8, Speed: 1, NormBBox: fieldBox},
		{SceneID: "login", BeatID: "beat-01", Kind: "speech", Label: "login-beat-01-speech-002", StartS: 4.8, DurS: 2, VideoStartS: 4.8, VideoEndS: 6.8, Speed: 1},
		{SceneID: "login", BeatID: "beat-01", Kind: "action", Label: "press Enter", StartS: 6.8, DurS: 0.8, VideoStartS: 6.8, VideoEndS: 7.6, Speed: 1, NormBBox: formBox},
		{SceneID: "home", BeatID: "beat-01", Kind: "speech", Label: "home-beat-01-speech-001", StartS: 7.6, DurS: 2, VideoStartS: 7.6, VideoEndS: 9.6, Speed: 1},
	}
	ft.TotalS = 9.6
	vis := visual.Default()
	cine := visual.DefaultCinematic()
	bundle := Direct(r, sb, ft, nil, vis, cine)

	// 1. Scene-level zoom=strong lands on login actions...
	for _, sg := range ft.Segments {
		if sg.SceneID == "login" && sg.Kind == "action" && sg.Zoom <= 1.01 {
			t.Errorf("login action must be zoomed (scene zoom=strong): %+v", sg)
		}
	}
	// ...and nowhere else (no tutorial zoom set).
	for _, d := range bundle.Camera.Decisions {
		if d.SceneID == "home" && d.Zoom > 1.01 {
			t.Errorf("home must stay wide: %+v", d)
		}
	}
	// 2. Tutorial spotlight=off kills every dim.
	for _, d := range bundle.Attention.Decisions {
		if d.Spotlight {
			t.Errorf("spotlight must be off everywhere: %+v", d)
		}
	}
	// 3. Keyboard overlay planned for the Enter press with label.
	foundKeys := false
	for _, sg := range ft.Segments {
		if sg.Keyboard {
			foundKeys = true
			if sg.KeyLabel != "ENTER" {
				t.Errorf("keyboard label must be ENTER, got %q", sg.KeyLabel)
			}
		}
	}
	if !foundKeys {
		t.Error("Enter press must plan a keyboard overlay")
	}
	// 4. Scene focus=outline annotates login actions.
	foundOutline := false
	for _, sg := range ft.Segments {
		if sg.SceneID == "login" && sg.Kind == "action" && sg.Outline {
			foundOutline = true
		}
	}
	if !foundOutline {
		t.Error("login actions must carry the outline flag")
	}
	// 5. Camera lock pins the form shot (no restore inside the scene).
	for _, d := range bundle.Camera.Decisions {
		if d.SceneID == "login" && (d.Move == CamZoomOut || d.Move == CamContextRestore) {
			t.Errorf("camera lock must suppress %s inside login: %+v", d.Move, d)
		}
	}
	// 6. Resolved sources prove precedence (scene > tutorial > director).
	seenScene, seenTutorial := false, false
	for _, rb := range bundle.Resolved {
		if rb.SceneID == "login" && rb.Sources["zoom"] == SourceScene {
			seenScene = true
		}
		if rb.Sources["spotlight"] == SourceTutorial {
			seenTutorial = true
		}
		if rb.Sources["cursor"] == SourceDirector && rb.Cursor != "auto" {
			t.Errorf("director-owned cursor must read auto: %+v", rb)
		}
	}
	if !seenScene || !seenTutorial {
		t.Error("resolved sources must show scene + tutorial origins")
	}
	// 7. Effect timeline coherent, collision-free.
	if bundle.Effects == nil || len(bundle.Effects.Events) == 0 {
		t.Fatal("effect timeline must not be empty")
	}
	if len(bundle.Effects.Collisions) != 0 {
		t.Errorf("effect collisions: %v", bundle.Effects.Collisions)
	}
	// 8. Directive compliance: every explicit directive honored.
	if bundle.Compliance == nil || bundle.Compliance.Requested == 0 {
		t.Fatal("compliance must detect user directives")
	}
	if bundle.Compliance.Ignored != 0 {
		t.Errorf("directives ignored:\n%s", bundle.Compliance.Print())
	}
}

// TestDirectedMinimalEndToEnd proves the no-effects render: zoom off,
// spotlight off, callouts off, keyboard off across the same beats.
func TestDirectedMinimalEndToEnd(t *testing.T) {
	sb, err := storyboard.LoadFile("../../test/fixture/storyboard-directed.yml")
	if err != nil {
		t.Fatal(err)
	}
	off := visual.DirectionConfig{Zoom: "off", Spotlight: "off", Callout: "off", Keyboard: "off", Focus: "off"}
	sb.Direction = &off
	sb.Scenes[0].Direction = nil
	r := recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
	formBox := &visual.BBox{X: 0.32, Y: 0.30, Width: 0.36, Height: 0.30}
	ft := &timeline.FinalTimeline{StoryboardHash: r.StoryboardHash}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "login", BeatID: "beat-01", Kind: "action", Label: "press Enter", StartS: 0, DurS: 1, VideoStartS: 0, VideoEndS: 1, Speed: 1, NormBBox: formBox},
	}
	ft.TotalS = 1
	vis := visual.Default()
	cine := visual.DefaultCinematic()
	bundle := Direct(r, sb, ft, nil, vis, cine)
	for _, d := range bundle.Camera.Decisions {
		if d.Zoom > 1.01 {
			t.Errorf("minimal render must have zero zoom events: %+v", d)
		}
	}
	for _, sg := range ft.Segments {
		if sg.Keyboard || sg.Outline || sg.ResultFlash {
			t.Errorf("minimal render must plan no overlays: %+v", sg)
		}
	}
	if len(bundle.Callouts) != 0 {
		t.Errorf("minimal render must plan no callouts: %v", bundle.Callouts)
	}
	if bundle.Compliance.Ignored != 0 {
		t.Errorf("minimal directives ignored:\n%s", bundle.Compliance.Print())
	}
}
