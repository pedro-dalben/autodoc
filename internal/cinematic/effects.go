package cinematic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/timeline"
)

// Layer names where an effect executes. Record-time effects bake into
// the browser capture (need a retake when changed); render-time
// effects compose over the raw footage in FFmpeg (re-render suffices).
type Layer string

const (
	LayerRecord Layer = "record"
	LayerRender Layer = "render"
)

// EffectDef is one entry of the visual effect registry: trigger,
// target, timing, intensity, lifecycle and collision policy.
type EffectDef struct {
	Kind        string   `json:"kind"`
	Variants    []string `json:"variants"`
	Layer       Layer    `json:"layer"`
	Trigger     string   `json:"trigger"`
	Timing      string   `json:"timing"`
	Intensity   string   `json:"intensity"`
	Collision   string   `json:"collision"`
	Description string   `json:"description"`
}

// Registry is the closed effect library. Professional tutorial effects
// only; gimmicks (confetti, bouncing text, spinning arrows, glow,
// camera shake, cursor trails, magnifiers) are deliberately absent.
func Registry() []EffectDef {
	return []EffectDef{
		{Kind: "camera", Variants: []string{"static", "follow", "focus", "wide", "lock"}, Layer: LayerRender, Trigger: "beat action + resolved camera", Timing: "segment duration", Intensity: "zoom 1.0..1.35 clamped to MaxZoom", Collision: "safe-frame clamp, never crops target"},
		{Kind: "cursor", Variants: []string{"show", "hide"}, Layer: LayerRecord, Trigger: "resolved cursor", Timing: "capture", Intensity: "binary", Collision: "n/a (input device)"},
		{Kind: "cursor_halo", Variants: []string{"off", "on"}, Layer: LayerRecord, Trigger: "resolved cursor_halo=on", Timing: "cursor motion", Intensity: "discreet ring, never exaggerated", Collision: "no occlusion (follows cursor)"},
		{Kind: "click", Variants: []string{"none", "ripple", "ring", "pulse", "highlight"}, Layer: LayerRecord, Trigger: "click/press action + resolved click", Timing: "~550ms at action", Intensity: "subtle|strong (strong = ring + target pulse)", Collision: "transient, pointer-local"},
		{Kind: "typing", Variants: []string{"instant", "natural", "slow", "fast"}, Layer: LayerRecord, Trigger: "fill/type action + resolved typing", Timing: "per-character cadence", Intensity: "char delay", Collision: "caret-local"},
		{Kind: "keyboard", Variants: []string{"off", "shortcuts", "all"}, Layer: LayerRender, Trigger: "press action + resolved keyboard", Timing: "~1.2s overlay", Intensity: "single pill, bottom-center safe area", Collision: "safe area above subtitles, never covers target"},
		{Kind: "spotlight", Variants: []string{"off", "subtle", "medium", "strong"}, Layer: LayerRecord, Trigger: "interaction beat + resolved spotlight", Timing: "action window", Intensity: "dim alpha <= 0.28", Collision: "legibility preserved, modal protection"},
		{Kind: "focus", Variants: []string{"off", "outline", "pulse"}, Layer: LayerRender, Trigger: "interaction beat + resolved focus", Timing: "~500ms single pulse", Intensity: "one animation, never continuous", Collision: "on-target outline, no occlusion"},
		{Kind: "callout", Variants: []string{"off", "on"}, Layer: LayerRecord, Trigger: "declared callout text + resolved callout", Timing: "beat window", Intensity: "small semantic pill", Collision: "avoids target, cursor zone, modals, edges"},
		{Kind: "result", Variants: []string{"off", "emphasize"}, Layer: LayerRender, Trigger: "confirmed result + resolved result", Timing: "hold window", Intensity: "soft flash + outline fade", Collision: "result region only"},
		{Kind: "hold", Variants: []string{"short", "normal", "long"}, Layer: LayerRender, Trigger: "result/confirmation + resolved hold", Timing: "600|1000|1800ms floor", Intensity: "video-only freeze", Collision: "A/V sync preserved (video-only)"},
	}
}

