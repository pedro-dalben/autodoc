package timeline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/pedro-dalben/autodoc/internal/capture"
	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

type ActualEvent struct {
	Kind   string
	Label  string
	AtMs   int64
	Visual *visual.VisualEvent
}

func ParseActualEvents(records []capture.EventRecord) []ActualEvent {
	out := make([]ActualEvent, 0, len(records))
	for _, r := range records {
		ae := ActualEvent{Kind: r.Kind, Label: r.Label, AtMs: r.AtMs}
		if r.Note != "" {
			var ve visual.VisualEvent
			if err := json.Unmarshal([]byte(r.Note), &ve); err == nil && ve.Type != "" {
				ae.Visual = &ve
			}
		}
		out = append(out, ae)
	}
	return out
}

type AVSegment struct {
	SceneID      string       `json:"scene_id"`
	BeatID       string       `json:"beat_id,omitempty"`
	Kind         string       `json:"kind"`
	Label        string       `json:"label"`
	StartS       float64      `json:"start_s"`
	DurS         float64      `json:"dur_s"`
	VideoStartS  float64      `json:"video_start_s"`
	VideoEndS    float64      `json:"video_end_s"`
	Speed        float64      `json:"speed"`
	ActionAtS    float64      `json:"action_at_s,omitempty"`
	WavPath      string       `json:"wav_path,omitempty"`
	Zoom         float64      `json:"zoom,omitempty"`
	NormBBox     *visual.BBox `json:"norm_bbox,omitempty"`
	Keyboard     bool         `json:"keyboard,omitempty"`
	Compressed   bool         `json:"compressed,omitempty"`
	Estimated    bool         `json:"estimated,omitempty"`
	Compressible bool         `json:"compressible"`
}

type SyncItem struct {
	Scope       string  `json:"scope"`
	Kind        string  `json:"kind"`
	DriftMs     float64 `json:"drift_ms"`
	ToleranceMs int     `json:"tolerance_ms"`
	Pass        bool    `json:"pass"`
	Detail      string  `json:"detail,omitempty"`
}

type SyncReport struct {
	Items      []SyncItem `json:"items"`
	MaxDriftMs float64    `json:"max_drift_ms"`
	Pass       bool       `json:"pass"`
}

type FinalTimeline struct {
	StoryboardHash string             `json:"storyboard_hash"`
	TotalS         float64            `json:"total_s"`
	AudioS         float64            `json:"audio_s"`
	Segments       []AVSegment        `json:"segments"`
	Speeches       []Segment          `json:"speeches"`
	SceneClips     []SceneClip        `json:"scene_clips"`
	RawDurationS   map[string]float64 `json:"raw_duration_s"`
	Sync           *SyncReport        `json:"sync"`
}

type ReconcileOpts struct {
	SpeechTolMs int
	ActionTolMs int
	FinalTolMs  int
	// MaxWaitOutS caps a compressed loading wait in the final video.
	MaxWaitOutS float64
	// MinWaitOutS floors a compressed wait so the cut never feels abrupt.
	MinWaitOutS float64
	// WaitSpeed caps the fast-forward factor for compressible waits.
	WaitSpeed float64
}

func DefaultReconcileOpts() ReconcileOpts {
	d := visual.Default()
	return ReconcileOpts{
		SpeechTolMs: d.SyncTol.SpeechBoundaryMs,
		ActionTolMs: d.SyncTol.ActionCueMs,
		FinalTolMs:  d.SyncTol.FinalAVMs,
		MaxWaitOutS: 2.0,
		MinWaitOutS: 0.7,
		WaitSpeed:   8,
	}
}

