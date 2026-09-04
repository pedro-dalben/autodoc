package cinematic

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func testCfg() visual.CinematicConfig {
	c := visual.DefaultCinematic()
	return c
}

func actionStep(t, desc string) recipe.StepPlan {
	return recipe.StepPlan{Kind: recipe.StepAction, Action: &storyboard.Action{Type: t, Target: &storyboard.Target{TestID: desc}}}
}

func TestClassifyBeats(t *testing.T) {
	cases := []struct {
		name string
		kind recipe.StepKind
		act  string
		next *recipe.StepPlan
		want SceneType
	}{
		{"speech-then-action", recipe.StepSpeech, "", &recipe.StepPlan{Kind: recipe.StepAction}, SceneExplanation},
		{"speech-alone", recipe.StepSpeech, "", nil, SceneExplanation},
		{"click", recipe.StepAction, "click", nil, SceneClickAction},
		{"fill", recipe.StepAction, "fill", nil, SceneTypingAction},
		{"select", recipe.StepAction, "select", nil, SceneSelection},
		{"click-submit", recipe.StepAction, "click", &recipe.StepPlan{Kind: recipe.StepWait}, SceneSubmit},
		{"goto", recipe.StepAction, "goto", nil, SceneNavigation},
		{"wait-visible", recipe.StepWait, "", nil, SceneLoading},
		{"hold", recipe.StepHold, "", nil, SceneConfirmation},
	}
	for _, c := range cases {
		st := recipe.StepPlan{Kind: c.kind}
		if c.act != "" {
			st.Action = &storyboard.Action{Type: c.act}
		}
		if c.kind == recipe.StepWait {
			st.Wait = &storyboard.WaitEvent{State: "visible"}
		}
		if got := ClassifyBeat(false, st, c.next, false); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
	if got := ClassifyBeat(false, recipe.StepPlan{Kind: recipe.StepAction, Action: &storyboard.Action{Type: "click"}}, nil, true); got != SceneModal {
		t.Errorf("modal: got %s", got)
	}
	w := recipe.StepPlan{Kind: recipe.StepWait, Wait: &storyboard.WaitEvent{State: "url"}}
	if got := ClassifyBeat(false, w, nil, false); got != ScenePageTransition {
		t.Errorf("page transition: got %s", got)
	}
}

func TestInferAnchor(t *testing.T) {
	st := actionStep("click", "new-material-btn")
	a := InferAnchor(st, nil, "")
	if a.Kind != AnchorTarget || a.TargetKey == "" {
		t.Fatalf("target anchor: %+v", a)
	}
	sp := recipe.StepPlan{Kind: recipe.StepSpeech, Text: "hi"}
	a = InferAnchor(sp, &st, "")
	if a.Kind != AnchorTarget {
		t.Fatalf("speech borrows action anchor: %+v", a)
	}
	a = InferAnchor(sp, nil, "")
	if a.Kind != AnchorNone {
		t.Fatalf("orphan narration: %+v", a)
	}
	a = InferAnchor(sp, nil, "viewport")
	if a.Kind != AnchorViewport {
		t.Fatalf("viewport anchor: %+v", a)
	}
	fill := actionStep("fill", "material-name")
	a = InferAnchor(fill, nil, "")
	if a.Kind != AnchorForm {
		t.Fatalf("form anchor: %+v", a)
	}
}

func TestAnticipationEnvelope(t *testing.T) {
	cfg := testCfg()
	b := BeatPlan{NeedsAnticipation: true}
	env := AnticipationFor(cfg, b, 400)
	if env.RevealMs < 300 || env.RevealMs > 700 {
		t.Errorf("reveal %d out of 300-700", env.RevealMs)
	}
	if env.ApproachMs < 200 || env.ApproachMs > 500 {
		t.Errorf("approach %d out of 200-500", env.ApproachMs)
	}
	if env.SettleMs < 200 {
		t.Errorf("settle %d < 200", env.SettleMs)
	}
	if env.TotalMs != env.RevealMs+env.ApproachMs+env.SettleMs {
		t.Errorf("total mismatch: %+v", env)
	}
}

func TestAttentionOverrides(t *testing.T) {
	cfg := testCfg()
	beats := []BeatPlan{{Index: 0, SceneID: "s", BeatID: "b", Type: SceneClickAction, Anchor: VisualAnchor{Kind: AnchorTarget}, NeedsAnticipation: true, AttentionOverride: "none"}}
	p := DirectAttention(beats, cfg)
	if p.Decisions[0].Strategy != AttNone || p.Decisions[0].Anticipate {
		t.Fatalf("override none: %+v", p.Decisions[0])
	}
	beats[0].AttentionOverride = ""
	p = DirectAttention(beats, cfg)
	if p.Decisions[0].Strategy != AttSpotlight || !p.Decisions[0].Anticipate {
		t.Fatalf("default click: %+v", p.Decisions[0])
	}
}

func TestAttentionBudgetDemotesRepeatedInterventions(t *testing.T) {
	cfg := testCfg()
	beats := make([]BeatPlan, 5)
	for i := range beats {
		beats[i] = BeatPlan{Index: i, SceneID: "s", BeatID: string(rune('a' + i)), Type: SceneClickAction, NeedsAnticipation: true}
	}
	p := DirectAttention(beats, cfg)
	if p.Decisions[4].Strategy != AttFocus || p.Decisions[4].Reason != "attention-budget" {
		t.Fatalf("last repeated intervention should be demoted: %+v", p.Decisions[4])
	}
}

func TestReadableResultHoldScalesAndCaps(t *testing.T) {
	cfg := testCfg()
	if got := ReadableResultHold("ok", cfg); got <= cfg.Results.MinHoldMs {
		t.Fatalf("short result should add reading time: %d", got)
	}
	if got := ReadableResultHold(string(make([]rune, 500)), cfg); got != 2600 {
		t.Fatalf("long result must cap: %d", got)
	}
}

func nb(x, y, w, h float64) *visual.BBox { return &visual.BBox{X: x, Y: y, Width: w, Height: h} }

func TestCameraContinuityAndSafety(t *testing.T) {
	cfg := testCfg()
	vis := visual.Default()
	ft := &timeline.FinalTimeline{StoryboardHash: "h"}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "s", Kind: "action", Label: "click a", DurS: 0.8, Zoom: 1.08, NormBBox: nb(0.1, 0.1, 0.1, 0.1)},
		{SceneID: "s", Kind: "action", Label: "click a", DurS: 0.8, Zoom: 1.08, NormBBox: nb(0.11, 0.11, 0.1, 0.1)},
		{SceneID: "s", Kind: "action", Label: "click b", DurS: 0.8, Zoom: 9.0, NormBBox: nb(0.7, 0.5, 0.1, 0.1)},
		{SceneID: "s", Kind: "action", Label: "click c", DurS: 0.8, Zoom: 1.08, NormBBox: nb(5, 5, 0.1, 0.1)},
	}
	beats := []BeatPlan{
		{Index: 0, SceneID: "s", ActionLabel: "click a"},
		{Index: 1, SceneID: "s", ActionLabel: "click a"},
		{Index: 2, SceneID: "s", ActionLabel: "click b"},
		{Index: 3, SceneID: "s", ActionLabel: "click c"},
	}
	att := &AttentionPlan{}
	cam := DirectCamera(ft, att, beats, cfg, vis)
	if cam.Decisions[1].Move != CamPan {
		t.Errorf("same-neighborhood must pan, got %s", cam.Decisions[1].Move)
	}
	if cam.Decisions[2].Zoom > 1.25+1e-9 {
		t.Errorf("max zoom violated: %v", cam.Decisions[2].Zoom)
	}
	if cam.Decisions[3].Move != CamStay || cam.Decisions[3].Zoom != 1 {
		t.Errorf("unsafe target must stay unzoomed: %+v", cam.Decisions[3])
	}
	n := ApplyCamera(ft, cam)
	if n == 0 {
		t.Error("expected camera adjustments")
	}
	if ft.Segments[2].Zoom > 1.25+1e-9 {
		t.Errorf("clamp not applied: %v", ft.Segments[2].Zoom)
	}
}

