package storyboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/visual"
)

type Storyboard struct {
	Version int            `yaml:"version" json:"version"`
	Meta    Meta           `yaml:"meta" json:"meta"`
	Config  Config         `yaml:"config" json:"config"`
	Setup   Setup          `yaml:"setup" json:"setup"`
	Visuals *visual.Config `yaml:"visuals,omitempty" json:"visuals,omitempty"`
	// Cinematic carries optional AI Director overrides. Nil/absent means
	// safe V2 defaults (director on, callouts/sound off). V1 storyboards
	// without this block keep validating and rendering unchanged.
	Cinematic *visual.CinematicConfig `yaml:"cinematic,omitempty" json:"cinematic,omitempty"`
	Scenes    []Scene                 `yaml:"scenes" json:"scenes"`
	Redact    Redact                  `yaml:"redact" json:"redact"`
}

type Meta struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Language    string `yaml:"language" json:"language"`
	Resolution  string `yaml:"resolution" json:"resolution"`
	FPS         int    `yaml:"fps" json:"fps"`
	Author      string `yaml:"author" json:"author"`
}

type Config struct {
	BaseURL   string `yaml:"base_url" json:"base_url"`
	ViewportW int    `yaml:"viewport_width" json:"viewport_width"`
	ViewportH int    `yaml:"viewport_height" json:"viewport_height"`
}

type Setup struct {
	StartURL       string          `yaml:"start_url" json:"start_url"`
	StorageState   string          `yaml:"storage_state" json:"storage_state"`
	Sequence       []SetupStep     `yaml:"sequence,omitempty" json:"sequence,omitempty"`
	Preconditions  []SetupAction   `yaml:"preconditions" json:"preconditions"`
	ExpectedStates []ExpectedState `yaml:"expected_states" json:"expected_states"`
}

// SetupStep is one off-camera preparation step. It runs before capture
// starts, never appears in the video, and may reference secrets via
// secret_ref (env:NAME) instead of literal values.
type SetupStep struct {
	Action *Action    `yaml:"action,omitempty" json:"action,omitempty"`
	Wait   *WaitEvent `yaml:"wait,omitempty" json:"wait,omitempty"`
}

type SetupAction struct {
	Action Action `yaml:",inline" json:",inline"`
}