func Reconcile(planned *Timeline, rec *recipe.Recipe, sceneEvents map[string][]ActualEvent, rawDur map[string]float64, opts ReconcileOpts) *FinalTimeline {
	if opts.SpeechTolMs == 0 {
		opts = DefaultReconcileOpts()
	}
	ft := &FinalTimeline{
		StoryboardHash: planned.StoryboardHash,
		RawDurationS:   rawDur,
		Sync:           &SyncReport{Pass: true},
	}
	fail := func(scope, kind string, drift float64, tol int, detail string) {
		pass := math.Abs(drift) <= float64(tol)
		if !pass {
			ft.Sync.Pass = false
		}
		if math.Abs(drift) > math.Abs(ft.Sync.MaxDriftMs) {
			ft.Sync.MaxDriftMs = drift
		}
		ft.Sync.Items = append(ft.Sync.Items, SyncItem{Scope: scope, Kind: kind, DriftMs: drift, ToleranceMs: tol, Pass: pass, Detail: detail})
	}
	out := 0.0
	sceneOut := map[string][2]float64{}
	segByID := map[string]Segment{}
	for _, s := range planned.Segments {
		segByID[s.SpeechID] = s
	}
	for _, sc := range rec.Scenes {
		clip := planned.clipForScene(sc.ID)
		sceneStart := out
		events := sceneEvents[sc.ID]
		t0 := videoZero(events)
		byType := map[string][]ActualEvent{}
		var waits []ActualEvent
		var holdWins [][2]float64
		var pendingHold float64
		havePendingHold := false
		var navTimes []float64
		var speechVideo [][2]float64
		speechWin := map[string][2]float64{}
		for _, e := range events {
			v := float64(e.AtMs-t0) / 1000.0
			switch e.Kind {
			case "visual":
				if e.Visual != nil {
					byType[e.Visual.Interaction] = append(byType[e.Visual.Interaction], e)
				}
			case "wait_span":
				waits = append(waits, e)
			case "hold_start":
				pendingHold = v
				havePendingHold = true
			case "hold_end":
				if havePendingHold {
					holdWins = append(holdWins, [2]float64{pendingHold, v})
					havePendingHold = false
				} else if e.Visual != nil && e.Visual.DurationMs > 0 {
					d := float64(e.Visual.DurationMs) / 1000.0
					holdWins = append(holdWins, [2]float64{v - d, v})
				}
			case "goto":
				navTimes = append(navTimes, v)
			case "speech_start":
				if e.Visual != nil {
					speechWin[e.Label] = [2]float64{v, v + float64(e.Visual.DurationMs)/1000.0}
				}
			case "speech_end":
				if e.Visual != nil {
					if w, ok := speechWin[e.Label]; ok {
						speechWin[e.Label] = [2]float64{w[0], v}
					} else {
						d := float64(e.Visual.DurationMs) / 1000.0
						speechWin[e.Label] = [2]float64{v - d, v}
					}
				}
			}
		}
		wi, hi, ni := 0, 0, 0
		typeCursor := map[string]int{}
		sceneSegs := plannedSegsFromRecipe(&sc, segByID)
		waitStarts := make([]float64, 0, len(waits))
		for _, we := range waits {
			ws, _ := spanWindow(we, t0)
			waitStarts = append(waitStarts, ws)
		}
		for stepIdx, ps := range sceneSegs {
			switch ps.kind {
			case "speech":
				w, ok := speechWin[ps.speechID]
				est := !ok
				if !ok {
					w = [2]float64{ps.start, ps.end}
				}
				winDur := w[1] - w[0]
				if winDur <= 0 {
					winDur = ps.dur
					w[1] = w[0] + winDur
				}
				speed := winDur / ps.dur
				if speed < 0.95 || speed > 1.05 {
					speed = 1
				}
				ft.Segments = append(ft.Segments, AVSegment{
					SceneID: sc.ID, BeatID: ps.beat, Kind: "speech", Label: ps.speechID,
					StartS: out, DurS: ps.dur, VideoStartS: w[0], VideoEndS: w[1],
					Speed: speed, WavPath: ps.wav, Estimated: est,
				})
				ft.Speeches = append(ft.Speeches, Segment{SpeechID: ps.speechID, Text: ps.text, StartS: out, EndS: out + ps.dur, DurationS: ps.dur, WavPath: ps.wav})
				speechVideo = append(speechVideo, [2]float64{w[0], w[1]})
				fail(sc.ID+"/"+ps.speechID, "speech", (winDur-ps.dur)*1000, opts.SpeechTolMs,
					fmt.Sprintf("wav %.2fs captured %.2fs", ps.dur, winDur))
				out += ps.dur
				ft.AudioS += ps.dur
			case "action":
				var vw0, vw1 float64
				est := true
				keyboardSeg := false
				var zoom float64 = 1
				var nb *visual.BBox
				label := ps.label
				actionAt := -1.0
				if ps.actionType == "goto" && ni < len(navTimes) {
					// Scene navigation claims its own window; never consumes
					// an interaction event. Trimmed at the next step so the
					// following wait keeps its own footage.
					vw0 = navTimes[ni]
					ni++
					vw1 = nextEventAfter(events, t0, vw0)
					if stepIdx+1 < len(sceneSegs) && sceneSegs[stepIdx+1].kind == "wait" && wi < len(waitStarts) && waitStarts[wi] > vw0+0.05 {
						vw1 = waitStarts[wi]
					}
					if vw1 <= vw0+0.05 {
						vw1 = vw0 + 0.5
					}
					est = false
					actionAt = vw0
				} else if q := byType[ps.actionType]; typeCursor[ps.actionType] < len(q) {
					e := q[typeCursor[ps.actionType]]
					typeCursor[ps.actionType]++
					vw0, vw1, zoom, nb, label, est = interactionWindow(e, t0)
					actionAt = actionAtS(e, t0)
					if e.Visual != nil && e.Visual.Keyboard {
						keyboardSeg = true
					}
				} else if ps.actionType == "goto" || ps.actionType == "expect" || ps.actionType == "screenshot" || ps.actionType == "scroll" || ps.actionType == "reload" || ps.actionType == "goback" {
					vw0 = out - sceneStart + videoOffsetGuess(events, t0)
					vw1 = vw0 + 0.5
					est = true
				} else {
					vw0 = out - sceneStart + videoOffsetGuess(events, t0)
					vw1 = vw0 + 0.8
				}
				dur := vw1 - vw0
				if dur <= 0.05 {
					dur = 0.8
					vw1 = vw0 + dur
				}
				if actionAt < 0 {
					actionAt = vw0 + dur/2
				}
				ft.Segments = append(ft.Segments, AVSegment{
					SceneID: sc.ID, BeatID: ps.beat, Kind: "action", Label: label,
					StartS: out, DurS: dur, VideoStartS: vw0, VideoEndS: vw1,
					Speed: 1, Zoom: zoom, NormBBox: nb, Estimated: est, ActionAtS: actionAt,
					Keyboard: keyboardSeg,
				})
				if !est {
					overlap := rangeOverlap(vw0, vw1, speechVideo)
					fail(sc.ID+"/"+label, "action", overlap*1000, opts.ActionTolMs,
						"action video overlapping narration (>0 = click over voice)")
				}
				out += dur
			case "wait":
				var vw0, vw1 float64
				est := true
				if wi < len(waits) {
					vw0, vw1 = spanWindow(waits[wi], t0)
					wi++
					est = false
				} else {
					vw0 = out - sceneStart + videoOffsetGuess(events, t0)
					vw1 = vw0 + ps.dur
				}
				actual := vw1 - vw0
				if actual <= 0 {
					actual = ps.dur
					vw1 = vw0 + actual
				}
				outDur, speed, compressed := waitOutput(actual, ps.dur, ps.compressible, opts)
				ft.Segments = append(ft.Segments, AVSegment{
					SceneID: sc.ID, BeatID: ps.beat, Kind: "wait", Label: ps.label,
					StartS: out, DurS: outDur, VideoStartS: vw0, VideoEndS: vw1,
					Speed: speed, Compressed: compressed, Estimated: est, Compressible: ps.compressible,
				})
				if compressed {
					fail(sc.ID+"/"+ps.label, "wait", (actual-outDur)*1000, 8000,
						fmt.Sprintf("compressed %.2fs raw to %.2fs final (%.1fx)", actual, outDur, speed))
					ft.Sync.Items[len(ft.Sync.Items)-1].Pass = speed <= opts.WaitSpeed
					if speed > opts.WaitSpeed {
						ft.Sync.Pass = false
					}
				}
				out += outDur
			case "hold":
				var vw0, vw1 float64
				est := true
				if hi < len(holdWins) {
					vw0, vw1 = holdWins[hi][0], holdWins[hi][1]
					hi++
					est = false
				} else {
					vw0 = out - sceneStart + videoOffsetGuess(events, t0)
					vw1 = vw0 + ps.dur
				}
				dur := vw1 - vw0
				if dur <= 0 {
					dur = ps.dur
				}
				ft.Segments = append(ft.Segments, AVSegment{
					SceneID: sc.ID, BeatID: ps.beat, Kind: "hold", Label: ps.label,
					StartS: out, DurS: ps.dur, VideoStartS: vw0, VideoEndS: vw1,
					Speed: dur / ps.dur, Estimated: est,
				})
				out += ps.dur
			}
		}
		sceneOut[sc.ID] = [2]float64{sceneStart, out}
		_ = clip
	}
	for _, scn := range rec.Scenes {
		if w, ok := sceneOut[scn.ID]; ok {
			ft.SceneClips = append(ft.SceneClips, SceneClip{SceneID: scn.ID, StartS: w[0], EndS: w[1], Duration: w[1] - w[0]})
		}
	}
	ft.TotalS = out
	estimated := 0
	for _, s := range ft.Segments {
		if s.Estimated {
			estimated++
		}
	}
	fail("final", "coverage", float64(estimated), 0,
		"segments without capture evidence (re-record for exact sync)")
	sort.Slice(ft.Segments, func(i, j int) bool { return ft.Segments[i].StartS < ft.Segments[j].StartS })
	return ft
}

