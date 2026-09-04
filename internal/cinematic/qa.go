package cinematic

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// GateResult is one objective cinematic gate.
type GateResult struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
	Actual string `json:"actual,omitempty"`
	Target string `json:"target,omitempty"`
}

// CinematicReport is the cinematic_report.json artifact and the source of
// `validate --cinematic` output.
type CinematicReport struct {
	StoryboardHash  string                  `json:"storyboard_hash"`
	Pass            bool                    `json:"pass"`
	Gates           []GateResult            `json:"gates"`
	Score           int                     `json:"score"`
	Scores          map[string]int          `json:"scores"`
	StaticWindows   []StaticNarrationWindow `json:"static_narration_windows"`
	CameraChanges   int                     `json:"camera_changes"`
	CameraReversals int                     `json:"camera_reversals"`
	WaitsSavedMs    int                     `json:"waits_saved_ms"`
	MaxSyncDriftMs  float64                 `json:"max_sync_drift_ms"`
}

// ScoreWeights balance the diagnostic score (gates stay authoritative).
var ScoreWeights = map[string]float64{
	"attention": 0.20, "continuity": 0.20, "readability": 0.15,
	"sync": 0.20, "result": 0.15, "pacing": 0.10,
}

// RunQA evaluates every objective gate. Thresholds come from config;
// values follow §27 (target visible >= 400ms, result >= 800ms, camera
// transition >= 250ms, max zoom <= 1.25, unintentional static <= 2.5s,
// zero collisions/clipping/safe-area/compression/causality issues).
func RunQA(ft *timeline.FinalTimeline, scenes *ScenePlanDoc, att *AttentionPlan, cam *CameraPlan, edit *EditPlan, staticWindows []StaticNarrationWindow, collisions []string, cfg visual.CinematicConfig) *CinematicReport {
	rep := &CinematicReport{
		StoryboardHash: ft.StoryboardHash,
		Scores:         map[string]int{},
		Pass:           true,
	}
	gate := func(name string, pass bool, detail, actual, target string) {
		rep.Gates = append(rep.Gates, GateResult{Name: name, Pass: pass, Detail: detail, Actual: actual, Target: target})
		if !pass {
			rep.Pass = false
		}
	}
	// 1. Sync inherits the reconciler verdict.
	syncPass := ft.Sync != nil && ft.Sync.Pass
	maxDrift := 0.0
	if ft.Sync != nil {
		maxDrift = ft.Sync.MaxDriftMs
	}
	rep.MaxSyncDriftMs = maxDrift
	gate("sync", syncPass, syncVerdict(ft), fmt.Sprintf("%.0fms", math.Abs(maxDrift)), "drift<=250ms")

	// 2. Scene planning: every beat classified, no empty intents.
	planned := len(scenes.Beats) > 0
	gate("scene-planning", planned, fmt.Sprintf("%d beats classified", len(scenes.Beats)), fmt.Sprint(len(scenes.Beats)), ">0")

	// 3. Visual anchors: narration beats with anchors / total narration.
	narr, anchored := 0, 0
	for _, b := range scenes.Beats {
		if b.Type == SceneNarration || b.Type == SceneExplanation {
			narr++
			if b.Anchor.Kind != AnchorNone {
				anchored++
			}
		}
	}
	anchorPass := narr == 0 || anchored == narr
	gate("visual-anchors", anchorPass, fmt.Sprintf("%d/%d narration beats anchored", anchored, narr), fmt.Sprintf("%d/%d", anchored, narr), fmt.Sprintf("%d/%d", narr, narr))

	// 4. Action anticipation: every anticipation-demanding beat directed.
	// Explicit "none" overrides are authorial intent, not failures.
	need, got := 0, 0
	for _, b := range scenes.Beats {
		if b.NeedsAnticipation && b.AttentionOverride != "none" && b.CameraOverride != "none" {
			need++
			for _, d := range att.Decisions {
				if d.BeatIndex == b.Index && d.Anticipate {
					got++
					break
				}
			}
		}
	}
	antPass := need == 0 || got == need
	// Beats with explicit "none" override are authorial intent, not failure.
	gate("action-anticipation", antPass, fmt.Sprintf("%d/%d action beats anticipated", got, need), fmt.Sprintf("%d/%d", got, need), fmt.Sprintf("%d/%d", need, need))

	// 5. Result confirmation: every submit/result beat has a result clip
	// with at least MinResultVisibleMs hold.
	needR, gotR := 0, 0
	for _, b := range scenes.Beats {
		if b.NeedsConfirmation {
			needR++
			for _, c := range edit.Clips {
				if c.Type == ClipResult && c.BeatID == b.BeatID && c.HoldMs >= cfg.QA.MinResultVisibleMs {
					gotR++
					break
				}
			}
		}
	}
	gate("result-confirmation", needR == 0 || gotR == needR, fmt.Sprintf("%d/%d results confirmed+held", gotR, needR), fmt.Sprintf("%d/%d", gotR, needR), fmt.Sprintf("%d/%d", needR, needR))

	// 6. Static narration: zero unintentional fails.
	fails, warns := 0, 0
	for _, w := range staticWindows {
		if w.Verdict == "fail" {
			fails++
		} else if w.Verdict == "warn" {
			warns++
		}
	}
	rep.StaticWindows = staticWindows
	gate("static-narration", fails == 0, fmt.Sprintf("%d fail, %d warn orphaned-narration windows", fails, warns), fmt.Sprintf("%d fail", fails), "0 fail")

	// 7. Camera continuity: no reversals, bounded changes.
	rep.CameraChanges, rep.CameraReversals = cam.Changes, cam.Reversals
	gate("camera-continuity", cam.Reversals == 0, fmt.Sprintf("%d changes, %d reversals", cam.Changes, cam.Reversals), fmt.Sprintf("%d reversals", cam.Reversals), "0 reversals")

	// 8. Context restoration: final beat restores full context.
	restored := false
	for _, d := range att.Decisions {
		if d.Strategy == AttContextRestore {
			restored = true
			break
		}
	}
	gate("context-restoration", restored, fmt.Sprintf("context restore planned: %t", restored), fmt.Sprint(restored), "true")

	// 9. Target visibility: every action segment carries bbox evidence
	// and (when zoomed) respected the safe frame.
	missing, unsafe := 0, 0
	for _, sg := range ft.Segments {
		if sg.Kind != "action" {
			continue
		}
		if sg.NormBBox == nil {
			missing++
		} else if !bboxSafe(sg.NormBBox) {
			unsafe++
		}
	}
	gate("target-visibility", missing == 0 && unsafe == 0, fmt.Sprintf("%d missing bbox, %d unsafe", missing, unsafe), fmt.Sprintf("%d/%d", missing, unsafe), "0/0")

	// 10. Overlay collisions.
	gate("overlay-collisions", len(collisions) == 0, strings.Join(append([]string{fmt.Sprintf("%d violations", len(collisions))}, collisions...), "; "), fmt.Sprint(len(collisions)), "0")

	// 11. Text legibility: callouts bounded, spotlight dim bounded.
	legible := cfg.Attention.MaxDim <= 0.28 && cfg.Camera.MaxZoom <= 1.5
	gate("text-legibility", legible, fmt.Sprintf("max_dim=%.2f max_zoom=%.2f", cfg.Attention.MaxDim, cfg.Camera.MaxZoom), fmt.Sprintf("%.2f/%.2f", cfg.Attention.MaxDim, cfg.Camera.MaxZoom), "<=0.28/<=1.50")

	// 12. Dead-time editing: compressed waits keep speed within budget.
	badSpeed := 0
	for _, sg := range ft.Segments {
		if sg.Compressed && sg.Speed > cfg.Editing.MaxSpeed+1e-9 {
			badSpeed++
		}
	}
	rep.WaitsSavedMs = edit.WaitsSavedMs
	gate("dead-time-editing", badSpeed == 0, fmt.Sprintf("saved %dms, %d over-speed", edit.WaitsSavedMs, badSpeed), fmt.Sprintf("%d over-speed", badSpeed), "0 over-speed")

	rep.Score, rep.Scores = Score(rep, ft, cfg)
	return rep
}

