package storyboard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML accepts both the long locator object and the compact scalar
// alias used by scene.targets. Expansion and alias validation happen later.
func (t *Target) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Ref = value.Value
		return nil
	}
	type plain Target
	var v plain
	if err := value.Decode(&v); err != nil {
		return err
	}
	*t = Target(v)
	return nil
}

func LoadFile(path string) (*Storyboard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sb Storyboard
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(false)
	if err := dec.Decode(&sb); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if errs := sb.Validate(); len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, "  - "+e.Error())
		}
		return nil, fmt.Errorf("storyboard %s invalid:\n%s", path, strings.Join(msgs, "\n"))
	}
	return &sb, nil
}

func (s *Storyboard) SourceHash() string {
	h := sha256.New()
	fmt.Fprintf(h, "v2|%d|%s|%s|%s|%dx%d|%s|", s.Version, s.Meta.Title, s.Meta.Language, s.Config.BaseURL, s.Config.ViewportW, s.Config.ViewportH, s.Setup.StartURL)
	if s.Visuals != nil {
		fmt.Fprintf(h, "visuals:%+v|", *s.Visuals)
	}
	if s.Cinematic != nil {
		// Explicit value formatting: %+v would print *bool pointer
		// addresses and make the hash non-deterministic across loads.
		c := *s.Cinematic
		c.ApplyDefaults()
		fmt.Fprintf(h, "cinematic:dir=%t|att=%t|spot=%t|dim=%.3f|cam-cont=%t|cam-restore=%t|maxzoom=%.3f|trans=%d|ant=%t|rev=%d|appr=%d|settle=%d|conf=%t|hold=%d|compress=%t|speed=%.2f|call=%t|maxcall=%d|snd=%t|click=%s|success=%s|typing=%t|qa=%d|%d|%d|",
			c.DirectorOn(), c.AttentionOn(), c.SpotlightOn(), c.Attention.MaxDim,
			c.ContinuityOn(), c.ContextRestoreOn(), c.Camera.MaxZoom, c.Camera.MinTransitionMs,
			c.AnticipationOn(), c.Anticipation.RevealMs, c.Anticipation.ApproachMs, c.Anticipation.SettleMs,
			c.ConfirmationOn(), c.Results.MinHoldMs, c.CompressOn(), c.Editing.MaxSpeed,
			c.CalloutsOn(), c.Callouts.MaxPerScene, c.SoundOn(), c.Sound.Click, c.Sound.Success, c.Sound.Typing,
			c.QA.MaxUnintentionalStaticMs, c.QA.MinTargetVisibleMs, c.QA.MinResultVisibleMs)
	}
	for _, st := range s.Setup.Sequence {
		switch {
		case st.Action != nil:
			fmt.Fprintf(h, "setup-action:%s|%s|%s|%s|%s|%s|secret:%t|", st.Action.Type, targetKey(st.Action.Target), st.Action.URL, st.Action.Text, st.Action.Value, st.Action.Key, st.Action.SecretRef != "")
		case st.Wait != nil:
			fmt.Fprintf(h, "setup-wait:%s|%s|%s|%s|%d|%d|%t|", st.Wait.State, targetKey(st.Wait.Target), st.Wait.URL, st.Wait.Value, st.Wait.TimeoutMs, st.Wait.SettleMs, st.Wait.IsCompressible())
		}
	}
	for _, sc := range s.Scenes {
		fmt.Fprintf(h, "scene:%s|%s|", sc.ID, sc.URL)
		for _, b := range sc.Beats {
			fmt.Fprintf(h, "beat:%s|", b.ID)
			for _, ev := range b.Sequence {
				switch {
				case ev.Speech != nil:
					fmt.Fprintf(h, "speech:%s|%s|%d|%d|%s|", ev.Speech.Text, ev.Speech.Voice, ev.Speech.PauseBeforeMs, ev.Speech.PauseAfterMs, ev.Speech.Anchor)
				case ev.Action != nil:
					a := ev.Action
					fmt.Fprintf(h, "action:%s|%s|%s|%s|%s|%s|secret:%t|instant:%s|zoom:%s|camera:%s|att:%s|result:%s|hold:%s|call:%s|anticipation:%s|", a.Type, targetKey(a.Target), a.URL, a.Text, a.Value, a.Key, a.SecretRef != "", boolKey(a.Instant), boolKey(a.NoZoom), a.Camera, a.Attention, targetKey(a.ResultTarget), intKey(a.ResultHoldMs), a.Callout, boolKey(a.NoAnticipation))
				case ev.Wait != nil:
					fmt.Fprintf(h, "wait:%s|%s|%s|%s|%d|%d|%t|", ev.Wait.State, targetKey(ev.Wait.Target), ev.Wait.URL, ev.Wait.Value, ev.Wait.TimeoutMs, ev.Wait.SettleMs, ev.Wait.IsCompressible())
				case ev.Hold != nil:
					fmt.Fprintf(h, "hold:%d|", ev.Hold.DurationMs)
				}
			}
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:16]
}

func targetKey(t *Target) string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("ref=%s,test=%s,id=%s,role=%s,name=%s,label=%s,text=%s,css=%s", t.Ref, t.TestID, t.ID, t.Role, t.Name, t.Label, t.Text, t.CSS)
}