type plannedSeg struct {
	kind         string
	beat         string
	label        string
	actionType   string
	speechID     string
	text         string
	wav          string
	start        float64
	end          float64
	dur          float64
	compressible bool
}

func (t *Timeline) clipForScene(sceneID string) SceneClip {
	for _, c := range t.SceneClips {
		if c.SceneID == sceneID {
			return c
		}
	}
	return SceneClip{SceneID: sceneID}
}

func plannedSegsFromRecipe(sc *recipe.ScenePlan, segByID map[string]Segment) []plannedSeg {
	var out []plannedSeg
	for _, b := range sc.Beats {
		for _, st := range b.Steps {
			switch st.Kind {
			case recipe.StepSpeech:
				if s, ok := segByID[st.SpeechID]; ok {
					out = append(out, plannedSeg{kind: "speech", beat: b.ID, label: st.SpeechID, speechID: s.SpeechID, text: s.Text, wav: s.WavPath, start: s.StartS, end: s.EndS, dur: s.DurationS})
				}
			case recipe.StepAction:
				label := st.Action.Type
				if st.Action.Target != nil {
					label += " " + st.Action.Target.Describe()
				}
				out = append(out, plannedSeg{kind: "action", beat: b.ID, label: label, actionType: st.Action.Type, start: -1})
			case recipe.StepWait:
				dur := float64(st.Wait.SettleMs) / 1000.0
				if st.Wait.SettleMs == 0 {
					dur = 0.6
				}
				out = append(out, plannedSeg{kind: "wait", beat: b.ID, label: st.Wait.State, start: -1, dur: dur, compressible: st.Wait.IsCompressible()})
			case recipe.StepHold:
				out = append(out, plannedSeg{kind: "hold", beat: b.ID, label: fmt.Sprintf("%dms", st.HoldMs), start: -1, dur: float64(st.HoldMs) / 1000.0})
			}
		}
	}
	return out
}

