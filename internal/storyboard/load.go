package storyboard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

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
		fmt.Fprintf(h, "cinematic:%+v|", *s.Cinematic)
	}
	for _, st := range s.Setup.Sequence {
		switch {
		case st.Action != nil:
			t := ""
			if st.Action.Target != nil {
				t = st.Action.Target.PlaywrightSelector()
			}
			fmt.Fprintf(h, "setup-action:%s|%s|%s|secret:%t|", st.Action.Type, t, st.Action.URL, st.Action.SecretRef != "")
		case st.Wait != nil:
			fmt.Fprintf(h, "setup-wait:%s|%s|%d|%d|", st.Wait.State, st.Wait.URL, st.Wait.TimeoutMs, st.Wait.SettleMs)
		}
	}
	for _, sc := range s.Scenes {
		fmt.Fprintf(h, "scene:%s|%s|", sc.ID, sc.URL)
		for _, b := range sc.Beats {
			fmt.Fprintf(h, "beat:%s|", b.ID)
			for _, ev := range b.Sequence {
				switch {
				case ev.Speech != nil:
					fmt.Fprintf(h, "speech:%s|%s|", ev.Speech.Text, ev.Speech.Voice)
				case ev.Action != nil:
					t := ""
					if ev.Action.Target != nil {
						t = ev.Action.Target.PlaywrightSelector()
					}
					fmt.Fprintf(h, "action:%s|%s|%s|%s|%s|%s|secret:%t|", ev.Action.Type, t, ev.Action.URL, ev.Action.Text, ev.Action.Value, ev.Action.Key, ev.Action.SecretRef != "")
				case ev.Wait != nil:
					fmt.Fprintf(h, "wait:%s|%s|%s|%d|%d|", ev.Wait.State, ev.Wait.URL, ev.Wait.Value, ev.Wait.TimeoutMs, ev.Wait.SettleMs)
				case ev.Hold != nil:
					fmt.Fprintf(h, "hold:%d|", ev.Hold.DurationMs)
				}
			}
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:16]
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
