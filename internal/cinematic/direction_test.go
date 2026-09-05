package cinematic

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func dirPtr(d visual.DirectionConfig) *visual.DirectionConfig { return &d }

func testBeats() []BeatPlan {
	return []BeatPlan{
		{SceneID: "login", BeatID: "b1", Index: 0, Type: SceneTypingAction, ActionType: "fill", ActionLabel: "fill email"},
		{SceneID: "login", BeatID: "b2", Index: 1, Type: SceneTypingAction, ActionType: "fill", ActionLabel: "fill password"},
		{SceneID: "home", BeatID: "b1", Index: 2, Type: SceneClickAction, ActionType: "click", ActionLabel: "click save"},
	}
}

func TestResolvePrecedence(t *testing.T) {
	sb := &storyboard.Storyboard{
		Direction: dirPtr(visual.DirectionConfig{Zoom: "off"}),
		Scenes: []storyboard.Scene{
			{ID: "login", Direction: dirPtr(visual.DirectionConfig{Zoom: "strong"})},
			{ID: "home"},
		},
	}
	res := Resolve(sb, testBeats())
	if len(res) != 3 {
		t.Fatalf("want 3 resolved beats, got %d", len(res))
	}
	// Scene override beats tutorial override.
	if res[0].Zoom != "strong" || res[0].Sources["zoom"] != SourceScene {
		t.Errorf("login beat: want zoom=strong[user:scene], got %s[%s]", res[0].Zoom, res[0].Sources["zoom"])
	}
	// Tutorial override applies where no scene override exists.
	if res[2].Zoom != "off" || res[2].Sources["zoom"] != SourceTutorial {
		t.Errorf("home beat: want zoom=off[user:tutorial], got %s[%s]", res[2].Zoom, res[2].Sources["zoom"])
	}
	// Unspecified controls stay auto under the director.
	if res[0].Cursor != "auto" || res[0].Sources["cursor"] != SourceDirector {
		t.Errorf("cursor should stay auto[director], got %s[%s]", res[0].Cursor, res[0].Sources["cursor"])
	}
}

func TestResolveActionBeatsTutorial(t *testing.T) {
	sb := &storyboard.Storyboard{
		Direction: dirPtr(visual.DirectionConfig{Zoom: "off"}),
		Scenes: []storyboard.Scene{
			{ID: "login", Beats: []storyboard.Beat{{ID: "b2", Sequence: []storyboard.Event{
				{Action: &storyboard.Action{Type: "fill", Direction: dirPtr(visual.DirectionConfig{Zoom: "strong"})}},
			}}}},
			{ID: "home"},
		},
	}
	res := Resolve(sb, testBeats())
	// Action override wins for the password beat only.
	if res[1].Zoom != "strong" || res[1].Sources["zoom"] != SourceAction {
		t.Errorf("password beat: want zoom=strong[user:action], got %s[%s]", res[1].Zoom, res[1].Sources["zoom"])
	}
	if res[0].Zoom != "off" {
		t.Errorf("sibling beat must keep tutorial zoom=off, got %s", res[0].Zoom)
	}
}

func TestResolveBackwardCompatible(t *testing.T) {
	res := Resolve(nil, testBeats())
	for _, r := range res {
		if r.Zoom != "auto" || r.Cursor != "auto" || r.Spotlight != "auto" {
			t.Errorf("nil storyboard must resolve all-auto, got %+v", r)
		}
		if r.UserDirected() {
			t.Errorf("nil storyboard must not be user-directed")
		}
	}
}

