package cli

import (
	"fmt"
	"sort"

	"github.com/pedro-dalben/autodoc/internal/cinematic"
	"github.com/pedro-dalben/autodoc/internal/mcp"
	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// registerDirectionMCPTools exposes the directable-cinematics control
// surface as two high-level tools (never one tool per effect):
//
//   - cinematic_capabilities: compact on-demand capability discovery.
//   - direction_plan: compile natural-language + structured direction
//     into a validated, resolved plan preview (no recording).
func registerDirectionMCPTools(s *mcp.Server) {
	s.Register("cinematic_capabilities",
		"On-demand cinematic controls (camera/zoom/cursor/click/typing/keyboard/spotlight/focus/callout/result/hold, presets, precedence). Query when the user directs the video; unspecified controls stay auto.",
		map[string]any{"type": "object", "properties": map[string]any{}},
		func(args map[string]any) (any, error) {
			defs := cinematic.Registry()
			kinds := make([]string, 0, len(defs))
			for _, d := range defs {
				kinds = append(kinds, d.Kind)
			}
			return map[string]any{
				"controls":   visual.Capabilities(),
				"structured": visual.CapabilitiesMap(),
				"effects":    defs,
				"presets":    []string{"minimal", "balanced", "dynamic", "training"},
				"precedence": "action>scene>tutorial>preset>director",
				"layers":     "record-time (cursor/click/typing capture: needs retake) vs render-time (camera/zoom/spotlight/callout/keyboard/focus/result/hold: re-render only)",
			}, nil
		})
	s.Register("direction_plan",
		"Preview resolved cinematic direction: natural-language request and/or structured tutorial+scene overrides compiled, validated and resolved (action>scene>tutorial>preset>director). No browser, no render.",
		map[string]any{"type": "object", "properties": map[string]any{
			"storyboard": map[string]any{"type": "string", "description": "optional path to storyboard.yml for per-beat resolution preview"},
			"request":    map[string]any{"type": "string", "description": "optional free-form direction (PT/EN), e.g. 'sem zoom, mostre o cursor'"},
			"direction":  map[string]any{"type": "object", "description": "optional tutorial-level overrides, e.g. {\"zoom\":\"strong\",\"spotlight\":\"off\"}"},
			"scenes":     map[string]any{"type": "object", "description": "optional per-scene overrides, e.g. {\"login\":{\"zoom\":\"strong\"}}"},
		}},
		func(args map[string]any) (any, error) {
			tutorial := cinematic.CompileNL(mcp.StrArg(args, "request", ""))
			if m, ok := args["direction"].(map[string]any); ok {
				frag := directionFromMap(m)
				tutorial = tutorial.Merge(&frag)
			}
			sceneDirs := map[string]*visual.DirectionConfig{}
			if m, ok := args["scenes"].(map[string]any); ok {
				for k, v := range m {
					if vm, ok := v.(map[string]any); ok {
						d := directionFromMap(vm)
						sceneDirs[k] = &d
					}
				}
			}
			errs := tutorial.Validate()
			for sc, d := range sceneDirs {
				for _, e := range d.Validate() {
					errs = append(errs, fmt.Errorf("scene %s: %v", sc, e))
				}
			}
			render, record := visual.InvalidationScope(tutorial)
			out := map[string]any{
				"valid":      len(errs) == 0,
				"normalized": tutorial,
				"invalidation": map[string]any{
					"render_only":  render,
					"needs_retake": record,
				},
			}
			if len(errs) > 0 {
				strs := make([]string, 0, len(errs))
				for _, e := range errs {
					strs = append(strs, e.Error())
				}
				out["errors"] = strs
			}
			if sbPath := mcp.StrArg(args, "storyboard", ""); sbPath != "" {
				sb, err := storyboard.LoadFile(sbPath)
				if err != nil {
					return nil, err
				}
				preview := *sb
				preview.Direction = &tutorial
				for i := range preview.Scenes {
					if d, ok := sceneDirs[preview.Scenes[i].ID]; ok {
						preview.Scenes[i].Direction = d
					}
				}
				rec := recipe.Compile(&preview, "", "", "", 0)
				scenes := cinematic.PlanScenes(rec, &preview)
				res := cinematic.Resolve(&preview, scenes.Beats)
				var lines []string
				seen := map[string]bool{}
				for _, r := range res {
					key := r.SceneID
					if seen[key] {
						continue
					}
					seen[key] = true
					lines = append(lines, fmt.Sprintf("%s: zoom=%s[%s] camera=%s[%s] spotlight=%s[%s] keyboard=%s[%s]",
						r.SceneID, r.Zoom, r.Sources["zoom"], r.Camera, r.Sources["camera"],
						r.Spotlight, r.Sources["spotlight"], r.Keyboard, r.Sources["keyboard"]))
				}
				sort.Strings(lines)
				out["resolved_preview"] = lines
			}
			return out, nil
		})
}

// directionFromMap folds a flat string map into a DirectionConfig
// (unknown keys ignored so agent payloads stay forward-compatible).
func directionFromMap(m map[string]any) visual.DirectionConfig {
	str := func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	return visual.DirectionConfig{
		Preset: str("preset"), Camera: str("camera"), Zoom: str("zoom"),
		Cursor: str("cursor"), CursorHalo: str("cursor_halo"),
		Click: str("click"), ClickEffect: str("click_effect"),
		Typing: str("typing"), Keyboard: str("keyboard"),
		Spotlight: str("spotlight"), Focus: str("focus"),
		Callout: str("callout"), Result: str("result"), Hold: str("hold"),
		Transition: str("transition"), CameraLock: str("camera_lock"),
	}
}
