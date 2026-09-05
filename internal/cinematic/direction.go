package cinematic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// Source names where a resolved direction value came from. Never lost:
// every resolved field carries its origin for explainability.
type Source string

const (
	SourceAction   Source = "user:action"
	SourceScene    Source = "user:scene"
	SourceTutorial Source = "user:tutorial"
	SourcePreset   Source = "preset"
	SourceDirector Source = "director"
)

// ResolvedBeat is the per-beat direction contract after
// defaults + preset + tutorial + scene + action resolution.
// Precedence: action > scene > tutorial > preset > director.
// Unset fields resolve to "auto" with SourceDirector: the director
// fills the gaps, a specific direction never flips unrelated
// controls into manual mode.
type ResolvedBeat struct {
	SceneID string `json:"scene_id"`
	BeatID  string `json:"beat_id"`
	Index   int    `json:"index"`

	Camera      string `json:"camera"`
	Zoom        string `json:"zoom"`
	Cursor      string `json:"cursor"`
	CursorHalo  string `json:"cursor_halo"`
	Click       string `json:"click"`
	ClickEffect string `json:"click_effect"`
	Typing      string `json:"typing"`
	Keyboard    string `json:"keyboard"`
	Spotlight   string `json:"spotlight"`
	Focus       string `json:"focus"`
	Callout     string `json:"callout"`
	Result      string `json:"result"`
	Hold        string `json:"hold"`
	Transition  string `json:"transition"`
	CameraLock  string `json:"camera_lock"`

	Sources map[string]Source `json:"sources"`
}

// UserDirected reports whether any explicit user override (tutorial,
// scene or action scope) contributed to this beat.
func (r ResolvedBeat) UserDirected() bool {
	for _, s := range r.Sources {
		if s == SourceAction || s == SourceScene || s == SourceTutorial {
			return true
		}
	}
	return false
}

func srcOf(s Source) Source { return s }

// Resolve computes the per-beat direction contract for every semantic
// beat. sb may be nil (pure automatic director). Deterministic.
func Resolve(sb *storyboard.Storyboard, beats []BeatPlan) []ResolvedBeat {
	var tutorial *visual.DirectionConfig
	sceneDir := map[string]*visual.DirectionConfig{}
	if sb != nil {
		tutorial = sb.Direction
		for i := range sb.Scenes {
			sc := &sb.Scenes[i]
			if sc.Direction != nil && !sc.Direction.Empty() {
				sceneDir[sc.ID] = sc.Direction
			}
		}
	}
	preset := ""
	if tutorial != nil && tutorial.Preset != "" {
		preset = tutorial.Preset
	}
	presetName := visual.PresetName(preset)
	base := visual.DirectionConfig{Preset: presetName}.WithPreset(presetName)

	// Action-level overrides come from the storyboard: index them by
	// scene|beat|action-type|occurrence, since PlanScenes emits one
	// semantic beat per step and sibling same-type actions share a beat.
	actionDir := map[string]*visual.DirectionConfig{}
	if sb != nil {
		for _, sc := range sb.Scenes {
			for _, b := range sc.Beats {
				occ := map[string]int{}
				for _, ev := range b.Sequence {
					if ev.Action == nil {
						continue
					}
					n := occ[ev.Action.Type]
					occ[ev.Action.Type]++
					if ev.Action.Direction != nil && !ev.Action.Direction.Empty() {
						key := actionOccKey(sc.ID, b.ID, ev.Action.Type, n)
						actionDir[key] = ev.Action.Direction
					}
				}
			}
		}
	}

	out := make([]ResolvedBeat, 0, len(beats))
	seenOcc := map[string]int{}
	for _, bp := range beats {
		merged := base
		sources := map[string]Source{}
		for _, f := range directionFields() {
			sources[f] = SourcePreset
		}
		sources["preset"] = SourcePreset
		if tutorial != nil {
			markSources(merged, tutorial, SourceTutorial, sources)
			merged = merged.Merge(tutorial)
		}
		if sd, ok := sceneDir[bp.SceneID]; ok {
			markSources(merged, sd, SourceScene, sources)
			merged = merged.Merge(sd)
		}
		occKey := bp.SceneID + "|" + bp.BeatID + "|" + bp.ActionType
		n := seenOcc[occKey]
		if bp.ActionType != "" {
			seenOcc[occKey]++
		}
		if ad, ok := actionDir[actionOccKey(bp.SceneID, bp.BeatID, bp.ActionType, n)]; ok {
			markSources(merged, ad, SourceAction, sources)
			merged = merged.Merge(ad)
		}
		fillAuto(&merged, sources)
		merged.Preset = presetName
		out = append(out, ResolvedBeat{
			SceneID: bp.SceneID, BeatID: bp.BeatID, Index: bp.Index,
			Camera: merged.Camera, Zoom: merged.Zoom,
			Cursor: merged.Cursor, CursorHalo: merged.CursorHalo,
			Click: merged.Click, ClickEffect: merged.ClickEffect,
			Typing: merged.Typing, Keyboard: merged.Keyboard,
			Spotlight: merged.Spotlight, Focus: merged.Focus,
			Callout: merged.Callout, Result: merged.Result,
			Hold: merged.Hold, Transition: merged.Transition,
			CameraLock: merged.CameraLock, Sources: sources,
		})
	}
	return out
}