func TestCompileNL(t *testing.T) {
	cases := []struct {
		name string
		text string
		want visual.DirectionConfig
	}{
		{"plain request stays auto", "Crie um tutorial do login.", visual.DirectionConfig{}},
		{"no zoom", "Não use zoom nesse vídeo.", visual.DirectionConfig{Zoom: "off"}},
		{"strong zoom", "Use bastante zoom no formulário.", visual.DirectionConfig{Zoom: "strong"}},
		{"strong zoom EN", "Use strong zoom while editing the profile.", visual.DirectionConfig{Zoom: "strong"}},
		{"subtle zoom", "Use um zoom discreto.", visual.DirectionConfig{Zoom: "subtle"}},
		{"hide cursor", "Não mostre o cursor.", visual.DirectionConfig{Cursor: "hide"}},
		{"show cursor", "Mostre o cursor.", visual.DirectionConfig{Cursor: "show"}},
		{"click emphasis", "Deixe os cliques bem visíveis.", visual.DirectionConfig{Click: "strong"}},
		{"keyboard", "Mostre Enter quando ele for pressionado.", visual.DirectionConfig{Keyboard: "shortcuts"}},
		{"keyboard EN", "Show keyboard shortcuts.", visual.DirectionConfig{Keyboard: "shortcuts"}},
		{"no spotlight keeps zoom", "Sem spotlight, mas mantenha zoom.", visual.DirectionConfig{Spotlight: "off"}},
		{"callout", "Mostre um callout explicando esse botão.", visual.DirectionConfig{Callout: "on"}},
		{"calm", "Quero esse vídeo mais calmo e sem muitos movimentos.", visual.DirectionConfig{Preset: "minimal", Camera: "static"}},
		{"dynamic", "Faça um tutorial mais dinâmico.", visual.DirectionConfig{Preset: "dynamic"}},
		{"hold", "Segure o resultado mais tempo para dar tempo de ler.", visual.DirectionConfig{Hold: "long"}},
		{"outline+result", "Destaque o botão de entrar.", visual.DirectionConfig{Focus: "outline", Result: "emphasize"}},
		{"static", "Use câmera fixa.", visual.DirectionConfig{Camera: "static"}},
		{"lock", "Mantenha a câmera focada no formulário até terminar.", visual.DirectionConfig{Camera: "focus", CameraLock: "on", Zoom: "strong"}},
	}
	for _, c := range cases {
		got := CompileNL(c.text)
		want := c.want
		if got.Zoom != want.Zoom || got.Cursor != want.Cursor || got.Click != want.Click ||
			got.Keyboard != want.Keyboard || got.Spotlight != want.Spotlight ||
			got.Callout != want.Callout || got.Preset != want.Preset ||
			got.Hold != want.Hold || got.Focus != want.Focus ||
			got.Result != want.Result || got.Camera != want.Camera ||
			got.CameraLock != want.CameraLock {
			t.Errorf("%s: %q -> %+v, want %+v", c.name, c.text, got, want)
		}
	}
}

func TestDirectionValidation(t *testing.T) {
	bad := visual.DirectionConfig{Cursor: "hide", CursorHalo: "on"}
	if len(bad.Validate()) == 0 {
		t.Error("cursor=hide + halo=on must fail validation")
	}
	bad2 := visual.DirectionConfig{Camera: "static", Zoom: "strong"}
	if len(bad2.Validate()) == 0 {
		t.Error("camera=static + zoom=strong must fail validation")
	}
	bad3 := visual.DirectionConfig{Click: "off", ClickEffect: "ring"}
	if len(bad3.Validate()) == 0 {
		t.Error("click=off + effect=ring must fail validation")
	}
	good := visual.DirectionConfig{Zoom: "off", Spotlight: "off", Keyboard: "shortcuts"}
	if len(good.Validate()) != 0 {
		t.Errorf("valid direction must pass: %v", good.Validate())
	}
	// tutorial off + scoped strong is NOT a conflict (precedence resolves it).
	sb := &storyboard.Storyboard{
		Direction: dirPtr(visual.DirectionConfig{Zoom: "off"}),
		Scenes:    []storyboard.Scene{{ID: "login", Direction: dirPtr(visual.DirectionConfig{Zoom: "strong"})}},
	}
	if errs := sb.Direction.Validate(); len(errs) != 0 {
		t.Errorf("scoped override must validate: %v", errs)
	}
}

func TestZoomSafetyClamp(t *testing.T) {
	rb := &ResolvedBeat{Zoom: "extreme"}
	if got := ZoomScaleFor(rb, 1.25); got != 1.25 {
		t.Errorf("extreme must clamp to MaxZoom 1.25, got %v", got)
	}
	rb2 := &ResolvedBeat{Zoom: "strong", Camera: "static"}
	if got := ZoomScaleFor(rb2, 1.25); got != 1.0 {
		t.Errorf("static camera must pin 1.0, got %v", got)
	}
	if got := ZoomScaleFor(nil, 1.25); got != 0 {
		t.Errorf("nil beat = director decides (0), got %v", got)
	}
}

func TestInvalidationScope(t *testing.T) {
	prev := visual.DirectionConfig{Zoom: "medium"}
	next := visual.DirectionConfig{Zoom: "strong"}
	if visual.NeedsRetake(prev, next) {
		t.Error("zoom-only change must NOT need a browser retake")
	}
	next.Typing = "slow"
	if !visual.NeedsRetake(prev, next) {
		t.Error("typing change must need a browser retake")
	}
	render, record := visual.InvalidationScope(visual.DirectionConfig{Zoom: "strong", Cursor: "hide", Keyboard: "all"})
	if len(render) == 0 || len(record) == 0 {
		t.Errorf("scope must split render/record, got %v / %v", render, record)
	}
}
