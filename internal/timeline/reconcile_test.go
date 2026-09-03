package timeline_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func testRecipe() *recipe.Recipe {
	sb := &storyboard.Storyboard{Version: 2}
	sb.Meta.Title = "T"
	sb.Config.ViewportW, sb.Config.ViewportH = 1280, 720
	click := storyboard.Action{Type: "click", Target: &storyboard.Target{TestID: "btn"}}
	fill := storyboard.Action{Type: "fill", Target: &storyboard.Target{TestID: "name"}, Value: "abc"}
	wait := storyboard.WaitEvent{State: "timeout", TimeoutMs: 1500}
	hold := storyboard.HoldEvent{DurationMs: 500}
	sb.Scenes = []storyboard.Scene{{
		ID: "s1", Title: "S", URL: "/",
		Beats: []storyboard.Beat{{
			ID: "b1",
			Sequence: []storyboard.Event{
				{Speech: &storyboard.SpeechEvent{Text: "Speech A"}},
				{Action: &click},
				{Wait: &wait},
				{Speech: &storyboard.SpeechEvent{Text: "Speech B"}},
				{Action: &fill},
				{Hold: &hold},
			},
		}},
	}}
	return recipe.Compile(sb, "m", "v", "pt-BR", 1.0)
}

func plannedWith(durA, durB float64) *timeline.Timeline {
	r := testRecipe()
	tl, err := timeline.Build(r, func(id string) (float64, bool, string) {
		if id == "s1-b1-speech-001" {
			return durA, false, "/a.wav"
		}
		return durB, false, "/b.wav"
	}, 600)
	if err != nil {
		panic(err)
	}
	return tl
}

func ve(typ string, start, end int64) *visual.VisualEvent {
	return &visual.VisualEvent{Type: typ, StartedAtMs: start, EndedAtMs: end, DurationMs: end - start}
}

func TestReconcilePacedCapture(t *testing.T) {
	planned := plannedWith(2.0, 4.0)
	// Paced capture: speech A [1000,3000], click [3250,4200], wait [4200,5700],
	// speech B [6150,10150], fill [10400,11200], hold [11200,11700].
	events := map[string][]timeline.ActualEvent{
		"s1": {
			{Kind: "session", AtMs: 0},
			{Kind: "speech_start", Label: "s1-b1-speech-001", AtMs: 1000, Visual: ve("speech", 1000, 3000)},
			{Kind: "speech_end", Label: "s1-b1-speech-001", AtMs: 3000, Visual: ve("speech", 1000, 3000)},
			{Kind: "visual", Label: "click", AtMs: 4200, Visual: &visual.VisualEvent{Type: "interaction", Interaction: "click", StartedAtMs: 3250, FocusAtMs: 3600, ActionAtMs: 3950, EndedAtMs: 4200, Zoom: 1.08}},
			{Kind: "wait_span", Label: "timeout", AtMs: 5700, Visual: ve("wait", 4200, 5700)},
			{Kind: "speech_start", Label: "s1-b1-speech-002", AtMs: 6150, Visual: ve("speech", 6150, 10150)},
			{Kind: "speech_end", Label: "s1-b1-speech-002", AtMs: 10150, Visual: ve("speech", 6150, 10150)},
			{Kind: "visual", Label: "fill", AtMs: 11200, Visual: &visual.VisualEvent{Type: "interaction", Interaction: "type", StartedAtMs: 10400, FocusAtMs: 10600, ActionAtMs: 11000, EndedAtMs: 11200, Zoom: 1.15, TypingChars: 3, Progressive: true}},
			{Kind: "hold", Label: "500ms", AtMs: 11700, Visual: ve("hold", 11200, 11700)},
		},
	}
	ft := timeline.Reconcile(planned, testRecipe(), events, map[string]float64{"s1": 12.0}, timeline.DefaultReconcileOpts())
	if len(ft.Segments) != 6 {
		t.Fatalf("expected 6 segments, got %d", len(ft.Segments))
	}
	// Speech windows intact.
	if ft.Speeches[0].DurationS != 2.0 || ft.Speeches[1].DurationS != 4.0 {
		t.Fatalf("speech durations altered: %+v", ft.Speeches)
	}
	// Speech A video window ≈ 2s starting at 1.0.
	if s := ft.Segments[0]; s.Kind != "speech" || diff(s.VideoStartS, 1.0) > 0.01 || diff(s.VideoEndS-s.VideoStartS, 2.0) > 0.01 {
		t.Fatalf("speech A window wrong: %+v", s)
	}
	// Action cue preserved with zoom.
	if s := ft.Segments[1]; s.Kind != "action" || s.Zoom != 1.08 {
		t.Fatalf("action cue wrong: %+v", s)
	}
	// Wait 1.5s compressible but <= 1.5s threshold → intact.
	if s := ft.Segments[2]; s.Kind != "wait" || s.Compressed || diff(s.DurS, 1.5) > 0.01 {
		t.Fatalf("1.5s wait must stay intact: %+v", s)
	}
	// Total = 2 + 0.95 + 1.5 + 4 + 0.8 + 0.5.
	if want := 2.0 + 0.95 + 1.5 + 4.0 + 0.8 + 0.5; diff(ft.TotalS, want) > 0.01 {
		t.Fatalf("total %.3f want %.3f", ft.TotalS, want)
	}
	if ft.Sync == nil || !ft.Sync.Pass {
		t.Fatalf("paced capture must PASS sync: %+v", ft.Sync)
	}
}