// SceneHash computes a deterministic content-addressable hash of a single scene:
// its ID, URL, targets, beats, actions, speech, waits, and holds. If a different
// scene in the storyboard changes, this scene's hash remains unchanged, enabling
// granular rebuild and safe video reuse across retakes.
func (sc *Scene) SceneHash() string {
	h := sha256.New()
	fmt.Fprintf(h, "scene:%s|%s|%t|%s|%s|", sc.ID, sc.URL, sc.Screenshot, sc.Camera, sc.Attention)
	var targetNames []string
	for k := range sc.Targets {
		targetNames = append(targetNames, k)
	}
	sort.Strings(targetNames)
	for _, k := range targetNames {
		t := sc.Targets[k]
		fmt.Fprintf(h, "target:%s=%s|", k, targetKey(&t))
	}
	for _, b := range sc.Beats {
		fmt.Fprintf(h, "beat:%s|", b.ID)
		for _, ev := range b.Sequence {
			switch {
			case ev.Speech != nil:
				fmt.Fprintf(h, "speech:%s|%s|%d|%d|%s|", ev.Speech.Text, ev.Speech.Voice, ev.Speech.PauseBeforeMs, ev.Speech.PauseAfterMs, ev.Speech.Anchor)
			case ev.Action != nil:
				a := ev.Action
				fmt.Fprintf(h, "action:%s|%s|%s|%s|%s|%s|secret:%t|instant:%s|zoom:%s|camera:%s|att:%s|result:%s|hold:%s|call:%s|anticipation:%s|", a.Type, targetKey(a.Target), a.URL, a.Text, a.Value, a.Key, a.SecretRef != "", boolKey(a.Instant), boolKey(a.NoZoom), a.Camera, a.Attention, targetKey(a.ResultTarget), intKey(a.ResultHoldMs), a.Callout, boolKey(a.NoAnticipation))
			case ev.Wait != nil:
				fmt.Fprintf(h, "wait:%s|%s|%s|%s|%d|%d|%t|", ev.Wait.State, targetKey(ev.Wait.Target), ev.Wait.URL, ev.Wait.Value, ev.Wait.TimeoutMs, ev.Wait.SettleMs, ev.Wait.IsCompressible())
			case ev.Hold != nil:
				fmt.Fprintf(h, "hold:%d|", ev.Hold.DurationMs)
			}
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:16]
}

func boolKey(v *bool) string {
	if v == nil {
		return "unset"
	}
	if *v {
		return "true"
	}
	return "false"
}

func intKey(v *int) string {
	if v == nil {
		return "unset"
	}
	return fmt.Sprintf("%d", *v)
}

func WriteExample(path string) error {
	ex := `version: 1
meta:
  title: "Gerenciando materiais"
  description: "Tutorial de cadastro e listagem de materiais"
  language: "pt-BR"
  resolution: "1920x1080"
  fps: 30
config:
  base_url: "http://localhost:8099"
  viewport_width: 1280
  viewport_height: 720
setup:
  start_url: "/login"
redact:
  selectors:
    - "[data-sensitive]"
  mask_password_inputs: true
scenes:
  - id: scene-001
    title: "Acesso ao sistema"
    url: "/login"
    screenshot: true
    beats:
      - id: beat-01
        sequence:
          - speech:
              text: "No menu lateral, acesse Materiais."
          - action:
              type: click
              target:
                role: link
                name: Materiais
          - wait:
              state: visible
              target:
                role: heading
                name: Materiais
              timeout_ms: 8000
          - speech:
              text: "Nesta tela encontramos os materiais cadastrados."
          - hold:
              duration_ms: 600
  - id: scene-002
    title: "Novo material"
    url: "/materiais"
    beats:
      - id: beat-01
        sequence:
          - speech:
              text: "Clique em Novo para cadastrar um material."
          - action:
              type: click
              target:
                test_id: new-material-btn
          - wait:
              state: visible
              target:
                role: dialog
              timeout_ms: 8000
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(ex), 0o644)
}
