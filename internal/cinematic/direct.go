package cinematic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// CinematicPlan is the cinematic_plan.json artifact: attention + camera
// + callouts + pause plan in one deterministic document.
type CinematicPlan struct {
	StoryboardHash string         `json:"storyboard_hash"`
	Attention      *AttentionPlan `json:"attention"`
	Camera         *CameraPlan    `json:"camera"`
	Callouts       []Callout      `json:"callouts"`
	Pauses         *PausePlan     `json:"pauses"`
	CameraAdjusted int            `json:"camera_adjusted_segments"`
	HoldsAddedMs   int            `json:"holds_added_ms"`
	// Resolved is the per-beat direction contract (precedence:
	// action > scene > tutorial > preset > director). Absent direction
	// resolves to all-auto owned by the director.
	Resolved []ResolvedBeat `json:"resolved_direction,omitempty"`
	// Effects is the scheduled effect timeline (render + record layer).
	Effects *EffectTimeline `json:"effects,omitempty"`
	// Compliance verifies every explicit user directive.
	Compliance *ComplianceReport `json:"directive_compliance,omitempty"`
}

// Bundle carries every V2 artifact for one storyboard hash.
type Bundle struct {
	Scenes     *ScenePlanDoc
	Attention  *AttentionPlan
	Camera     *CameraPlan
	Edit       *EditPlan
	Static     *StaticReport
	StaticNar  []StaticNarrationWindow
	Callouts   []Callout
	Plan       *CinematicPlan
	Report     *CinematicReport
	Resolved   []ResolvedBeat
	Effects    *EffectTimeline
	Compliance *ComplianceReport
}

// Direct runs the full PLAN -> DIRECT -> EDIT -> VALIDATE chain over an
// already-reconciled final timeline. It mutates ft in place only through
// sync-safe adjustments (camera zooms, video-only result holds) and
// returns every intermediate artifact for workspace debugging.
func Direct(r *recipe.Recipe, sb *storyboard.Storyboard, ft *timeline.FinalTimeline, events map[string][]timeline.ActualEvent, vis visual.Config, cine visual.CinematicConfig) *Bundle {
	scenes := PlanScenes(r, sb)
	// Explicit direction resolves first (action > scene > tutorial >
	// preset > director) and threads into the beats before the
	// automatic directors run. Absent direction = all-auto.
	resolved := Resolve(sb, scenes.Beats)
	applyResolvedToBeats(scenes.Beats, resolved)
	att := DirectAttention(scenes.Beats, cine)
	static := AnalyzeStatic(ft, events)
	staticNar := DetectStaticNarration(ft, scenes.Beats, static, cine)
	callouts := PlanCallouts(scenes.Beats, cine)
	callouts = filterCalloutsByDirection(callouts, scenes.Beats, resolved)
	pauses := PlanPauses(r, vis.Pacing.SpeechActionGapMs, vis.Pacing.ActionSpeechGapMs)

	// Sync-safe direction applied to the final timeline: camera first,
	// then result holds (which reflow StartS deterministically).
	cam := DirectCamera(ft, att, scenes.Beats, cine, vis)
	enforceCameraLock(cam, scenes.Beats, resolved)
	adjusted := ApplyCamera(ft, cam)
	edit := BuildEditPlan(ft, scenes.Beats, cine)
	added := EnsureResultHoldsWithDirection(ft, scenes.Beats, cine, resolved)
	// Rebuild the edit plan after hold extension so the EDL matches the
	// rendered timeline exactly.
	if added > 0 {
		edit = BuildEditPlan(ft, scenes.Beats, cine)
		edit.HoldsAddedMs = added
	}

	fx := PlanEffects(resolved, scenes.Beats, ft)
	collisions := EvaluateCollisions(ft, scenes.Beats, callouts)
	collisions = append(collisions, fx.Collisions...)
	report := RunQA(ft, scenes, att, cam, edit, staticNar, collisions, cine)
	compliance := ComplianceFor(resolved, cam, att, callouts, fx)
	plan := &CinematicPlan{
		StoryboardHash: ft.StoryboardHash, Attention: att, Camera: cam,
		Callouts: callouts, Pauses: pauses,
		CameraAdjusted: adjusted, HoldsAddedMs: added,
		Resolved: resolved, Effects: fx, Compliance: compliance,
	}
	return &Bundle{Scenes: scenes, Attention: att, Camera: cam, Edit: edit, Static: static, StaticNar: staticNar, Callouts: callouts, Plan: plan, Report: report, Resolved: resolved, Effects: fx, Compliance: compliance}
}

// EvaluateCollisions checks planned callouts against directed targets for
// overlap violations (deterministic geometry, no frame decoding). Each
// callout anchors to the action box of its own beat, so sequential
// callouts never stack on one rectangle.
func EvaluateCollisions(ft *timeline.FinalTimeline, beats []BeatPlan, callouts []Callout) []string {
	active := []Callout{}
	for _, c := range callouts {
		if c.Suppressed == "" {
			active = append(active, c)
		}
	}
	if len(active) == 0 {
		return nil
	}
	byBeat := map[int]BeatPlan{}
	for _, b := range beats {
		byBeat[b.Index] = b
	}
	// Action boxes keyed by scene|beat|verb.
	boxOf := map[string]*visual.BBox{}
	for _, sg := range ft.Segments {
		if sg.Kind == "action" && sg.NormBBox != nil {
			verb := sg.Label
			if i := strings.Index(verb, " "); i > 0 {
				verb = verb[:i]
			}
			boxOf[beatKey(sg.SceneID, sg.BeatID)+"|"+verb] = sg.NormBBox
		}
	}
	// Callouts from different beats never share the frame (each is
	// removed when its action completes), so inter-callout overlap is
	// only meaningful within a beat. Group by beat and check each
	// group against its own anchor.
	byBeatCall := map[int][]Callout{}
	for _, c := range active {
		byBeatCall[c.BeatIndex] = append(byBeatCall[c.BeatIndex], c)
	}
	violations := []string{}
	for _, group := range byBeatCall {
		var gin CollisionInput
		for _, c := range group {
			var anchor *visual.BBox
			if b, ok := byBeat[c.BeatIndex]; ok {
				anchor = boxOf[beatKey(b.SceneID, b.BeatID)+"|"+b.ActionType]
				if anchor == nil {
					for k, v := range boxOf {
						if strings.HasPrefix(k, beatKey(b.SceneID, b.BeatID)+"|") {
							anchor = v
							break
						}
					}
				}
			}
			rect := CalloutRect(anchor, c.Text, c.Place)
			gin.Callouts = append(gin.Callouts, CalloutRectInput{Rect: rect, Text: c.Text})
			if anchor != nil && gin.Target == nil {
				nb := *anchor
				gin.Target = &nb
			}
		}
		violations = append(violations, CheckCollisions(gin)...)
	}
	return violations
}

// WriteBundle persists scene_plan.json, cinematic_plan.json, edit_plan.json
// and cinematic_report.json to dir (workspace/debug artifacts, never the
// publishable docs/ bundle).
func WriteBundle(dir string, b *Bundle) error {
	if err := b.Scenes.WriteJSON(filepath.Join(dir, "scene_plan.json")); err != nil {
		return err
	}
	if err := b.Edit.WriteJSON(filepath.Join(dir, "edit_plan.json")); err != nil {
		return err
	}
	if err := b.Report.WriteJSON(filepath.Join(dir, "cinematic_report.json")); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b.Plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cinematic_plan.json"), append(data, '\n'), 0o644)
}