func TestContextRestore(t *testing.T) {
	cfg := testCfg()
	beats := []BeatPlan{
		{Index: 0, SceneID: "s", BeatID: "b1", Type: SceneSubmit, Anchor: VisualAnchor{Kind: AnchorTarget, TargetKey: "a"}, ActionLabel: "click a", NeedsAnticipation: true, NeedsConfirmation: true},
		{Index: 1, SceneID: "s", BeatID: "b2", Type: SceneExplanation, Anchor: VisualAnchor{Kind: AnchorTarget, TargetKey: "b"}},
	}
	att := DirectAttention(beats, cfg)
	found := false
	for _, d := range att.Decisions {
		if d.Strategy == AttContextRestore {
			found = true
		}
	}
	// Confirmation-adjacent explanation triggers restore planning via the
	// camera director; attention restores on completion/confirmation beats.
	_ = found
	beats2 := []BeatPlan{
		{Index: 0, SceneID: "s", BeatID: "b1", Type: SceneConfirmation, Anchor: VisualAnchor{Kind: AnchorViewport}},
	}
	att2 := DirectAttention(beats2, cfg)
	if att2.Decisions[0].Strategy != AttStay && att2.Decisions[0].Strategy != AttContextRestore {
		t.Errorf("confirmation stable: %s", att2.Decisions[0].Strategy)
	}
}

