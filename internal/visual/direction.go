package visual

import (
	"fmt"
	"sort"
	"strings"
)

// DirectionConfig is the explicit cinematic direction contract.
//
// Every field is semantic intent, never an implementation parameter:
// the director chooses coordinates, scales and durations. Empty string
// means "unset at this scope" (falls through to the next precedence
// level, ultimately the automatic director). This keeps storyboards
// small: only non-default overrides are persisted.
//
// Precedence: action > scene > tutorial > preset > automatic director.
type DirectionConfig struct {
	Preset      string `yaml:"preset,omitempty" json:"preset,omitempty"`
	Camera      string `yaml:"camera,omitempty" json:"camera,omitempty"`
	Zoom        string `yaml:"zoom,omitempty" json:"zoom,omitempty"`
	Cursor      string `yaml:"cursor,omitempty" json:"cursor,omitempty"`
	CursorHalo  string `yaml:"cursor_halo,omitempty" json:"cursor_halo,omitempty"`
	Click       string `yaml:"click,omitempty" json:"click,omitempty"`
	ClickEffect string `yaml:"click_effect,omitempty" json:"click_effect,omitempty"`
	Typing      string `yaml:"typing,omitempty" json:"typing,omitempty"`
	Keyboard    string `yaml:"keyboard,omitempty" json:"keyboard,omitempty"`
	Spotlight   string `yaml:"spotlight,omitempty" json:"spotlight,omitempty"`
	Focus       string `yaml:"focus,omitempty" json:"focus,omitempty"`
	Callout     string `yaml:"callout,omitempty" json:"callout,omitempty"`
	Result      string `yaml:"result,omitempty" json:"result,omitempty"`
	Hold        string `yaml:"hold,omitempty" json:"hold,omitempty"`
	Transition  string `yaml:"transition,omitempty" json:"transition,omitempty"`
	CameraLock  string `yaml:"camera_lock,omitempty" json:"camera_lock,omitempty"`
}

// Direction value sets. "auto" is first-class everywhere; "" (unset)
// behaves as auto at resolution time.
var (
	ValidPresets     = []string{"auto", "minimal", "balanced", "dynamic", "training"}
	ValidCameras     = []string{"auto", "static", "subtle", "dynamic", "follow", "focus", "wide"}
	ValidZooms       = []string{"auto", "off", "subtle", "medium", "strong", "extreme"}
	ValidCursors     = []string{"auto", "show", "hide"}
	ValidHalos       = []string{"auto", "off", "on"}
	ValidClicks      = []string{"auto", "off", "subtle", "strong"}
	ValidClickFx     = []string{"auto", "none", "ripple", "ring", "pulse", "highlight"}
	ValidTypings     = []string{"auto", "instant", "natural", "slow", "fast"}
	ValidKeyboards   = []string{"auto", "off", "shortcuts", "all"}
	ValidSpotlights  = []string{"auto", "off", "subtle", "medium", "strong"}
	ValidFocus       = []string{"auto", "off", "outline", "pulse"}
	ValidCallouts    = []string{"auto", "off", "on"}
	ValidResults     = []string{"auto", "off", "emphasize"}
	ValidHolds       = []string{"auto", "short", "normal", "long"}
	ValidTransitions = []string{"auto", "cut", "smooth", "none"}
	ValidLocks       = []string{"auto", "off", "on"}
)