type Scene struct {
	ID          string `yaml:"id" json:"id"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	URL         string `yaml:"url" json:"url"`
	Beats       []Beat `yaml:"beats" json:"beats"`
	Screenshot  bool   `yaml:"screenshot" json:"screenshot"`
	// Optional V2 overrides for special situations (default: semantic).
	Camera    string `yaml:"camera,omitempty" json:"camera,omitempty"`
	Attention string `yaml:"attention,omitempty" json:"attention,omitempty"`
	// Targets and Defaults are optional compact-authoring helpers. They expand
	// in memory before validation/compilation, so V1/V2 recipes stay unchanged.
	Targets  map[string]Target `yaml:"targets,omitempty" json:"targets,omitempty"`
	Defaults SceneDefaults     `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

type SceneDefaults struct {
	WaitTimeoutMs int   `yaml:"wait_timeout_ms,omitempty" json:"wait_timeout_ms,omitempty"`
	ResultHoldMs  *int  `yaml:"result_hold_ms,omitempty" json:"result_hold_ms,omitempty"`
	Compressible  *bool `yaml:"compressible,omitempty" json:"compressible,omitempty"`
	Instant       *bool `yaml:"instant,omitempty" json:"instant,omitempty"`
}

type Beat struct {
	ID       string  `yaml:"id" json:"id"`
	Sequence []Event `yaml:"sequence" json:"sequence"`
}

type Event struct {
	Speech *SpeechEvent `yaml:"speech,omitempty" json:"speech,omitempty"`
	Action *Action      `yaml:"action,omitempty" json:"action,omitempty"`
	Wait   *WaitEvent   `yaml:"wait,omitempty" json:"wait,omitempty"`
	Hold   *HoldEvent   `yaml:"hold,omitempty" json:"hold,omitempty"`
}

type SpeechEvent struct {
	Text  string `yaml:"text" json:"text"`
	Voice string `yaml:"voice,omitempty" json:"voice,omitempty"`
	// PauseBeforeMs/PauseAfterMs model pacing separately from the text.
	// Content is never rewritten; pauses are orchestration only.
	PauseBeforeMs int `yaml:"pause_before_ms,omitempty" json:"pause_before_ms,omitempty"`
	PauseAfterMs  int `yaml:"pause_after_ms,omitempty" json:"pause_after_ms,omitempty"`
	// Anchor optionally names the visual anchor for this narration
	// (target describe string, scene id, or "viewport"). Empty means
	// the director infers it from surrounding actions.
	Anchor string `yaml:"anchor,omitempty" json:"anchor,omitempty"`
}

type Action struct {
	Type   string  `yaml:"type" json:"type"`
	Target *Target `yaml:"target,omitempty" json:"target,omitempty"`
	URL    string  `yaml:"url,omitempty" json:"url,omitempty"`
	Text   string  `yaml:"text,omitempty" json:"text,omitempty"`
	Value  string  `yaml:"value,omitempty" json:"value,omitempty"`
	Key    string  `yaml:"key,omitempty" json:"key,omitempty"`
	Name   string  `yaml:"name,omitempty" json:"name,omitempty"`
	// SecretRef resolves the fill/type value from the environment at record
	// time ("env:VAR_NAME"). The literal value never lands in artifacts.
	SecretRef string `yaml:"secret_ref,omitempty" json:"secret_ref,omitempty"`
	// Instant forces instant fill for didactic typing (default: progressive).
	Instant *bool `yaml:"instant,omitempty" json:"instant,omitempty"`
	// NoZoom disables camera focus for this action (default: follow visuals).
	NoZoom *bool `yaml:"no_zoom,omitempty" json:"no_zoom,omitempty"`
	// Camera overrides the camera strategy for this action:
	// stay|focus|contextual|none. Empty = semantic default.
	Camera string `yaml:"camera,omitempty" json:"camera,omitempty"`
	// Attention overrides the attention strategy for this action:
	// stay|focus|soft_zoom|pan|pan_zoom|spotlight|highlight|follow|
	// context_restore|none. Empty = semantic default.
	Attention string `yaml:"attention,omitempty" json:"attention,omitempty"`
	// ResultTarget declares the expected visual result of this action
	// (confirmed post-action, held on screen for MinHoldMs).
	ResultTarget *Target `yaml:"result_target,omitempty" json:"result_target,omitempty"`
	// ResultHoldMs overrides the result hold for this action.
	ResultHoldMs *int `yaml:"result_hold_ms,omitempty" json:"result_hold_ms,omitempty"`
	// Callout is an optional short semantic label ("1. Escolha a
	// conversa"). Rendered only when callouts are enabled.
	Callout string `yaml:"callout,omitempty" json:"callout,omitempty"`
	// NoAnticipation disables pre-action anticipation for this action.
	NoAnticipation *bool `yaml:"no_anticipation,omitempty" json:"no_anticipation,omitempty"`
}

type Target struct {
	// Ref is accepted as `target: alias` or `target: {ref: alias}` and resolved
	// against Scene.Targets before the deterministic compiler sees the scene.
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
	TestID string `yaml:"test_id,omitempty" json:"test_id,omitempty"`
	Role   string `yaml:"role,omitempty" json:"role,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Label  string `yaml:"label,omitempty" json:"label,omitempty"`
	Text   string `yaml:"text,omitempty" json:"text,omitempty"`
	CSS    string `yaml:"css,omitempty" json:"css,omitempty"`
}

type WaitEvent struct {
	State     string  `yaml:"state" json:"state"`
	Target    *Target `yaml:"target,omitempty" json:"target,omitempty"`
	URL       string  `yaml:"url,omitempty" json:"url,omitempty"`
	Value     string  `yaml:"value,omitempty" json:"value,omitempty"`
	TimeoutMs int     `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	SettleMs  int     `yaml:"settle_ms,omitempty" json:"settle_ms,omitempty"`
	// Compressible lets the reconciler shorten dead loading time in the
	// final video (default true; narration is never compressed).
	Compressible *bool `yaml:"compressible,omitempty" json:"compressible,omitempty"`
}

func (w *WaitEvent) IsCompressible() bool {
	if w == nil || w.Compressible == nil {
		return true
	}
	return *w.Compressible
}

type HoldEvent struct {
	DurationMs int `yaml:"duration_ms" json:"duration_ms"`
}

type ExpectedState struct {
	Description string  `yaml:"description" json:"description"`
	Target      *Target `yaml:"target,omitempty" json:"target,omitempty"`
	State       string  `yaml:"state" json:"state"`
	URL         string  `yaml:"url,omitempty" json:"url,omitempty"`
	TimeoutMs   int     `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
}

type Redact struct {
	Selectors          []string `yaml:"selectors" json:"selectors"`
	MaskPasswordInputs bool     `yaml:"mask_password_inputs" json:"mask_password_inputs"`
}

var validActionTypes = map[string]bool{
	"goto": true, "click": true, "fill": true, "type": true,
	"press": true, "select": true, "check": true, "uncheck": true,
	"hover": true, "screenshot": true, "scroll": true, "reload": true,
	"goback": true, "expect": true,
}

var validWaitStates = map[string]bool{
	"visible": true, "hidden": true, "attached": true, "detached": true,
	"networkidle": true, "load": true, "url": true, "timeout": true, "settle": true,
}

func (s *Storyboard) Validate() []error {
	var errs []error
	add := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }
	errs = append(errs, s.ExpandCompact()...)
	if s.Version != 1 && s.Version != 2 {
		add("version must be 1 or 2, got %d", s.Version)
	}
	if s.Visuals != nil {
		s.Visuals.ApplyDefaults()
	}
	if s.Cinematic != nil {
		s.Cinematic.ApplyDefaults()
		validateCinematic(s.Cinematic, &errs)
	}
	if strings.TrimSpace(s.Meta.Title) == "" {
		add("meta.title is required")
	}
	if s.Meta.Language == "" {
		add("meta.language is required (e.g. pt-BR)")
	}
	w, h := s.Config.ViewportW, s.Config.ViewportH
	if w != 0 && (w < 320 || w > 7680) {
		add("config.viewport_width out of range: %d", w)
	}
	if h != 0 && (h < 240 || h > 4320) {
		add("config.viewport_height out of range: %d", h)
	}
	if s.Config.BaseURL == "" && s.Setup.StartURL == "" {
		add("either config.base_url or setup.start_url is required")
	}
	if len(s.Scenes) == 0 {
		add("at least one scene is required")
	}
	for i := range s.Setup.Sequence {
		st := &s.Setup.Sequence[i]
		where := fmt.Sprintf("setup.sequence[%d]", i)
		n := 0
		if st.Action != nil {
			n++
		}
		if st.Wait != nil {
			n++
		}
		if n != 1 {
			add("%s: exactly one of action/wait is required, got %d", where, n)
			continue
		}
		if st.Action != nil {
			validateAction(st.Action, where, &errs)
		}
		if st.Wait != nil {
			validateWait(st.Wait, where, &errs)
		}
	}
	seenScenes := map[string]bool{}
	seenBeats := map[string]bool{}
	seenSpeech := map[string]int{}
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		if strings.TrimSpace(sc.ID) == "" {
			add("scenes[%d]: id is required", i)
		} else {
			if seenScenes[sc.ID] {
				add("scenes[%d]: duplicate scene id %q", i, sc.ID)
			}
			seenScenes[sc.ID] = true
		}
		if len(sc.Beats) == 0 {
			add("scene %q: at least one beat is required", sc.ID)
		}
		if sc.Camera != "" && !validCameraOverride[sc.Camera] {
			add("scene %q: unknown camera override %q", sc.ID, sc.Camera)
		}
		if sc.Attention != "" && !validAttentionOverride[sc.Attention] {
			add("scene %q: unknown attention override %q", sc.ID, sc.Attention)
		}
		for j := range sc.Beats {
			b := &sc.Beats[j]
			if strings.TrimSpace(b.ID) == "" {
				add("scene %q beats[%d]: id is required", sc.ID, j)
			} else {
				key := sc.ID + "/" + b.ID
				if seenBeats[key] {
					add("duplicate beat id %q", key)
				}
				seenBeats[key] = true
			}
			if len(b.Sequence) == 0 {
				add("scene %q beat %q: sequence must not be empty", sc.ID, b.ID)
			}
			for k := range b.Sequence {
				ev := &b.Sequence[k]
				where := fmt.Sprintf("scene %q beat %q sequence[%d]", sc.ID, b.ID, k)
				n := 0
				if ev.Speech != nil {
					n++
				}
				if ev.Action != nil {
					n++
				}
				if ev.Wait != nil {
					n++
				}
				if ev.Hold != nil {
					n++
				}
				if n != 1 {
					add("%s: exactly one of speech/action/wait/hold is required, got %d", where, n)
					continue
				}
				if ev.Speech != nil {
					if strings.TrimSpace(ev.Speech.Text) == "" {
						add("%s: speech.text is required", where)
					}
					if len(ev.Speech.Text) > 2000 {
						add("%s: speech.text exceeds 2000 chars", where)
					}
					if ev.Speech.PauseBeforeMs < 0 || ev.Speech.PauseBeforeMs > 5000 {
						add("%s: speech.pause_before_ms out of range 0..5000", where)
					}
					if ev.Speech.PauseAfterMs < 0 || ev.Speech.PauseAfterMs > 5000 {
						add("%s: speech.pause_after_ms out of range 0..5000", where)
					}
					id := SpeechID(sc.ID, b.ID, seenSpeech[sc.ID+"/"+b.ID])
					seenSpeech[sc.ID+"/"+b.ID]++
					_ = id
				}
				if ev.Action != nil {
					validateAction(ev.Action, where, &errs)
				}
				if ev.Wait != nil {
					validateWait(ev.Wait, where, &errs)
				}
				if ev.Hold != nil {
					if ev.Hold.DurationMs < 0 || ev.Hold.DurationMs > 30000 {
						add("%s: hold.duration_ms out of range 0..30000", where)
					}
				}
			}
		}
	}
	return errs
}

// ExpandCompact resolves target aliases and applies scene defaults. It is
// idempotent, intentionally local to a scene, and keeps the compiler's input
// identical to the long-form schema.
func (s *Storyboard) ExpandCompact() []error {
	var errs []error
	for si := range s.Scenes {
		sc := &s.Scenes[si]
		resolve := func(t **Target, where string) {
			if *t == nil || (*t).Ref == "" {
				return
			}
			name := (*t).Ref
			v, ok := sc.Targets[name]
			if !ok {
				errs = append(errs, fmt.Errorf("scene %q %s: unknown target alias %q", sc.ID, where, name))
				return
			}
			if v.Ref != "" {
				errs = append(errs, fmt.Errorf("scene %q target alias %q must not reference another alias", sc.ID, name))
				return
			}
			copy := v
			*t = &copy
		}
		for bi := range sc.Beats {
			for ei := range sc.Beats[bi].Sequence {
				ev := &sc.Beats[bi].Sequence[ei]
				where := fmt.Sprintf("beat %q sequence[%d]", sc.Beats[bi].ID, ei)
				if ev.Action != nil {
					resolve(&ev.Action.Target, where+" action")
					resolve(&ev.Action.ResultTarget, where+" result_target")
					if ev.Action.Instant == nil && sc.Defaults.Instant != nil {
						v := *sc.Defaults.Instant
						ev.Action.Instant = &v
					}
					if ev.Action.ResultHoldMs == nil && sc.Defaults.ResultHoldMs != nil {
						v := *sc.Defaults.ResultHoldMs
						ev.Action.ResultHoldMs = &v
					}
				}
				if ev.Wait != nil {
					resolve(&ev.Wait.Target, where+" wait")
					if ev.Wait.TimeoutMs == 0 && sc.Defaults.WaitTimeoutMs != 0 {
						ev.Wait.TimeoutMs = sc.Defaults.WaitTimeoutMs
					}
					if ev.Wait.Compressible == nil && sc.Defaults.Compressible != nil {
						v := *sc.Defaults.Compressible
						ev.Wait.Compressible = &v
					}
				}
			}
		}
	}
	return errs
}

func validateAction(a *Action, where string, errs *[]error) {
	add := func(f string, args ...any) { *errs = append(*errs, fmt.Errorf(f, args...)) }
	if !validActionTypes[a.Type] {
		add("%s: unknown action type %q", where, a.Type)
	}
	switch a.Type {
	case "goto":
		if a.URL == "" && a.Target == nil {
			add("%s: goto requires url", where)
		}
	case "click", "fill", "type", "hover", "select", "check", "uncheck":
		if a.Target == nil || a.Target.Empty() {
			add("%s: action %q requires target", where, a.Type)
		}
		if a.Type == "fill" || a.Type == "type" {
			if a.Value == "" && a.Text == "" && a.SecretRef == "" {
				add("%s: fill/type requires value, text, or secret_ref", where)
			}
			if a.SecretRef != "" && !strings.HasPrefix(a.SecretRef, "env:") {
				add("%s: secret_ref must be env:VAR_NAME, got %q", where, a.SecretRef)
			}
			if a.SecretRef != "" && (a.Value != "" || a.Text != "") {
				add("%s: secret_ref is exclusive with literal value/text", where)
			}
		}
	case "press":
		if a.Key == "" {
			add("%s: press requires key", where)
		}
	}
	if a.Target != nil && a.Target.CSS != "" &&
		a.Target.TestID == "" && a.Target.Role == "" && a.Target.Label == "" {
		add("%s: warning-class: css-only locator is fragile; prefer test_id/role/label", where)
	}
	if a.Camera != "" && !validCameraOverride[a.Camera] {
		add("%s: unknown camera override %q", where, a.Camera)
	}
	if a.Attention != "" && !validAttentionOverride[a.Attention] {
		add("%s: unknown attention override %q", where, a.Attention)
	}
	if len(a.Callout) > 120 {
		add("%s: callout exceeds 120 chars", where)
	}
	if a.ResultHoldMs != nil && (*a.ResultHoldMs < 0 || *a.ResultHoldMs > 10000) {
		add("%s: result_hold_ms out of range 0..10000", where)
	}
}

var validCameraOverride = map[string]bool{
	"stay": true, "focus": true, "contextual": true, "none": true,
}

var validAttentionOverride = map[string]bool{
	"stay": true, "focus": true, "soft_zoom": true, "pan": true,
	"pan_zoom": true, "spotlight": true, "highlight": true, "follow": true,
	"context_restore": true, "none": true,
}

func validateCinematic(c *visual.CinematicConfig, errs *[]error) {
	add := func(f string, args ...any) { *errs = append(*errs, fmt.Errorf(f, args...)) }
	if c.Camera.MaxZoom < 1 || c.Camera.MaxZoom > 1.5 {
		add("cinematic.camera.max_zoom out of range 1..1.5: %v", c.Camera.MaxZoom)
	}
	if c.Attention.MaxDim < 0 || c.Attention.MaxDim > 0.28 {
		add("cinematic.attention.max_dim out of range 0..0.28: %v", c.Attention.MaxDim)
	}
	if c.Anticipation.RevealMs < 0 || c.Anticipation.RevealMs > 2000 {
		add("cinematic.anticipation.reveal_ms out of range 0..2000")
	}
	if c.Anticipation.SettleMs < 0 || c.Anticipation.SettleMs > 2000 {
		add("cinematic.anticipation.settle_ms out of range 0..2000")
	}
	if c.Results.MinHoldMs < 0 || c.Results.MinHoldMs > 10000 {
		add("cinematic.results.min_hold_ms out of range 0..10000")
	}
	if c.Editing.MaxSpeed < 1 || c.Editing.MaxSpeed > 16 {
		add("cinematic.editing.max_speed out of range 1..16: %v", c.Editing.MaxSpeed)
	}
	if c.Sound.Typing && (c.Sound.Enabled == nil || !*c.Sound.Enabled) {
		add("cinematic.sound.typing requires sound.enabled")
	}
}

func validateWait(w *WaitEvent, where string, errs *[]error) {
	add := func(f string, args ...any) { *errs = append(*errs, fmt.Errorf(f, args...)) }
	if !validWaitStates[strings.ToLower(w.State)] {
		add("%s: unknown wait state %q", where, w.State)
	}
	if w.TimeoutMs < 0 || w.TimeoutMs > 120000 {
		add("%s: wait.timeout_ms out of range 0..120000", where)
	}
	if w.SettleMs < 0 || w.SettleMs > 30000 {
		add("%s: wait.settle_ms out of range 0..30000", where)
	}
}

func SpeechID(sceneID, beatID string, idx int) string {
	return fmt.Sprintf("%s-%s-speech-%03d", sceneID, beatID, idx+1)
}

func (t *Target) Empty() bool {
	if t == nil {
		return true
	}
	return t.Ref == "" && t.TestID == "" && t.Role == "" && t.Name == "" && t.Label == "" && t.Text == "" && t.CSS == ""
}

func (t *Target) Describe() string {
	if t == nil {
		return "<none>"
	}
	switch {
	case t.TestID != "":
		return fmt.Sprintf("test-id=%s", t.TestID)
	case t.Role != "" && t.Name != "":
		return fmt.Sprintf("role=%s name=%q", t.Role, t.Name)
	case t.Role != "":
		return fmt.Sprintf("role=%s", t.Role)
	case t.Label != "":
		return fmt.Sprintf("label=%q", t.Label)
	case t.Text != "":
		return fmt.Sprintf("text=%q", t.Text)
	default:
		return fmt.Sprintf("css=%s", t.CSS)
	}
}

func (t *Target) PlaywrightSelector() string {
	if t == nil {
		return "body"
	}
	switch {
	case t.TestID != "":
		return fmt.Sprintf("[data-testid=%q]", t.TestID)
	case t.Role != "" && t.Name != "":
		return fmt.Sprintf("role=%s[name=%q]", t.Role, t.Name)
	case t.Role != "":
		return fmt.Sprintf("role=%s", t.Role)
	case t.Label != "":
		// No `label=` engine exists; resolve the labeled control through the
		// label's `for` attribute (standard markup) or the first following
		// input (sibling-pattern markup), first in document order wins.
		q := fmt.Sprintf("label[contains(normalize-space(string(.)), %q)]", t.Label)
		return fmt.Sprintf("xpath=(//input[@id=//%s/@for] | //%s/following::input[1])[1]", q, q)
	case t.Text != "":
		return fmt.Sprintf("text=%s", t.Text)
	default:
		return t.CSS
	}
}

func SecretSentinels(text string) []string {
	lower := strings.ToLower(text)
	var found []string
	for _, s := range []string{"password", "passwd", "api_key", "apikey", "secret", "aws_secret", "bearer ", "sk-live", "sk-test", "ghp_", "gho_"} {
		if strings.Contains(lower, s) {
			found = append(found, s)
		}
	}
	return found
}

func (s *Storyboard) VisualsOrDefault() visual.Config {
	if s == nil || s.Visuals == nil {
		return visual.Default()
	}
	c := *s.Visuals
	c.ApplyDefaults()
	return c
}

// CinematicOrDefault resolves the effective AI Director configuration.
// Nil storyboard or absent block yields safe V2 defaults; V1 storyboards
// are unaffected structurally but still get intelligent direction.
func (s *Storyboard) CinematicOrDefault() visual.CinematicConfig {
	if s == nil || s.Cinematic == nil {
		return visual.DefaultCinematic()
	}
	c := *s.Cinematic
	c.ApplyDefaults()
	return c
}

func ResolveSecret(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, "env:")
	if !ok || name == "" {
		return "", fmt.Errorf("secret_ref must be env:VAR_NAME, got %q", ref)
	}
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("secret_ref %q: environment variable %s is empty or unset", ref, name)
	}
	return v, nil
}

func (a *Action) IsInstant() bool { return a != nil && a.Instant != nil && *a.Instant }

func (a *Action) WantsZoom() bool { return a == nil || a.NoZoom == nil || !*a.NoZoom }