func actionOccKey(scene, beat, actionType string, n int) string {
	return scene + "|" + beat + "|" + actionType + "#" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func directionFields() []string {
	return []string{"camera", "zoom", "cursor", "cursor_halo", "click",
		"click_effect", "typing", "keyboard", "spotlight", "focus",
		"callout", "result", "hold", "transition", "camera_lock"}
}

func eachField(d visual.DirectionConfig) map[string]string {
	return map[string]string{
		"camera": d.Camera, "zoom": d.Zoom, "cursor": d.Cursor,
		"cursor_halo": d.CursorHalo, "click": d.Click,
		"click_effect": d.ClickEffect, "typing": d.Typing,
		"keyboard": d.Keyboard, "spotlight": d.Spotlight, "focus": d.Focus,
		"callout": d.Callout, "result": d.Result, "hold": d.Hold,
		"transition": d.Transition, "camera_lock": d.CameraLock,
	}
}

func markSources(cur visual.DirectionConfig, over *visual.DirectionConfig, src Source, sources map[string]Source) {
	if over == nil {
		return
	}
	for f, v := range eachField(*over) {
		if v != "" {
			sources[f] = src
		}
	}
}

func fillAuto(m *visual.DirectionConfig, sources map[string]Source) {
	set := func(cur *string, field string) {
		if *cur == "" {
			*cur = "auto"
		}
		// "auto" always means the director decides, no matter who wrote it.
		if *cur == "auto" {
			sources[field] = SourceDirector
		}
	}
	set(&m.Camera, "camera")
	set(&m.Zoom, "zoom")
	set(&m.Cursor, "cursor")
	set(&m.CursorHalo, "cursor_halo")
	set(&m.Click, "click")
	set(&m.ClickEffect, "click_effect")
	set(&m.Typing, "typing")
	set(&m.Keyboard, "keyboard")
	set(&m.Spotlight, "spotlight")
	set(&m.Focus, "focus")
	set(&m.Callout, "callout")
	set(&m.Result, "result")
	set(&m.Hold, "hold")
	set(&m.Transition, "transition")
	set(&m.CameraLock, "camera_lock")
}

// ZoomScaleFor maps the resolved zoom intent to a render scale,
// clamped to the cinematic MaxZoom safety ceiling (never above 1.5,
// never cropping the target; enforced again in ApplyCamera).
func ZoomScaleFor(rb *ResolvedBeat, maxZoom float64) float64 {
	if rb == nil {
		return 0
	}
	if rb.Camera == "static" {
		return 1.0
	}
	z := visual.ZoomScale(rb.Zoom)
	if z == 0 {
		return 0 // director decides
	}
	if maxZoom <= 0 {
		maxZoom = 1.25
	}
	if maxZoom > 1.5 {
		maxZoom = 1.5
	}
	if z > maxZoom {
		z = maxZoom
	}
	return z
}

// SpotlightAlphaFor maps resolved spotlight intent to dim alpha.
// Returns -1 when the director decides, 0 when off.
func SpotlightAlphaFor(rb *ResolvedBeat) float64 {
	if rb == nil {
		return -1
	}
	return visual.SpotlightAlpha(rb.Spotlight)
}

// HoldMsFor maps resolved hold intent onto the result-hold floor.
func HoldMsFor(rb *ResolvedBeat, defMs int) int {
	if rb == nil {
		return visual.HoldMs("auto", defMs)
	}
	return visual.HoldMs(rb.Hold, defMs)
}

// --- Natural-language compilation ---

// nlRule maps informal user phrasing (PT + EN, no exact vocabulary
// required) onto a semantic direction fragment. Ordered: positive forms
// first, explicit negatives last so "sem X" wins over a coincidental
// substring ("sem spotlight" contains "spotlight"). neg lists guard
// substrings that veto the rule (narrow contexts like click-only).
type nlRule struct {
	match []string
	neg   []string
	set   func(*visual.DirectionConfig)
}

var nlRules = []nlRule{
	{match: []string{"zoom muito forte", "muito zoom", "extremely strong zoom", "extreme zoom"}, set: func(d *visual.DirectionConfig) { d.Zoom = "extreme" }},
	{match: []string{"bastante zoom", "zoom forte", "strong zoom", "muito próximo", "aproxime bastante", "deixe a tela mais próxima", "tela mais próxima", "close up", "close-up", "zoom in a lot", "focado", "focada", "em foco"}, set: func(d *visual.DirectionConfig) { d.Zoom = "strong" }},
	{match: []string{"zoom discreto", "zoom leve", "zoom sutil", "pouco zoom", "subtle zoom", "slight zoom"}, set: func(d *visual.DirectionConfig) { d.Zoom = "subtle" }},
	{match: []string{"zoom normal", "normal zoom", "medium zoom"}, set: func(d *visual.DirectionConfig) { d.Zoom = "medium" }},
	{match: []string{"sem zoom", "não use zoom", "não usar zoom", "sem usar zoom", "no zoom", "without zoom", "disable zoom", "zoom off"}, set: func(d *visual.DirectionConfig) { d.Zoom = "off" }},
	{match: []string{"não mova a câmera", "não mover a câmera", "câmera fixa", "camera fixa", "câmera estática", "static camera", "don't move the camera", "fixed camera", "no camera movement", "sem muitos movimentos", "sem movimento"}, set: func(d *visual.DirectionConfig) { d.Camera = "static" }},
	{match: []string{"mantenha a câmera", "mantenha o foco", "manter a câmera", "keep the camera", "lock the camera", "mantenha focado", "foco até", "focado", "focada", "camera lock", "shot estável", "plano estável"}, set: func(d *visual.DirectionConfig) { d.CameraLock = "on"; d.Camera = "focus" }},
	{match: []string{"comece aberto", "tela inteira", "visão completa", "full screen", "full view", "wide shot", "start wide"}, set: func(d *visual.DirectionConfig) { d.Camera = "wide" }},
	{match: []string{"mostre o cursor", "mostrar o cursor", "show the cursor", "keep the cursor", "cursor visible", "sempre mostre o cursor"}, set: func(d *visual.DirectionConfig) { d.Cursor = "show" }},
	{match: []string{"não mostre o cursor", "não mostrar o cursor", "sem cursor", "esconda o cursor", "hide the cursor", "no cursor", "without cursor", "cursor off"}, set: func(d *visual.DirectionConfig) { d.Cursor = "hide" }},
	{match: []string{"clique bem", "cliques bem", "bem visíveis", "bem visível", "evidente", "make clicks", "clicks easier to see", "emphasize clicks", "highlight clicks", "destaque os cliques", "destaque todos os cliques"}, set: func(d *visual.DirectionConfig) { d.Click = "strong" }},
	{match: []string{"sem efeitos no clique", "sem efeito no clique", "no click effect", "click off", "clique desligado"}, set: func(d *visual.DirectionConfig) { d.Click = "off" }},
	{match: []string{"todas as teclas", "all keys", "todas teclas", "sempre mostre as teclas"}, set: func(d *visual.DirectionConfig) { d.Keyboard = "all" }},
	{match: []string{"atalhos", "atalho", "shortcut", "enter", "tecla na tela", "tecla", "keyboard", "mostre a tecla", "indicação visual quando"}, set: func(d *visual.DirectionConfig) { d.Keyboard = "shortcuts" }},
	{match: []string{"não mostre o teclado", "sem teclado", "no keyboard", "keyboard off", "sem overlay de teclado"}, set: func(d *visual.DirectionConfig) { d.Keyboard = "off" }},
	{match: []string{"escureça", "escurecer", "destaque apenas", "spotlight", "dim the background", "dim background"}, set: func(d *visual.DirectionConfig) { d.Spotlight = "medium" }},
	{match: []string{"não use spotlight", "sem spotlight", "no spotlight", "spotlight off", "sem escurecer"}, set: func(d *visual.DirectionConfig) { d.Spotlight = "off" }},
	{match: []string{"callout", "explicando", "explique", "balão", "etiqueta", "explaining"}, set: func(d *visual.DirectionConfig) { d.Callout = "on" }},
	{match: []string{"não use callout", "sem callout", "no callout", "callout off"}, set: func(d *visual.DirectionConfig) { d.Callout = "off" }},
	{match: []string{"pulse", "pulsar", "pulsação"}, set: func(d *visual.DirectionConfig) { d.Focus = "pulse" }},
	{match: []string{"destaque o botão", "destaque nela", "destaque nesse botão", "outline", "borda", "destaque no resultado", "destaque o resultado", "destaque a mensagem", "sempre destaque o resultado", "result highlight"}, set: func(d *visual.DirectionConfig) { d.Focus = "outline"; d.Result = "emphasize" }},
	{match: []string{"sempre destaque", "emphasize the result", "result emphasis"}, set: func(d *visual.DirectionConfig) { d.Result = "emphasize" }},
	{match: []string{"sem resultado destacado", "no result emphasis", "result off"}, set: func(d *visual.DirectionConfig) { d.Result = "off" }},
	{match: []string{"pare um pouco", "tempo de ler", "segure", "segurar", "mais tempo", "hold longer", "freeze", "leitura"}, set: func(d *visual.DirectionConfig) { d.Hold = "long" }},
	{match: []string{"mais rápido", "acelere", "faster", "speed up", "trecho mais rápido"}, set: func(d *visual.DirectionConfig) { d.Hold = "short" }},
	{match: []string{"digitação lenta", "digite devagar", "slow typing", "type slowly"}, set: func(d *visual.DirectionConfig) { d.Typing = "slow" }},
	{match: []string{"digitação rápida", "fast typing", "type fast"}, set: func(d *visual.DirectionConfig) { d.Typing = "fast" }},
	{match: []string{"digitação instantânea", "instant typing", "sem digitação progressiva"}, set: func(d *visual.DirectionConfig) { d.Typing = "instant" }},
	{match: []string{"digitação natural", "natural typing"}, set: func(d *visual.DirectionConfig) { d.Typing = "natural" }},
	{match: []string{"bem simples", "sem efeitos", "minimalista", "minimal", "mais calmo", "calm", "calma", "simples"}, neg: []string{"clique", "click"}, set: func(d *visual.DirectionConfig) { d.Preset = "minimal" }},
	{match: []string{"dinâmico", "dinamico", "dynamic", "mais dinâmico"}, set: func(d *visual.DirectionConfig) { d.Preset = "dynamic" }},
	{match: []string{"treinamento", "training", "onboarding", "passo a passo", "tutorial passo"}, set: func(d *visual.DirectionConfig) { d.Preset = "training" }},
	{match: []string{"transição suave", "smooth transition", "transicao suave"}, set: func(d *visual.DirectionConfig) { d.Transition = "smooth" }},
	{match: []string{"corte seco", "sem transição", "hard cut", "no transition"}, set: func(d *visual.DirectionConfig) { d.Transition = "cut" }},
}

// CompileNL turns a free-form user request (PT or EN) into a tutorial
// level DirectionConfig. The agent normalizes vocabulary; unknown text
// yields an empty (all-auto) config, never an error.
func CompileNL(text string) visual.DirectionConfig {
	var d visual.DirectionConfig
	lower := strings.ToLower(text)
	for _, r := range nlRules {
		vetoed := false
		for _, n := range r.neg {
			if strings.Contains(lower, n) {
				vetoed = true
				break
			}
		}
		if vetoed {
			continue
		}
		for _, m := range r.match {
			if strings.Contains(lower, m) {
				r.set(&d)
				break
			}
		}
	}
	return d
}

// FormatResolved renders the resolved direction for explain/CLI output,
// with per-field sources (user vs director).
func FormatResolved(res []ResolvedBeat) string {
	var b strings.Builder
	byScene := map[string][]ResolvedBeat{}
	var scenes []string
	for _, r := range res {
		if _, ok := byScene[r.SceneID]; !ok {
			scenes = append(scenes, r.SceneID)
		}
		byScene[r.SceneID] = append(byScene[r.SceneID], r)
	}
	sort.Strings(scenes)
	for _, sc := range scenes {
		fmt.Fprintf(&b, "%s\n", sc)
		seen := map[string]bool{}
		for _, r := range byScene[sc] {
			for f, v := range eachField(resolvedToConfig(r)) {
				key := f + "=" + v
				if seen[key] {
					continue
				}
				seen[key] = true
				fmt.Fprintf(&b, "  %s=%s [%s]\n", f, v, r.Sources[f])
			}
		}
	}
	return b.String()
}

func resolvedToConfig(r ResolvedBeat) visual.DirectionConfig {
	return visual.DirectionConfig{
		Camera: r.Camera, Zoom: r.Zoom, Cursor: r.Cursor,
		CursorHalo: r.CursorHalo, Click: r.Click, ClickEffect: r.ClickEffect,
		Typing: r.Typing, Keyboard: r.Keyboard, Spotlight: r.Spotlight,
		Focus: r.Focus, Callout: r.Callout, Result: r.Result,
		Hold: r.Hold, Transition: r.Transition, CameraLock: r.CameraLock,
	}
}