func validIn(v string, set []string) bool {
	if v == "" || v == "auto" {
		return true
	}
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// Validate returns compact actionable errors for invalid enum values
// and contradictory combinations (cursor hidden + halo, static camera
// + follow/focus zoom intent, click off + explicit effect).
func (d *DirectionConfig) Validate() []error {
	if d == nil {
		return nil
	}
	var errs []error
	check := func(name, v string, set []string) {
		if !validIn(v, set) {
			errs = append(errs, fmt.Errorf("direction.%s=%q invalid (want one of %s)", name, v, strings.Join(set, "|")))
		}
	}
	check("preset", d.Preset, ValidPresets)
	check("camera", d.Camera, ValidCameras)
	check("zoom", d.Zoom, ValidZooms)
	check("cursor", d.Cursor, ValidCursors)
	check("cursor_halo", d.CursorHalo, ValidHalos)
	check("click", d.Click, ValidClicks)
	check("click_effect", d.ClickEffect, ValidClickFx)
	check("typing", d.Typing, ValidTypings)
	check("keyboard", d.Keyboard, ValidKeyboards)
	check("spotlight", d.Spotlight, ValidSpotlights)
	check("focus", d.Focus, ValidFocus)
	check("callout", d.Callout, ValidCallouts)
	check("result", d.Result, ValidResults)
	check("hold", d.Hold, ValidHolds)
	check("transition", d.Transition, ValidTransitions)
	check("camera_lock", d.CameraLock, ValidLocks)
	if d.Cursor == "hide" && d.CursorHalo == "on" {
		errs = append(errs, fmt.Errorf("direction.cursor=hide conflicts with cursor_halo=on (halo needs a visible cursor)"))
	}
	if d.Camera == "static" && (d.Zoom == "strong" || d.Zoom == "extreme" || d.CameraLock == "on") {
		// Static pins the full-viewport shot; any zoom intent contradicts it.
		if d.Zoom == "strong" || d.Zoom == "extreme" {
			errs = append(errs, fmt.Errorf("direction.camera=static conflicts with zoom=%s (static pins full viewport; use camera=follow|focus for zoom)", d.Zoom))
		}
	}
	if d.Click == "off" && (d.ClickEffect == "ripple" || d.ClickEffect == "ring" || d.ClickEffect == "pulse" || d.ClickEffect == "highlight") {
		errs = append(errs, fmt.Errorf("direction.click=off conflicts with click_effect=%s (use click=subtle|strong)", d.ClickEffect))
	}
	return errs
}

// Empty reports whether no explicit direction is set at this scope.
func (d *DirectionConfig) Empty() bool {
	if d == nil {
		return true
	}
	return *d == DirectionConfig{}
}

// Merge overlays non-empty fields of over onto d (over wins per field).
func (d DirectionConfig) Merge(over *DirectionConfig) DirectionConfig {
	if over == nil {
		return d
	}
	out := d
	if over.Preset != "" {
		out.Preset = over.Preset
	}
	if over.Camera != "" {
		out.Camera = over.Camera
	}
	if over.Zoom != "" {
		out.Zoom = over.Zoom
	}
	if over.Cursor != "" {
		out.Cursor = over.Cursor
	}
	if over.CursorHalo != "" {
		out.CursorHalo = over.CursorHalo
	}
	if over.Click != "" {
		out.Click = over.Click
	}
	if over.ClickEffect != "" {
		out.ClickEffect = over.ClickEffect
	}
	if over.Typing != "" {
		out.Typing = over.Typing
	}
	if over.Keyboard != "" {
		out.Keyboard = over.Keyboard
	}
	if over.Spotlight != "" {
		out.Spotlight = over.Spotlight
	}
	if over.Focus != "" {
		out.Focus = over.Focus
	}
	if over.Callout != "" {
		out.Callout = over.Callout
	}
	if over.Result != "" {
		out.Result = over.Result
	}
	if over.Hold != "" {
		out.Hold = over.Hold
	}
	if over.Transition != "" {
		out.Transition = over.Transition
	}
	if over.CameraLock != "" {
		out.CameraLock = over.CameraLock
	}
	return out
}

// presetDefaults maps high-level presets to concrete semantic values.
// Minimal keeps the interface protagonist; balanced is today's default;
// dynamic pushes camera + click emphasis; training maximizes legibility
// (keyboard, focus, holds).
func presetDefaults(p string) DirectionConfig {
	switch p {
	case "minimal":
		return DirectionConfig{Camera: "static", Zoom: "off", Cursor: "show", Click: "subtle", ClickEffect: "none", Typing: "natural", Keyboard: "off", Spotlight: "off", Focus: "off", Callout: "off", Result: "auto", Hold: "short", Transition: "cut", CameraLock: "off"}
	case "dynamic":
		return DirectionConfig{Camera: "dynamic", Zoom: "medium", Cursor: "show", CursorHalo: "on", Click: "strong", ClickEffect: "ring", Typing: "fast", Keyboard: "shortcuts", Spotlight: "subtle", Focus: "pulse", Callout: "auto", Result: "emphasize", Hold: "short", Transition: "smooth", CameraLock: "off"}
	case "training":
		return DirectionConfig{Camera: "follow", Zoom: "medium", Cursor: "show", CursorHalo: "on", Click: "strong", ClickEffect: "ripple", Typing: "slow", Keyboard: "all", Spotlight: "medium", Focus: "outline", Callout: "on", Result: "emphasize", Hold: "long", Transition: "smooth", CameraLock: "off"}
	default: // balanced / auto
		return DirectionConfig{Camera: "auto", Zoom: "auto", Cursor: "auto", Click: "auto", Typing: "auto", Keyboard: "shortcuts", Spotlight: "auto", Focus: "auto", Callout: "auto", Result: "auto", Hold: "auto", Transition: "auto", CameraLock: "auto"}
	}
}

// PresetName normalizes "" to "balanced" (today's automatic default).
func PresetName(p string) string {
	if p == "" || p == "auto" {
		return "balanced"
	}
	return p
}

// WithPreset fills every still-unset field from the named preset.
func (d DirectionConfig) WithPreset(preset string) DirectionConfig {
	def := presetDefaults(PresetName(preset))
	out := def
	out = out.Merge(&d)
	out.Preset = PresetName(preset)
	return out
}

// ZoomScale maps semantic zoom intent to a render-time scale factor.
// Safety clamp to maxZoom happens at apply time (never above 1.5,
// never cropping the target); off/static resolve to 1.0.
func ZoomScale(zoom string) float64 {
	switch zoom {
	case "off":
		return 1.0
	case "subtle":
		return 1.12
	case "medium":
		return 1.18
	case "strong":
		return 1.25
	case "extreme":
		return 1.35
	default: // auto
		return 0 // 0 = director decides per action
	}
}

// HoldMs maps semantic hold intent to a render-time hold floor.
func HoldMs(hold string, defMs int) int {
	if defMs <= 0 {
		defMs = 1000
	}
	switch hold {
	case "short":
		return 600
	case "normal":
		return 1000
	case "long":
		return 1800
	default:
		return defMs
	}
}

// SpotlightAlpha maps semantic spotlight intensity to dim alpha
// (always within the legibility ceiling 0.28).
func SpotlightAlpha(s string) float64 {
	switch s {
	case "off":
		return 0
	case "subtle":
		return 0.12
	case "medium":
		return 0.18
	case "strong":
		return 0.25
	default:
		return -1 // director default
	}
}

// InvalidationScope classifies which pipeline stages a direction change
// invalidates. Render-time fields (camera, zoom, spotlight, callouts,
// keyboard overlay, focus, result, holds) need only a re-render of the
// existing raw capture; record-time fields (cursor visibility, click
// capture, typing cadence) need a browser retake.
func InvalidationScope(d DirectionConfig) (render []string, record []string) {
	field := func(name, v string, recordTime bool) {
		if v == "" || v == "auto" {
			return
		}
		if recordTime {
			record = append(record, name)
		} else {
			render = append(render, name)
		}
	}
	field("camera", d.Camera, false)
	field("zoom", d.Zoom, false)
	field("cursor", d.Cursor, true)
	field("cursor_halo", d.CursorHalo, false)
	field("click", d.Click, true)
	field("click_effect", d.ClickEffect, true)
	field("typing", d.Typing, true)
	field("keyboard", d.Keyboard, false)
	field("spotlight", d.Spotlight, false)
	field("focus", d.Focus, false)
	field("callout", d.Callout, false)
	field("result", d.Result, false)
	field("hold", d.Hold, false)
	field("transition", d.Transition, false)
	field("camera_lock", d.CameraLock, false)
	sort.Strings(render)
	sort.Strings(record)
	return render, record
}

// NeedsRetake reports whether changing prev to next requires a new
// browser recording (any record-time field differs).
func NeedsRetake(prev, next DirectionConfig) bool {
	return prev.Cursor != next.Cursor || prev.Click != next.Click ||
		prev.ClickEffect != next.ClickEffect || prev.Typing != next.Typing
}

// Capabilities returns the compact on-demand capability summary
// (also served over MCP; never embedded in the core skill).
func Capabilities() string {
	return "camera: auto|static|subtle|dynamic|follow|focus|wide\n" +
		"zoom: auto|off|subtle|medium|strong|extreme\n" +
		"cursor: auto|show|hide  cursor_halo: auto|off|on\n" +
		"click: auto|off|subtle|strong  click_effect: auto|none|ripple|ring|pulse|highlight\n" +
		"typing: auto|instant|natural|slow|fast\n" +
		"keyboard: auto|off|shortcuts|all\n" +
		"spotlight: auto|off|subtle|medium|strong\n" +
		"focus: auto|off|outline|pulse\n" +
		"callout: auto|off|on  result: auto|off|emphasize\n" +
		"hold: auto|short|normal|long  transition: auto|cut|smooth|none\n" +
		"camera_lock: auto|off|on  preset: auto|minimal|balanced|dynamic|training\n" +
		"precedence: action>scene>tutorial>preset>director  unset=auto"
}

// CapabilitiesMap is the machine-readable form of Capabilities.
func CapabilitiesMap() map[string][]string {
	return map[string][]string{
		"camera":       ValidCameras,
		"zoom":         ValidZooms,
		"cursor":       ValidCursors,
		"cursor_halo":  ValidHalos,
		"click":        ValidClicks,
		"click_effect": ValidClickFx,
		"typing":       ValidTypings,
		"keyboard":     ValidKeyboards,
		"spotlight":    ValidSpotlights,
		"focus":        ValidFocus,
		"callout":      ValidCallouts,
		"result":       ValidResults,
		"hold":         ValidHolds,
		"transition":   ValidTransitions,
		"camera_lock":  ValidLocks,
		"preset":       ValidPresets,
	}
}