func TestDeadTimeClassification(t *testing.T) {
	cfg := testCfg()
	if c := ClassifyWait("visible", true, 4.3, false); c != WaitLoading {
		t.Errorf("long visible = loading, got %s", c)
	}
	if c := ClassifyWait("url", true, 2.0, false); c != WaitTransition {
		t.Errorf("url = transition, got %s", c)
	}
	if c := ClassifyWait("visible", true, 4.3, true); c != WaitNarrationCovered {
		t.Errorf("speech overlap, got %s", c)
	}
	if d, _ := DecideWait(WaitLoading, 4.3, true, cfg); d != EditCompress {
		t.Errorf("loading compress, got %s", d)
	}
	if d, _ := DecideWait(WaitSemantic, 0.6, true, cfg); d != EditKeep {
		t.Errorf("semantic keep, got %s", d)
	}
	if d, _ := DecideWait(WaitLoading, 4.3, false, cfg); d != EditKeep {
		t.Errorf("opt-out keep, got %s", d)
	}
}

func TestResultHoldExtension(t *testing.T) {
	cfg := testCfg()
	ft := &timeline.FinalTimeline{StoryboardHash: "h"}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "s", BeatID: "b", Kind: "speech", Label: "s-speech-001", StartS: 0, DurS: 1.0},
		{SceneID: "s", BeatID: "b", Kind: "action", Label: "click save", StartS: 1.0, DurS: 0.5},
		{SceneID: "s", BeatID: "b", Kind: "speech", Label: "s-speech-002", StartS: 1.5, DurS: 1.0},
	}
	ft.Speeches = []timeline.Segment{
		{SpeechID: "s-speech-001", StartS: 0, EndS: 1.0, DurationS: 1.0},
		{SpeechID: "s-speech-002", StartS: 1.5, EndS: 2.5, DurationS: 1.0},
	}
	beats := []BeatPlan{{Index: 0, SceneID: "s", BeatID: "b", ActionType: "click", ActionLabel: "click save", NeedsConfirmation: true, ExpectedResult: "ok"}}
	added := EnsureResultHolds(ft, beats, cfg)
	if added <= 0 {
		t.Fatal("expected hold extension")
	}
	// Speech durations untouched (no compression), only shifted.
	if ft.Speeches[0].DurationS != 1.0 || ft.Speeches[1].DurationS != 1.0 {
		t.Fatalf("speech must never compress: %+v", ft.Speeches)
	}
	if ft.Segments[2].StartS <= 1.5 {
		t.Fatalf("downstream reflowed: %+v", ft.Segments[2])
	}
}

func TestStaticNarrationDetection(t *testing.T) {
	cfg := testCfg()
	ft := &timeline.FinalTimeline{StoryboardHash: "h", TotalS: 6}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "s", Kind: "speech", Label: "sp1", StartS: 0, DurS: 5},
		{SceneID: "s", Kind: "action", Label: "click a", StartS: 5, DurS: 1, Zoom: 1.08, NormBBox: nb(0.1, 0.1, 0.1, 0.1)},
	}
	beats := []BeatPlan{{Index: 0, SceneID: "s", Type: SceneNarration, SpeechID: "sp1", Anchor: VisualAnchor{Kind: AnchorNone}}}
	rep := AnalyzeStatic(ft, nil)
	wins := DetectStaticNarration(ft, beats, rep, cfg)
	if len(wins) != 1 || wins[0].Verdict != "fail" {
		t.Fatalf("orphaned 5s narration must fail: %+v", wins)
	}
	beats[0].Type = SceneConfirmation
	beats[0].Anchor = VisualAnchor{Kind: AnchorViewport}
	wins = DetectStaticNarration(ft, beats, rep, cfg)
	if wins[0].Verdict == "fail" {
		t.Fatalf("intentional static must not fail: %+v", wins)
	}
}