func TestLongWaitCompressedNeverSpeech(t *testing.T) {
	planned := plannedWith(2.0, 4.0)
	events := map[string][]timeline.ActualEvent{
		"s1": {
			{Kind: "session", AtMs: 0},
			{Kind: "speech_start", Label: "s1-b1-speech-001", AtMs: 500, Visual: ve("speech", 500, 2500)},
			{Kind: "speech_end", Label: "s1-b1-speech-001", AtMs: 2500, Visual: ve("speech", 500, 2500)},
			{Kind: "visual", Label: "click", AtMs: 3400, Visual: &visual.VisualEvent{Type: "interaction", StartedAtMs: 2750, ActionAtMs: 3150, EndedAtMs: 3400}},
			{Kind: "wait_span", Label: "timeout", AtMs: 11400, Visual: ve("wait", 3400, 11400)},
			{Kind: "speech_start", Label: "s1-b1-speech-002", AtMs: 11850, Visual: ve("speech", 11850, 15850)},
			{Kind: "speech_end", Label: "s1-b1-speech-002", AtMs: 15850, Visual: ve("speech", 11850, 15850)},
			{Kind: "visual", Label: "fill", AtMs: 16650, Visual: &visual.VisualEvent{Type: "interaction", StartedAtMs: 16100, ActionAtMs: 16400, EndedAtMs: 16650}},
			{Kind: "hold", Label: "500ms", AtMs: 17150, Visual: ve("hold", 16650, 17150)},
		},
	}
	ft := timeline.Reconcile(planned, testRecipe(), events, map[string]float64{"s1": 17.5}, timeline.DefaultReconcileOpts())
	var wait *timeline.AVSegment
	for i := range ft.Segments {
		if ft.Segments[i].Kind == "wait" {
			wait = &ft.Segments[i]
		}
	}
	if wait == nil || !wait.Compressed {
		t.Fatalf("8s loading wait must compress: %+v", wait)
	}
	if wait.Speed < 2 || wait.Speed > 8 {
		t.Fatalf("compression speed out of sane range: %.2f", wait.Speed)
	}
	if wait.DurS > 2.0 || wait.DurS < 0.7 {
		t.Fatalf("compressed wait outside 0.7..2.0s: %.2f", wait.DurS)
	}
	// Speech B intact at 4s.
	if ft.Speeches[1].DurationS != 4.0 {
		t.Fatal("speech must never compress")
	}
}

func TestOverlapFailsSync(t *testing.T) {
	planned := plannedWith(2.0, 4.0)
	events := map[string][]timeline.ActualEvent{
		"s1": {
			{Kind: "session", AtMs: 0},
			{Kind: "speech_start", Label: "s1-b1-speech-001", AtMs: 500, Visual: ve("speech", 500, 2500)},
			{Kind: "speech_end", Label: "s1-b1-speech-001", AtMs: 2500, Visual: ve("speech", 500, 2500)},
			// Action overlapping speech A (buggy recorder): must FAIL.
			{Kind: "visual", Label: "click", AtMs: 2000, Visual: &visual.VisualEvent{Type: "interaction", StartedAtMs: 1500, ActionAtMs: 1800, EndedAtMs: 2000}},
			{Kind: "wait_span", Label: "timeout", AtMs: 3500, Visual: ve("wait", 2000, 3500)},
			{Kind: "speech_start", Label: "s1-b1-speech-002", AtMs: 3950, Visual: ve("speech", 3950, 7950)},
			{Kind: "speech_end", Label: "s1-b1-speech-002", AtMs: 7950, Visual: ve("speech", 3950, 7950)},
			{Kind: "visual", Label: "fill", AtMs: 8750, Visual: &visual.VisualEvent{Type: "interaction", StartedAtMs: 8200, ActionAtMs: 8500, EndedAtMs: 8750}},
			{Kind: "hold", Label: "500ms", AtMs: 9250, Visual: ve("hold", 8750, 9250)},
		},
	}
	ft := timeline.Reconcile(planned, testRecipe(), events, map[string]float64{"s1": 9.5}, timeline.DefaultReconcileOpts())
	if ft.Sync.Pass {
		t.Fatalf("action over narration must FAIL sync: %+v", ft.Sync)
	}
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
