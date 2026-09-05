package cinematic

import (
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func TestRegistryHasNoGimmicks(t *testing.T) {
	joined := ""
	for _, d := range Registry() {
		joined += d.Kind + " " + strings.Join(d.Variants, " ") + " "
	}
	for _, gimmick := range []string{"confetti", "trail", "glow", "shake", "magnifier", "bounce", "emoji"} {
		if strings.Contains(strings.ToLower(joined), gimmick) {
			t.Errorf("registry must not contain gimmick %q", gimmick)
		}
	}
	if len(RejectedEffects()) == 0 {
		t.Error("rejected effects must be documented")
	}
	// Priority library from the goal must be present.
	for _, need := range []string{"camera", "cursor", "click", "typing", "keyboard", "spotlight", "focus", "callout", "result", "hold"} {
		found := false
		for _, d := range Registry() {
			if d.Kind == need {
				found = true
			}
		}
		if !found {
			t.Errorf("registry missing priority effect %q", need)
		}
	}
}

func fxBeats() []BeatPlan {
	return []BeatPlan{
		{SceneID: "login", BeatID: "b1", Index: 0, Type: SceneTypingAction, ActionType: "fill", ActionLabel: "fill email"},
		{SceneID: "login", BeatID: "b2", Index: 1, Type: SceneSubmit, ActionType: "press", ActionLabel: "press Enter", NeedsConfirmation: true, ExpectedResult: "home screen"},
	}
}

func fxTimeline() *timeline.FinalTimeline {
	nb := &visual.BBox{X: 0.3, Y: 0.4, Width: 0.2, Height: 0.1}
	return &timeline.FinalTimeline{
		StoryboardHash: "test",
		Segments: []timeline.AVSegment{
			{SceneID: "login", BeatID: "b1", Kind: "action", Label: "fill", StartS: 0, DurS: 1.0, Zoom: 1.18, NormBBox: nb},
			{SceneID: "login", BeatID: "b2", Kind: "action", Label: "press Enter", StartS: 1.0, DurS: 1.0, Zoom: 1.0, NormBBox: nb},
		},
	}
}

func fxResolved() []ResolvedBeat {
	return []ResolvedBeat{
		{SceneID: "login", BeatID: "b1", Index: 0, Zoom: "medium", Cursor: "show", Click: "strong", ClickEffect: "ring", Typing: "natural", Keyboard: "shortcuts", Spotlight: "off", Focus: "outline", Callout: "auto", Result: "auto", Hold: "auto", Transition: "auto", Camera: "follow", CameraLock: "auto",
			Sources: map[string]Source{"zoom": SourceTutorial, "keyboard": SourceTutorial, "focus": SourceAction, "result": SourceDirector, "spotlight": SourceTutorial, "click": SourceTutorial, "camera": SourceDirector, "cursor": SourceDirector, "cursor_halo": SourceDirector, "click_effect": SourceTutorial, "typing": SourceDirector, "callout": SourceDirector, "hold": SourceDirector, "transition": SourceDirector, "camera_lock": SourceDirector}},
		{SceneID: "login", BeatID: "b2", Index: 1, Zoom: "medium", Cursor: "show", Click: "strong", ClickEffect: "ring", Typing: "natural", Keyboard: "shortcuts", Spotlight: "off", Focus: "outline", Callout: "auto", Result: "emphasize", Hold: "long", Transition: "auto", Camera: "follow", CameraLock: "auto",
			Sources: map[string]Source{"zoom": SourceTutorial, "keyboard": SourceTutorial, "focus": SourceAction, "result": SourceTutorial, "spotlight": SourceTutorial, "click": SourceTutorial, "camera": SourceDirector, "cursor": SourceDirector, "cursor_halo": SourceDirector, "click_effect": SourceTutorial, "typing": SourceDirector, "callout": SourceDirector, "hold": SourceTutorial, "transition": SourceDirector, "camera_lock": SourceDirector}},
	}
}

func TestPlanEffectsAnnotatesSegments(t *testing.T) {
	ft := fxTimeline()
	fx := PlanEffects(fxResolved(), fxBeats(), ft)
	// press Enter under keyboard=shortcuts -> overlay + label.
	if !ft.Segments[1].Keyboard || ft.Segments[1].KeyLabel != "ENTER" {
		t.Errorf("b2 must carry keyboard ENTER overlay, got %+v", ft.Segments[1])
	}
	// focus=outline on b1 -> outline flag.
	if !ft.Segments[0].Outline {
		t.Errorf("b1 must carry outline flag")
	}
	// result=emphasize on confirmation beat -> result flash.
	if !ft.Segments[1].ResultFlash {
		t.Errorf("b2 must carry result flash flag")
	}
	if len(fx.Collisions) != 0 {
		t.Errorf("single-beat effects must not collide: %v", fx.Collisions)
	}
}

func TestKeyboardCollisionStacks(t *testing.T) {
	evs := []EffectEvent{
		{Kind: "keyboard", SceneID: "s", BeatID: "b"},
		{Kind: "keyboard", SceneID: "s", BeatID: "b"},
	}
	if got := CheckEffectCollisions(evs); len(got) == 0 {
		t.Error("stacked keyboard pills on one beat must be flagged")
	}
	safe := keyboardSafeRect()
	if !safe.InsideViewport(0.01) {
		t.Error("keyboard safe area must sit inside the viewport")
	}
}

func TestInterventionBudget(t *testing.T) {
	evs := []EffectEvent{
		{Kind: "keyboard", SceneID: "s", BeatID: "b", Layer: LayerRender, Source: SourceDirector},
		{Kind: "focus", SceneID: "s", BeatID: "b", Layer: LayerRender, Source: SourceDirector},
		{Kind: "result", SceneID: "s", BeatID: "b", Layer: LayerRender, Source: SourceDirector},
		{Kind: "callout", SceneID: "s", BeatID: "b", Layer: LayerRender, Source: SourceDirector},
	}
	if got := CheckInterventionBudget(evs, nil); len(got) == 0 {
		t.Error("4 simultaneous render effects must trip the budget")
	}
}

func TestComplianceHonorsDirectives(t *testing.T) {
	ft := fxTimeline()
	res := fxResolved()
	fx := PlanEffects(res, fxBeats(), ft)
	cam := &CameraPlan{Decisions: []CameraDecision{
		{SceneID: "login", Zoom: 1.18}, {SceneID: "login", Zoom: 1.0},
	}}
	att := &AttentionPlan{Decisions: []AttentionDecision{
		{SceneID: "login", Spotlight: false},
	}}
	rep := ComplianceFor(res, cam, att, nil, fx)
	if rep.Requested == 0 {
		t.Fatal("tutorial-scoped directives must be detected")
	}
	if rep.Ignored != 0 {
		t.Errorf("all directives should be honored:\n%s", rep.Print())
	}
	// Zoom off violation: a zoomed scene under tutorial zoom=off fails.
	resOff := []ResolvedBeat{{
		SceneID: "login", BeatID: "b1", Zoom: "off",
		Sources: map[string]Source{"zoom": SourceTutorial},
	}}
	repOff := ComplianceFor(resOff, cam, att, nil, &EffectTimeline{})
	if repOff.Ignored == 0 {
		t.Error("zoom event under zoom=off must fail compliance")
	}
}

func TestKeyLabel(t *testing.T) {
	if got := keyLabelFor("press Enter"); got != "ENTER" {
		t.Errorf("want ENTER, got %q", got)
	}
	if got := keyLabelFor("press Control+k"); got != "CONTROL+K" {
		t.Errorf("want CONTROL+K, got %q", got)
	}
	if got := keyLabelFor(""); got != "KEY" {
		t.Errorf("empty label must fall back to KEY, got %q", got)
	}
}