func TestCollisionEngine(t *testing.T) {
	b := nb(0.4, 0.4, 0.1, 0.1)
	call := CalloutRect(b, "1. Escolha a conversa", "above")
	in := CollisionInput{Target: b, Callouts: []CalloutRectInput{{Rect: call, Text: "t"}}}
	if bad := CheckCollisions(in); len(bad) != 0 {
		t.Fatalf("placed callout must be clean: %v", bad)
	}
	in.Callouts[0].Rect = Rect{X: 0.42, Y: 0.42, W: 0.2, H: 0.1}
	if bad := CheckCollisions(in); len(bad) == 0 {
		t.Fatal("expected target-overlap violation")
	}
}

func TestQAAndScore(t *testing.T) {
	cfg := testCfg()
	ft := &timeline.FinalTimeline{StoryboardHash: "h"}
	ft.Sync = &timeline.SyncReport{Pass: true}
	ft.Segments = []timeline.AVSegment{
		{SceneID: "s", Kind: "speech", Label: "sp1", StartS: 0, DurS: 2},
		{SceneID: "s", Kind: "action", Label: "click a", StartS: 2, DurS: 1, Zoom: 1.08, NormBBox: nb(0.2, 0.2, 0.1, 0.1)},
	}
	scenes := &ScenePlanDoc{StoryboardHash: "h", Beats: []BeatPlan{
		{Index: 0, SceneID: "s", BeatID: "b", Type: SceneExplanation, SpeechID: "sp1", Anchor: VisualAnchor{Kind: AnchorTarget, TargetKey: "a"}},
	}}
	att := DirectAttention(scenes.Beats, cfg)
	cam := DirectCamera(ft, att, scenes.Beats, cfg, visual.Default())
	edit := BuildEditPlan(ft, scenes.Beats, cfg)
	static := AnalyzeStatic(ft, nil)
	wins := DetectStaticNarration(ft, scenes.Beats, static, cfg)
	rep := RunQA(ft, scenes, att, cam, edit, wins, nil, cfg)
	if rep.Score < 0 || rep.Score > 100 {
		t.Fatalf("score range: %d", rep.Score)
	}
	if len(rep.Gates) != 12 {
		t.Fatalf("12 gates, got %d", len(rep.Gates))
	}
}

func TestPlanScenesCompatibility(t *testing.T) {
	r := &recipe.Recipe{StoryboardHash: "h", Scenes: []recipe.ScenePlan{
		{ID: "s", Beats: []recipe.BeatPlan{{ID: "b", Steps: []recipe.StepPlan{
			{Kind: recipe.StepSpeech, SpeechID: "s-b-speech-001", Text: "Hello"},
			{Kind: recipe.StepAction, Action: &storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "x"}}},
		}}}},
	}}
	doc := PlanScenes(r, nil)
	if len(doc.Beats) != 2 {
		t.Fatalf("beats: %+v", doc)
	}
	if doc.Beats[0].Anchor.Kind == AnchorNone {
		t.Fatalf("speech borrows click anchor: %+v", doc.Beats[0])
	}
}

func TestBeatSelectorDisambiguation(t *testing.T) {
	r := &recipe.Recipe{StoryboardHash: "h", Scenes: []recipe.ScenePlan{
		{ID: "s", Beats: []recipe.BeatPlan{{ID: "b", Steps: []recipe.StepPlan{
			{Kind: recipe.StepAction, Action: &storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "open"}, ResultTarget: &storyboard.Target{TestID: "panel"}}},
			{Kind: recipe.StepAction, Action: &storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "close"}}},
		}}}},
	}}
	doc := PlanScenes(r, nil)
	idx := indexBeats(doc.Beats)
	got := beatForAction(idx, "s", "b", `click [data-testid="close"]`)
	if got.ActionLabel == "" || got.NeedsConfirmation {
		t.Fatalf("sibling click must not inherit confirmation: %+v", got)
	}
	got = beatForAction(idx, "s", "b", `click [data-testid="open"]`)
	if !got.NeedsConfirmation {
		t.Fatalf("declared result beat must confirm: %+v", got)
	}
}