func nextEventAfter(events []ActualEvent, t0 int64, after float64) float64 {
	best := after + 30
	for _, e := range events {
		v := float64(e.AtMs-t0) / 1000.0
		if v > after+0.01 && v < best {
			best = v
		}
	}
	return best
}

func videoZero(events []ActualEvent) int64 {
	if len(events) == 0 {
		return 0
	}
	t0 := events[0].AtMs
	for _, e := range events {
		if e.Kind == "session" && e.AtMs < t0 {
			t0 = e.AtMs
		}
	}
	return t0
}

func videoOffsetGuess(events []ActualEvent, t0 int64) float64 {
	if len(events) == 0 {
		return 0
	}
	last := float64(events[len(events)-1].AtMs-t0) / 1000.0
	if last < 0 {
		return 0
	}
	return last
}

func spanWindow(e ActualEvent, t0 int64) (float64, float64) {
	if e.Visual != nil && e.Visual.EndedAtMs > e.Visual.StartedAtMs {
		return float64(e.Visual.StartedAtMs-t0) / 1000.0, float64(e.Visual.EndedAtMs-t0) / 1000.0
	}
	v0 := float64(e.AtMs-t0) / 1000.0
	v1 := v0
	if e.Visual != nil && e.Visual.DurationMs > 0 {
		v0 = v0 - float64(e.Visual.DurationMs)/1000.0
		v1 = float64(e.AtMs-t0) / 1000.0
	} else {
		v1 = v0 + 0.6
	}
	if v1 <= v0 {
		v1 = v0 + 0.3
	}
	return v0, v1
}