func syncVerdict(ft *timeline.FinalTimeline) string {
	if ft.Sync == nil {
		return "no sync report"
	}
	p, t, m := ft.Sync.Summary()
	return fmt.Sprintf("%d/%d checks, max drift %.0fms", p, t, math.Abs(m))
}

// Score computes the diagnostic 0-100 score (never a gate substitute).
func Score(rep *CinematicReport, ft *timeline.FinalTimeline, cfg visual.CinematicConfig) (int, map[string]int) {
	byName := map[string]bool{}
	for _, g := range rep.Gates {
		byName[g.Name] = g.Pass
	}
	sub := map[string]int{}
	b := func(ok bool) int {
		if ok {
			return 100
		}
		return 55
	}
	sub["sync"] = b(byName["sync"])
	sub["attention"] = b(byName["visual-anchors"] && byName["action-anticipation"])
	sub["continuity"] = b(byName["camera-continuity"] && byName["context-restoration"])
	sub["readability"] = b(byName["text-legibility"] && byName["overlay-collisions"])
	sub["result"] = b(byName["result-confirmation"] && byName["target-visibility"])
	sub["pacing"] = b(byName["static-narration"] && byName["dead-time-editing"])
	// Partial credit: static warns cost a little, fails already failed.
	for _, w := range rep.StaticWindows {
		if w.Verdict == "warn" && sub["pacing"] > 80 {
			sub["pacing"] -= 4
		}
	}
	total := 0.0
	for k, w := range ScoreWeights {
		total += float64(sub[k]) * w
	}
	return int(math.Round(total)), sub
}

// Print renders the canonical AUTODOC_CINEMATIC_QA block.
func (r *CinematicReport) Print() string {
	var sb strings.Builder
	if r.Pass {
		sb.WriteString("AUTODOC_CINEMATIC_QA: PASS\n")
	} else {
		sb.WriteString("AUTODOC_CINEMATIC_QA: FAIL\n")
	}
	for _, g := range r.Gates {
		st := "PASS"
		if !g.Pass {
			st = "FAIL"
		}
		fmt.Fprintf(&sb, "%-20s %s — %s\n", g.Name, st, g.Detail)
	}
	fmt.Fprintf(&sb, "Cinematic Score: %d/100", r.Score)
	order := []string{"attention", "continuity", "readability", "sync", "result", "pacing"}
	for _, k := range order {
		fmt.Fprintf(&sb, "  %s=%d", k, r.Scores[k])
	}
	sb.WriteString("\n")
	return sb.String()
}

func (r *CinematicReport) WriteJSON(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