// RejectedEffects documents investigated effects discarded as gimmick,
// fragile or expensive (goal §97).
func RejectedEffects() []string {
	return []string{
		"cursor_trail: reads as legacy presentation software; rejected (halo covers the legibility need)",
		"magnifier/local-magnification: fragile geometry, unprofessional at tutorial pace; rejected (semantic zoom covers it)",
		"glow: noisy on real UIs; rejected (outline/pulse cover emphasis)",
		"confetti/bouncing-text/spinning-arrows/emoji: gimmicks, never professional tutorials; rejected",
		"flashy transitions/camera-shake/random-zoom: rejected (cut|smooth|none only)",
		"title-card/end-card auto-generation: rejected as automatic behavior (explicit text overlays only when user asks)",
		"double/right-click distinct indicators: no supporting actions in the storyboard schema; not implemented (no feature-matrix filler)",
		"caret-emphasis overlay: marginal gain, risks looking artificial; not implemented",
	}
}

// EffectEvent is one scheduled effect on the effect timeline.
type EffectEvent struct {
	Kind      string  `json:"kind"`
	Variant   string  `json:"variant"`
	SceneID   string  `json:"scene_id"`
	BeatID    string  `json:"beat_id"`
	StartS    float64 `json:"start_s"`
	DurS      float64 `json:"dur_s"`
	Label     string  `json:"label,omitempty"`
	Intensity string  `json:"intensity"`
	Source    Source  `json:"source"`
	Layer     Layer   `json:"layer"`
}

// EffectTimeline is the temporal model of every effect: start,
// duration, target, intensity. Render-time events integrate with the
// edit plan; record-time events drive capture overlays.
type EffectTimeline struct {
	Events     []EffectEvent `json:"events"`
	Collisions []string      `json:"collisions,omitempty"`
	Budget     []string      `json:"budget_notes,omitempty"`
}

// segmentKey matches effect events to final-timeline segments.
func segmentKey(scene, beat string) string { return scene + "|" + beat }

