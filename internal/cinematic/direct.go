package cinematic

import (
	"encoding/json"
	"os"
	"path/filepath"

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
}

// Bundle carries every V2 artifact for one storyboard hash.
type Bundle struct {
	Scenes    *ScenePlanDoc
	Attention *AttentionPlan
	Camera    *CameraPlan
	Edit      *EditPlan
	Static    *StaticReport
	StaticNar []StaticNarrationWindow
	Callouts  []Callout
	Plan      *CinematicPlan
	Report    *CinematicReport
}

// Direct runs the full PLAN -> DIRECT -> EDIT -> VALIDATE chain over an
// already-reconciled final timeline. It mutates ft in place only through
// sync-safe adjustments (camera zooms, video-only result holds) and
// returns every intermediate artifact for workspace debugging.
func Direct(r *recipe.Recipe, sb *storyboard.Storyboard, ft *timeline.FinalTimeline, events map[string][]timeline.ActualEvent, vis visual.Config, cine visual.CinematicConfig) *Bundle {
	scenes := PlanScenes(r, sb)
	att := DirectAttention(scenes.Beats, cine)
	static := AnalyzeStatic(ft, events)
	staticNar := DetectStaticNarration(ft, scenes.Beats, static, cine)
	callouts := PlanCallouts(scenes.Beats, cine)
	pauses := PlanPauses(r, vis.Pacing.SpeechActionGapMs, vis.Pacing.ActionSpeechGapMs)

	// Sync-safe direction applied to the final timeline: camera first,
	// then result holds (which reflow StartS deterministically).
	cam := DirectCamera(ft, att, scenes.Beats, cine, vis)
	adjusted := ApplyCamera(ft, cam)
	edit := BuildEditPlan(ft, scenes.Beats, cine)
	added := EnsureResultHolds(ft, scenes.Beats, cine)
	// Rebuild the edit plan after hold extension so the EDL matches the
	// rendered timeline exactly.
	if added > 0 {
		edit = BuildEditPlan(ft, scenes.Beats, cine)
		edit.HoldsAddedMs = added
	}

	collisions := EvaluateCollisions(ft, callouts)
	report := RunQA(ft, scenes, att, cam, edit, staticNar, collisions, cine)
	plan := &CinematicPlan{
		StoryboardHash: ft.StoryboardHash, Attention: att, Camera: cam,
		Callouts: callouts, Pauses: pauses,
		CameraAdjusted: adjusted, HoldsAddedMs: added,
	}
	return &Bundle{Scenes: scenes, Attention: att, Camera: cam, Edit: edit, Static: static, StaticNar: staticNar, Callouts: callouts, Plan: plan, Report: report}
}

// EvaluateCollisions checks planned callouts against directed targets for
// overlap violations (deterministic geometry, no frame decoding).
func EvaluateCollisions(ft *timeline.FinalTimeline, callouts []Callout) []string {
	if len(callouts) == 0 {
		return nil
	}
	boxOf := map[string]*visual.BBox{}
	for _, sg := range ft.Segments {
		if sg.Kind == "action" && sg.NormBBox != nil {
			boxOf[sg.SceneID+"|"+sg.Label] = sg.NormBBox
		}
	}
	var in CollisionInput
	seen := map[string]bool{}
	for _, c := range callouts {
		if c.Suppressed != "" {
			continue
		}
		var anchor *visual.BBox
		for k, b := range boxOf {
			if seen[k] {
				continue
			}
			_ = b
		}
		// Attach to the first action box of the same scene when the
		// anchor key is unknown; geometry stays deterministic.
		for _, sg := range ft.Segments {
			if sg.SceneID == c.SceneID && sg.Kind == "action" && sg.NormBBox != nil {
				anchor = sg.NormBBox
				break
			}
		}
		rect := CalloutRect(anchor, c.Text, c.Place)
		in.Callouts = append(in.Callouts, CalloutRectInput{Rect: rect, Text: c.Text})
		if anchor != nil && in.Target == nil {
			nb := *anchor
			in.Target = &nb
		}
	}
	return CheckCollisions(in)
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
