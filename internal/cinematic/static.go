package cinematic

import (
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// ActivitySample is one analyzed window of the captured footage.
type ActivitySample struct {
	StartS  float64  `json:"start_s"`
	EndS    float64  `json:"end_s"`
	Level   string   `json:"level"` // LOW | ACTIVE | STATIC
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}

// StaticReport carries the static-scene analysis.
type StaticReport struct {
	Samples []ActivitySample `json:"samples"`
	// StaticS totals STATIC seconds; ActiveS totals ACTIVE seconds.
	StaticS float64 `json:"static_s"`
	ActiveS float64 `json:"active_s"`
}

// eventActivity adapts capture evidence into an activity score in [0,1].
// Inputs are deterministic capture facts (no pixel decoding here; frame
// evidence is asserted separately in E2E):
//
//   - interaction events (click/type/select/press/hover) => 1.0
//   - cursor travel (cursor_from != cursor_to) => scaled by distance
//   - typing progression (typing_chars > 0, progressive) => 0.9
//   - camera/zoom changes => 0.7
//   - overlay activity (highlight/ripple/spotlight/callout) => 0.6
//   - wait spans (loading) => 0.4 when compressible, 0.2 otherwise
//   - pure speech/hold windows => 0.05
func eventActivity(kind, interaction string, hasBBox, cursorMoved, progressive bool, typingChars int, zoom float64, overlay bool) float64 {
	switch kind {
	case "visual":
		switch interaction {
		case "click", "press", "select", "check", "uncheck":
			return 1.0
		case "fill", "type":
			if progressive && typingChars > 0 {
				return 0.9
			}
			return 0.7
		case "hover":
			return 0.5
		}
		if cursorMoved {
			return 0.6
		}
		if zoom > 1.01 || hasBBox {
			return 0.5
		}
		if overlay {
			return 0.6
		}
		return 0.3
	case "action", "goto":
		return 0.8
	case "wait_span":
		if cursorMoved || overlay {
			return 0.5
		}
		return 0.35
	case "speech_start", "speech_end", "hold_start", "hold_end", "pad":
		if cursorMoved {
			return 0.4
		}
		return 0.05
	default:
		if overlay || cursorMoved {
			return 0.4
		}
		return 0.1
	}
}

// AnalyzeStatic windows the final timeline into 1s samples scored from
// capture evidence (DOM/bbox, camera, cursor, overlay activity carried by
// the reconciled segments). Thresholds: score >= 0.45 ACTIVE, <= 0.18
// STATIC, else LOW.
func AnalyzeStatic(ft *timeline.FinalTimeline, events map[string][]timeline.ActualEvent) *StaticReport {
	rep := &StaticReport{}
	if ft == nil || ft.TotalS <= 0 {
		return rep
	}
	win := 1.0
	for s := 0.0; s < ft.TotalS; s += win {
		e := s + win
		if e > ft.TotalS {
			e = ft.TotalS
		}
		score, reasons := scoreWindow(ft, events, s, e)
		level := "LOW"
		switch {
		case score >= 0.45:
			level = "ACTIVE"
		case score <= 0.18:
			level = "STATIC"
		}
		rep.Samples = append(rep.Samples, ActivitySample{StartS: round2(s), EndS: round2(e), Level: level, Score: round2(score), Reasons: reasons})
		if level == "STATIC" {
			rep.StaticS += e - s
		} else if level == "ACTIVE" {
			rep.ActiveS += e - s
		}
	}
	return rep
}

func scoreWindow(ft *timeline.FinalTimeline, events map[string][]timeline.ActualEvent, s, e float64) (float64, []string) {
	var best float64
	var reasons []string
	add := func(v float64, r string) {
		if v > best {
			best = v
			reasons = []string{r}
		} else if v == best && v > 0 {
			reasons = append(reasons, r)
		}
	}
	for _, sg := range ft.Segments {
		if sg.StartS >= e || sg.StartS+sg.DurS <= s {
			continue
		}
		switch sg.Kind {
		case "action":
			v := 0.8
			r := "action:" + sg.Label
			if sg.Zoom > 1.01 {
				v = 1.0
				r += "+camera"
			}
			add(v, r)
		case "wait":
			if sg.Compressed {
				add(0.5, "compressed-transition:"+sg.Label)
			} else {
				add(0.3, "wait:"+sg.Label)
			}
		case "hold":
			add(0.1, "hold:"+sg.Label)
		case "speech":
			add(0.05, "speech:"+sg.Label)
		}
	}
	// Fold raw capture events overlapping the window (cursor/visual).
	for _, evs := range events {
		for _, ev := range evs {
			v := float64(ev.AtMs) / 1000.0
			_ = v
			if ev.Kind == "visual" && ev.Visual != nil {
				ve := ev.Visual
				moved := ve.CursorFrom != nil && ve.CursorTo != nil &&
					(ve.CursorFrom.X != ve.CursorTo.X || ve.CursorFrom.Y != ve.CursorTo.Y)
				hasBox := ve.BBox != nil || ve.NormBBox != nil
				a := eventActivity("visual", ve.Interaction, hasBox, moved, ve.Progressive, ve.TypingChars, ve.Zoom, ve.Type == "overlay")
				// Attribute to window via segment overlap already; only
				// upgrade when the event is strongly active.
				if a >= 0.6 {
					add(a, "capture:"+ve.Interaction)
				}
			}
		}
	}
	if len(reasons) == 0 {
		reasons = []string{"silence"}
	}
	return best, reasons
}

// StaticNarrationWindow flags speech windows that are visually orphaned.
type StaticNarrationWindow struct {
	SpeechID string  `json:"speech_id"`
	StartS   float64 `json:"start_s"`
	DurS     float64 `json:"dur_s"`
	Anchor   string  `json:"anchor"`
	StaticS  float64 `json:"static_s"`
	// Intentional marks stability that is correct (result reading,
	// conclusion, full-page explanation, modal): QA warns, never fails.
	Intentional bool   `json:"intentional"`
	Verdict     string `json:"verdict"` // ok | warn | fail
}

// DetectStaticNarration combines speech windows + visual activity +
// anchors. Speech longer than threshold with STATIC visuals and no
// meaningful anchor is unintentional (fail); the same stillness while
// reading a result or closing is intentional (ok).
func DetectStaticNarration(ft *timeline.FinalTimeline, beats []BeatPlan, rep *StaticReport, cfg visual.CinematicConfig) []StaticNarrationWindow {
	var out []StaticNarrationWindow
	anchorOf := map[string]BeatPlan{}
	for _, b := range beats {
		if b.SpeechID != "" {
			anchorOf[b.SpeechID] = b
		}
	}
	for _, sg := range ft.Segments {
		if sg.Kind != "speech" {
			continue
		}
		b := anchorOf[sg.Label]
		staticS := staticOverlap(rep, sg.StartS, sg.StartS+sg.DurS)
		intentional := b.Type == SceneConfirmation || b.Type == SceneCompletion ||
			b.Type == SceneResult || b.Anchor.Kind == AnchorViewport
		verdict := "ok"
		thresh := float64(cfg.QA.MaxUnintentionalStaticMs) / 1000.0
		if b.Anchor.Kind == AnchorNone && staticS >= thresh && !intentional {
			verdict = "fail"
		} else if b.Anchor.Kind == AnchorNone && staticS >= thresh/2 && !intentional {
			verdict = "warn"
		} else if staticS >= thresh && !intentional {
			verdict = "warn"
		}
		anchor := string(b.Anchor.Kind)
		if anchor == "" {
			anchor = "none"
		}
		out = append(out, StaticNarrationWindow{
			SpeechID: sg.Label, StartS: round2(sg.StartS), DurS: round2(sg.DurS),
			Anchor: anchor, StaticS: round2(staticS),
			Intentional: intentional, Verdict: verdict,
		})
	}
	return out
}

func staticOverlap(rep *StaticReport, s, e float64) float64 {
	var acc float64
	for _, sm := range rep.Samples {
		if sm.Level != "STATIC" {
			continue
		}
		lo := maxf(s, sm.StartS)
		hi := minf(e, sm.EndS)
		if hi > lo {
			acc += hi - lo
		}
	}
	return acc
}

func round2(f float64) float64 {
	if f < 0 {
		return 0
	}
	return float64(int(f*100+0.5)) / 100
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