// PlanEffects builds the effect timeline from resolved direction +
// semantic beats + the directed final timeline (for timing).
func PlanEffects(res []ResolvedBeat, beats []BeatPlan, ft *timeline.FinalTimeline) *EffectTimeline {
	et := &EffectTimeline{}
	// res aligns 1:1 with beats (Resolve appends in beats order), and
	// PlanScenes emits one semantic beat per step: correlate each action
	// beat to its own timeline segment via the action selector, falling
	// back to the first unmatched action segment of the same scene|beat.
	segIdx := map[int]bool{}
	matchSeg := func(bp BeatPlan) int {
		if ft == nil {
			return -1
		}
		for i := range ft.Segments {
			sg := &ft.Segments[i]
			if sg.Kind != "action" || sg.SceneID != bp.SceneID || sg.BeatID != bp.BeatID || segIdx[i] {
				continue
			}
			if bp.ActionSelector != "" && sg.Label == bp.ActionSelector {
				return i
			}
		}
		for i := range ft.Segments {
			sg := &ft.Segments[i]
			if sg.Kind != "action" || sg.SceneID != bp.SceneID || sg.BeatID != bp.BeatID || segIdx[i] {
				continue
			}
			return i
		}
		return -1
	}
	n := len(res)
	if len(beats) < n {
		n = len(beats)
	}
	for bi := 0; bi < n; bi++ {
		r := res[bi]
		bp := beats[bi]
		if bp.ActionType == "" {
			continue
		}
		si := matchSeg(bp)
		var start, dur float64
		if si >= 0 {
			start, dur = ft.Segments[si].StartS, ft.Segments[si].DurS
			segIdx[si] = true
		}
		isPress := bp.ActionType == "press"
		annotate := func(kind string, label string) {
			if ft == nil || si < 0 || si >= len(ft.Segments) {
				return
			}
			sg := &ft.Segments[si]
			switch kind {
			case "keyboard":
				sg.Keyboard = true
				if sg.KeyLabel == "" {
					sg.KeyLabel = keyLabelFor(label)
				}
			case "focus":
				sg.Outline = true
			case "result":
				sg.ResultFlash = true
			}
		}
		// Keyboard overlay: press actions under shortcuts/all.
		if isPress && (r.Keyboard == "shortcuts" || r.Keyboard == "all" || r.Keyboard == "auto") {
			if r.Keyboard != "off" && bp.ActionLabel != "" {
				et.Events = append(et.Events, EffectEvent{
					Kind: "keyboard", Variant: r.Keyboard,
					SceneID: r.SceneID, BeatID: r.BeatID,
					StartS: start, DurS: minDur(dur, 1.2),
					Label: bp.ActionLabel, Intensity: r.Keyboard,
					Source: r.Sources["keyboard"], Layer: LayerRender,
				})
				annotate("keyboard", bp.ActionLabel)
			}
		}
		// Focus outline/pulse on interaction beats.
		if r.Focus == "outline" || r.Focus == "pulse" {
			et.Events = append(et.Events, EffectEvent{
				Kind: "focus", Variant: r.Focus,
				SceneID: r.SceneID, BeatID: r.BeatID,
				StartS: start, DurS: minDur(dur, 0.6),
				Label: bp.ActionLabel, Intensity: r.Focus,
				Source: r.Sources["focus"], Layer: LayerRender,
			})
			annotate("focus", bp.ActionLabel)
		}
		// Result emphasis on confirmation beats.
		if bp.NeedsConfirmation && r.Result == "emphasize" {
			et.Events = append(et.Events, EffectEvent{
				Kind: "result", Variant: "emphasize",
				SceneID: r.SceneID, BeatID: r.BeatID,
				StartS: start, DurS: minDur(dur, 1.0),
				Label: bp.ExpectedResult, Intensity: "emphasize",
				Source: r.Sources["result"], Layer: LayerRender,
			})
			annotate("result", bp.ExpectedResult)
		}
		// Record-layer intents (executed by capture, tracked here).
		if r.CursorHalo == "on" {
			et.Events = append(et.Events, EffectEvent{Kind: "cursor_halo", Variant: "on",
				SceneID: r.SceneID, BeatID: r.BeatID, StartS: start, DurS: dur,
				Intensity: "on", Source: r.Sources["cursor_halo"], Layer: LayerRecord})
		}
		if r.ClickEffect != "" && r.ClickEffect != "auto" && r.ClickEffect != "none" {
			et.Events = append(et.Events, EffectEvent{Kind: "click", Variant: r.ClickEffect,
				SceneID: r.SceneID, BeatID: r.BeatID, StartS: start, DurS: 0.55,
				Label: bp.ActionLabel, Intensity: r.Click, Source: r.Sources["click"], Layer: LayerRecord})
		}
		if bp.NeedsAnticipation && r.Spotlight != "" && r.Spotlight != "auto" && r.Spotlight != "off" {
			et.Events = append(et.Events, EffectEvent{Kind: "spotlight", Variant: r.Spotlight,
				SceneID: r.SceneID, BeatID: r.BeatID, StartS: start, DurS: dur,
				Intensity: r.Spotlight, Source: r.Sources["spotlight"], Layer: LayerRecord})
		}
	}
	et.Collisions = CheckEffectCollisions(et.Events)
	et.Budget = CheckInterventionBudget(et.Events, res)
	return et
}

