package storyboard

import (
	"fmt"
	"strings"
)

type Storyboard struct {
	Version int     `yaml:"version" json:"version"`
	Meta    Meta    `yaml:"meta" json:"meta"`
	Config  Config  `yaml:"config" json:"config"`
	Setup   Setup   `yaml:"setup" json:"setup"`
	Scenes  []Scene `yaml:"scenes" json:"scenes"`
	Redact  Redact  `yaml:"redact" json:"redact"`
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
	Preconditions  []SetupAction   `yaml:"preconditions" json:"preconditions"`
	ExpectedStates []ExpectedState `yaml:"expected_states" json:"expected_states"`
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
	TimeoutMs int     `yaml:"timeout_ms,omitempty" json:"timeout_ms,omitempty"`
	SettleMs  int     `yaml:"settle_ms,omitempty" json:"settle_ms,omitempty"`
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
	if s.Version != 1 {
		add("version must be 1, got %d", s.Version)
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
					if !validActionTypes[ev.Action.Type] {
						add("%s: unknown action type %q", where, ev.Action.Type)
					}
					switch ev.Action.Type {
					case "goto":
						if ev.Action.URL == "" && sc.URL == "" && s.Setup.StartURL == "" {
							add("%s: goto requires url", where)
						}
					case "click", "fill", "type", "hover", "select", "check", "uncheck":
						if ev.Action.Target == nil || ev.Action.Target.Empty() {
							add("%s: action %q requires target", where, ev.Action.Type)
						}
						if ev.Action.Type == "fill" || ev.Action.Type == "type" {
							if ev.Action.Value == "" && ev.Action.Text == "" {
								add("%s: fill/type requires value or text", where)
							}
						}
					case "press":
						if ev.Action.Key == "" {
							add("%s: press requires key", where)
						}
					}
					if ev.Action.Target != nil && ev.Action.Target.CSS != "" &&
						ev.Action.Target.TestID == "" && ev.Action.Target.Role == "" && ev.Action.Target.Label == "" {
						add("%s: warning-class: css-only locator is fragile; prefer test_id/role/label", where)
					}
				}
				if ev.Wait != nil {
					if !validWaitStates[strings.ToLower(ev.Wait.State)] {
						add("%s: unknown wait state %q", where, ev.Wait.State)
					}
					if ev.Wait.TimeoutMs < 0 || ev.Wait.TimeoutMs > 120000 {
						add("%s: wait.timeout_ms out of range 0..120000", where)
					}
					if ev.Wait.SettleMs < 0 || ev.Wait.SettleMs > 30000 {
						add("%s: wait.settle_ms out of range 0..30000", where)
					}
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
