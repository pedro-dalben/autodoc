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
	Scenes  []Scene        `yaml:"scenes" json:"scenes"`
	Redact  Redact         `yaml:"redact" json:"redact"`
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
}

type Target struct {
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
	if s.Version != 1 && s.Version != 2 {
		add("version must be 1 or 2, got %d", s.Version)
	}
	if s.Visuals != nil {
		s.Visuals.ApplyDefaults()
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
	return t.TestID == "" && t.Role == "" && t.Name == "" && t.Label == "" && t.Text == "" && t.CSS == ""
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
		return fmt.Sprintf("label=%s", t.Label)
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