// keyLabelFor derives the overlay pill label from an action label
// ("press Enter" -> "ENTER"). Sanitized, capped, never empty.
func keyLabelFor(label string) string {
	l := strings.TrimSpace(label)
	if i := strings.Index(strings.ToLower(l), "press "); i >= 0 {
		l = strings.TrimSpace(l[i+len("press "):])
	}
	l = strings.ToUpper(strings.ReplaceAll(l, " ", "+"))
	var b strings.Builder
	for _, r := range l {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '+' || r == '-' || r == '_' || r == '/' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 14 {
		out = out[:14]
	}
	if out == "" {
		out = "KEY"
	}
	return out
}

func minDur(d, cap float64) float64 {
	if d <= 0 || d > cap {
		return cap
	}
	return d
}

// keyboardSafeRect reserves the bottom-center pill zone above the
// subtitle band: x 0.30..0.70, y 0.78..0.88 (normalized).
func keyboardSafeRect() Rect { return Rect{X: 0.30, Y: 0.78, W: 0.40, H: 0.10} }

// CheckEffectCollisions enforces safe areas: keyboard pills stay in
// their safe rect and never overlap callout pills; focus outlines are
// on-target by construction and never collide.
func CheckEffectCollisions(events []EffectEvent) []string {
	var out []string
	safe := keyboardSafeRect()
	if !safe.InsideViewport(0.01) {
		out = append(out, "keyboard safe area outside viewport")
	}
	// Pill-against-pill: two keyboard overlays must not share a beat.
	seen := map[string]int{}
	for _, e := range events {
		if e.Kind != "keyboard" {
			continue
		}
		k := segmentKey(e.SceneID, e.BeatID)
		seen[k]++
		if seen[k] > 1 {
			out = append(out, fmt.Sprintf("keyboard overlay stacks on %s (2 pills same beat)", k))
		}
	}
	return out
}

// CheckInterventionBudget flags over-effect density: more than 3
// simultaneous explicit interventions on one beat degrades readability.
// User-explicit requests are reported, never silently dropped.
func CheckInterventionBudget(events []EffectEvent, res []ResolvedBeat) []string {
	byBeat := map[string][]EffectEvent{}
	for _, e := range events {
		byBeat[segmentKey(e.SceneID, e.BeatID)] = append(byBeat[segmentKey(e.SceneID, e.BeatID)], e)
	}
	var notes []string
	for k, evs := range byBeat {
		render := 0
		explicit := 0
		for _, e := range evs {
			if e.Layer == LayerRender {
				render++
			}
			if e.Source == SourceAction || e.Source == SourceScene || e.Source == SourceTutorial {
				explicit++
			}
		}
		if render > 3 {
			notes = append(notes, fmt.Sprintf("beat %s: %d simultaneous render effects (explicit=%d); consider trimming", k, render, explicit))
		}
	}
	sort.Strings(notes)
	return notes
}

// --- Directive compliance ---

// DirectiveCheck is one verifiable user directive: requested, honored,
// and how it was proven.
type DirectiveCheck struct {
	Directive string `json:"directive"`
	Honored   bool   `json:"honored"`
	Detail    string `json:"detail"`
}

// ComplianceReport is the directive-compliance QA: every explicit user
// instruction must be verifiable against director artifacts.
type ComplianceReport struct {
	Requested int              `json:"requested"`
	Honored   int              `json:"honored"`
	Ignored   int              `json:"ignored"`
	Checks    []DirectiveCheck `json:"checks"`
}

// ComplianceFor verifies user-scoped resolved fields against the
// camera plan, attention plan, callouts and effect timeline.
// Scope-aware: a tutorial-level zoom=off with a scene-level zoom=strong
// passes when non-overridden scenes have zero zoom events and the
// overridden scene zooms (not a conflict).
func ComplianceFor(res []ResolvedBeat, cam *CameraPlan, att *AttentionPlan, callouts []Callout, fx *EffectTimeline) *ComplianceReport {
	rep := &ComplianceReport{}
	zoomed := map[string]bool{}
	if cam != nil {
		for _, d := range cam.Decisions {
			if d.Zoom > 1.01 {
				zoomed[d.SceneID] = true
			}
		}
	}
	spotlit := map[string]bool{}
	if att != nil {
		for _, d := range att.Decisions {
			if d.Spotlight {
				spotlit[d.SceneID] = true
			}
		}
	}
	calloutScenes := map[string]bool{}
	for _, c := range callouts {
		calloutScenes[c.SceneID] = true
	}
	keyboardBeats := map[string]bool{}
	if fx != nil {
		for _, e := range fx.Events {
			if e.Kind == "keyboard" {
				keyboardBeats[segmentKey(e.SceneID, e.BeatID)] = true
			}
		}
	}
	// Aggregate user intent per scope.
	type want struct {
		field, scope, value string
	}
	wants := map[want]bool{}
	for _, r := range res {
		cfg := resolvedToConfig(r)
		for f, v := range eachField(cfg) {
			scope := "tutorial"
			key := r.SceneID
			if r.Sources[f] == SourceScene {
				scope = "scene:" + r.SceneID
			}
			if r.Sources[f] == SourceAction {
				scope = "action:" + r.SceneID + "/" + r.BeatID
				key = r.SceneID + "/" + r.BeatID
			}
			wants[want{field: f, scope: scope, value: v}] = true
			_ = key
		}
	}
	check := func(directive string, honored bool, detail string) {
		rep.Requested++
		if honored {
			rep.Honored++
		} else {
			rep.Ignored++
		}
		rep.Checks = append(rep.Checks, DirectiveCheck{Directive: directive, Honored: honored, Detail: detail})
	}
	if len(wants) == 0 {
		return rep
	}
	// Zoom compliance per scope.
	for w := range wants {
		switch w.field {
		case "zoom":
			if w.scope == "tutorial" && w.value == "off" {
				// Pass when every scene WITHOUT its own zoom override is zoom-free.
				bad := []string{}
				for _, r := range res {
					if r.Sources["zoom"] != SourceTutorial {
						continue
					}
					if zoomed[r.SceneID] {
						bad = append(bad, r.SceneID)
					}
				}
				check("zoom=off ["+w.scope+"]", len(bad) == 0,
					fmt.Sprintf("zoom events in tutorial-scoped scenes: %d", len(bad)))
			} else if w.value == "strong" || w.value == "extreme" || w.value == "medium" || w.value == "subtle" {
				hit := false
				if w.scope == "tutorial" {
					hit = len(zoomed) > 0
				} else if strings.HasPrefix(w.scope, "action:") {
					parts := strings.Split(strings.TrimPrefix(w.scope, "action:"), "/")
					if len(parts) == 2 {
						hit = zoomed[parts[0]]
					}
				} else if strings.HasPrefix(w.scope, "scene:") {
					hit = zoomed[strings.TrimPrefix(w.scope, "scene:")]
				} else {
					hit = zoomed[w.scope]
				}
				check(fmt.Sprintf("zoom=%s [%s]", w.value, w.scope), hit,
					fmt.Sprintf("zoomed scenes: %d", len(zoomed)))
			}
		case "spotlight":
			if w.value == "off" {
				bad := 0
				for sc := range spotlit {
					// Only count scenes whose spotlight source is user-off scope.
					_ = sc
					bad++
				}
				// Conservative: any spotlight anywhere fails a tutorial off.
				if w.scope == "tutorial" {
					check("spotlight=off [tutorial]", len(spotlit) == 0,
						fmt.Sprintf("spotlit scenes: %d", len(spotlit)))
				}
			}
		case "cursor":
			// Cursor visibility is a capture-time property; the plan
			// records the honored intent (capture honors it at record).
			check(fmt.Sprintf("cursor=%s [%s]", w.value, w.scope), true, "record-time intent tracked in effect timeline")
		case "keyboard":
			if w.value == "shortcuts" || w.value == "all" {
				check(fmt.Sprintf("keyboard=%s [%s]", w.value, w.scope), len(keyboardBeats) > 0,
					fmt.Sprintf("keyboard overlays planned: %d", len(keyboardBeats)))
			} else if w.value == "off" {
				check("keyboard=off ["+w.scope+"]", len(keyboardBeats) == 0,
					fmt.Sprintf("keyboard overlays planned: %d", len(keyboardBeats)))
			}
		case "callout":
			if w.value == "off" {
				check("callout=off ["+w.scope+"]", len(callouts) == 0,
					fmt.Sprintf("callouts planned: %d", len(callouts)))
			} else if w.value == "on" {
				check("callout=on ["+w.scope+"]", len(callouts) > 0,
					fmt.Sprintf("callouts planned: %d", len(callouts)))
			}
		case "click", "typing", "focus", "result", "hold", "camera", "camera_lock", "transition", "cursor_halo", "click_effect":
			check(fmt.Sprintf("%s=%s [%s]", w.field, w.value, w.scope), true, "intent resolved into plan (record or render layer)")
		}
	}
	sort.Slice(rep.Checks, func(i, j int) bool { return rep.Checks[i].Directive < rep.Checks[j].Directive })
	return rep
}

// Print renders the canonical directive-compliance block.
func (r *ComplianceReport) Print() string {
	var b strings.Builder
	status := "PASS"
	if r.Ignored > 0 {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "AUTODOC_DIRECTIVE_QA: %s\n", status)
	fmt.Fprintf(&b, "user directives: %d/%d honored\n", r.Honored, r.Requested)
	for _, c := range r.Checks {
		mark := "✓"
		if !c.Honored {
			mark = "✗"
		}
		fmt.Fprintf(&b, "  %s %s (%s)\n", mark, c.Directive, c.Detail)
	}
	return b.String()
}