func interactionWindow(e ActualEvent, t0 int64) (v0, v1, zoom float64, nb *visual.BBox, label string, est bool) {
	label = e.Label
	if e.Visual == nil {
		v := float64(e.AtMs-t0) / 1000.0
		return v, v + 0.8, 1, nil, label, true
	}
	ve := e.Visual
	v0 = float64(ve.StartedAtMs-t0) / 1000.0
	if ve.EndedAtMs > ve.StartedAtMs {
		v1 = float64(ve.EndedAtMs-t0) / 1000.0
	} else {
		v1 = float64(e.AtMs-t0)/1000.0 + 0.8
	}
	zoom = ve.Zoom
	if zoom < 1 {
		zoom = 1
	}
	return v0, v1, zoom, ve.NormBBox, label, false
}

func actionAtS(e ActualEvent, t0 int64) float64 {
	if e.Visual != nil && e.Visual.ActionAtMs > 0 {
		return float64(e.Visual.ActionAtMs-t0) / 1000.0
	}
	return float64(e.AtMs-t0) / 1000.0
}

func waitOutput(actual, planned float64, compressible bool, opts ReconcileOpts) (out, speed float64, compressed bool) {
	if !compressible || actual <= 1.5 {
		return actual, 1, false
	}
	target := actual / 3
	if target < opts.MinWaitOutS {
		target = opts.MinWaitOutS
	}
	if target > opts.MaxWaitOutS {
		target = opts.MaxWaitOutS
	}
	if planned > target {
		target = planned
	}
	if target >= actual {
		return actual, 1, false
	}
	speed = actual / target
	if speed > opts.WaitSpeed {
		speed = opts.WaitSpeed
		target = actual / speed
	}
	return target, speed, true
}

// rangeOverlap returns seconds of overlap between [v0,v1) and any collected
// speech video window (action over narration is a sync failure).
func rangeOverlap(v0, v1 float64, wins [][2]float64) float64 {
	var overlap float64
	for _, w := range wins {
		lo := math.Max(v0, w[0])
		hi := math.Min(v1, w[1])
		if hi > lo {
			overlap += hi - lo
		}
	}
	return overlap
}

func (t *FinalTimeline) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func LoadFinalJSON(path string) (*FinalTimeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t FinalTimeline
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *SyncReport) Summary() (pass, total int, maxMs float64) {
	if r == nil {
		return 0, 0, 0
	}
	for _, it := range r.Items {
		total++
		if it.Pass {
			pass++
		}
	}
	return pass, total, r.MaxDriftMs
}
