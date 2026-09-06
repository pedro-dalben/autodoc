package visual

import (
	"math"
	"strings"
)

type CursorConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	Smooth   bool `yaml:"smooth" json:"smooth"`
	MinMs    int  `yaml:"min_ms" json:"min_ms"`
	MaxMs    int  `yaml:"max_ms" json:"max_ms"`
	ParkX    int  `yaml:"park_x" json:"park_x"`
	ParkY    int  `yaml:"park_y" json:"park_y"`
	ParkMs   int  `yaml:"park_ms" json:"park_ms"`
	Disabled bool `yaml:"disabled" json:"disabled"`
	// Halo draws a discreet ring around the cursor (training/onboarding).
	Halo bool `yaml:"halo" json:"halo"`
}

type ClickConfig struct {
	Ripple    bool `yaml:"ripple" json:"ripple"`
	Highlight bool `yaml:"highlight" json:"highlight"`
	RippleMs  int  `yaml:"ripple_ms" json:"ripple_ms"`
	Disabled  bool `yaml:"disabled" json:"disabled"`
}

type TypingConfig struct {
	Progressive bool `yaml:"progressive" json:"progressive"`
	CharDelayMs int  `yaml:"char_delay_ms" json:"char_delay_ms"`
	Highlight   bool `yaml:"highlight" json:"highlight"`
	FocusZoom   bool `yaml:"focus_zoom" json:"focus_zoom"`
	Disabled    bool `yaml:"disabled" json:"disabled"`
}

type CameraConfig struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	ClickZoom    float64 `yaml:"click_zoom" json:"click_zoom"`
	TypingZoom   float64 `yaml:"typing_zoom" json:"typing_zoom"`
	FormZoom     float64 `yaml:"form_zoom" json:"form_zoom"`
	MaxZoom      float64 `yaml:"max_zoom" json:"max_zoom"`
	TransitionMs int     `yaml:"transition_ms" json:"transition_ms"`
	MinTargetPx  int     `yaml:"min_target_px" json:"min_target_px"`
	MaxCover     float64 `yaml:"max_cover" json:"max_cover"`
	Disabled     bool    `yaml:"disabled" json:"disabled"`
}

type PacingConfig struct {
	SpeechActionGapMs int `yaml:"speech_action_gap_ms" json:"speech_action_gap_ms"`
	ActionSpeechGapMs int `yaml:"action_speech_gap_ms" json:"action_speech_gap_ms"`
	HighlightMs       int `yaml:"highlight_ms" json:"highlight_ms"`
	ClickSettleMs     int `yaml:"click_settle_ms" json:"click_settle_ms"`
}

type Tolerances struct {
	SpeechBoundaryMs int `yaml:"speech_boundary_ms" json:"speech_boundary_ms"`
	ActionCueMs      int `yaml:"action_cue_ms" json:"action_cue_ms"`
	FinalAVMs        int `yaml:"final_av_ms" json:"final_av_ms"`
}

// CinematicConfig is the AI Director / automatic editor configuration.
// All fields are optional; nil *Config-level pointer in the storyboard
// means "safe defaults". Every sub-block defaults to conservative,
// SaaS-tutorial-safe behavior: director on, callouts off, sound off,
// max zoom 1.25, subtle spotlight only.
type CinematicConfig struct {
	// Director is the master switch. Nil/true = enabled with safe
	// defaults; explicit false disables all V2 direction (pure V1).
	Director *bool `yaml:"director,omitempty" json:"director,omitempty"`
	// Attention controls the Attention Director (focus/spotlight/etc).
	Attention AttentionConfig `yaml:"attention,omitempty" json:"attention,omitempty"`
	// Camera controls Camera Director V2 (continuity, context restore).
	Camera DirectorCameraConfig `yaml:"camera,omitempty" json:"camera,omitempty"`
	// Anticipation controls pre-action anticipation envelopes.
	Anticipation AnticipationConfig `yaml:"anticipation,omitempty" json:"anticipation,omitempty"`
	// Results controls action→result→confirmation and result holds.
	Results ResultsConfig `yaml:"results,omitempty" json:"results,omitempty"`
	// Editing controls dead-time classification and compression.
	Editing EditingConfig `yaml:"editing,omitempty" json:"editing,omitempty"`
	// Callouts controls the optional semantic callout engine.
	Callouts CalloutConfig `yaml:"callouts,omitempty" json:"callouts,omitempty"`
	// Sound is optional sound-design infrastructure (default off).
	Sound SoundConfig `yaml:"sound,omitempty" json:"sound,omitempty"`
	// QA thresholds for `validate --cinematic`.
	QA QAConfig `yaml:"qa,omitempty" json:"qa,omitempty"`
}

type AttentionConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Spotlight enables the subtle de-emphasis overlay (default true,
	// but only applied when semantically justified and safe).
	Spotlight *bool `yaml:"spotlight,omitempty" json:"spotlight,omitempty"`
	// MaxDim is the spotlight dim alpha 0..1 (default 0.18, max 0.28).
	MaxDim float64 `yaml:"max_dim,omitempty" json:"max_dim,omitempty"`
}

type DirectorCameraConfig struct {
	// Continuity enables cross-scene camera continuity heuristics.
	Continuity *bool `yaml:"continuity,omitempty" json:"continuity,omitempty"`
	// ContextRestore enables automatic zoom-out after focused actions.
	ContextRestore *bool `yaml:"context_restore,omitempty" json:"context_restore,omitempty"`
	// MaxZoom caps render-time zoom (default 1.25, never above 1.5).
	MaxZoom float64 `yaml:"max_zoom,omitempty" json:"max_zoom,omitempty"`
	// MinTransitionMs floors camera transitions (default 250).
	MinTransitionMs int `yaml:"min_transition_ms,omitempty" json:"min_transition_ms,omitempty"`
}

type AnticipationConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// RevealMs: target highlight before cursor arrives (default 350).
	RevealMs int `yaml:"reveal_ms,omitempty" json:"reveal_ms,omitempty"`
	// ApproachMs cap for cursor approach (default 500; cursor config
	// min/max still apply).
	ApproachMs int `yaml:"approach_ms,omitempty" json:"approach_ms,omitempty"`
	// SettleMs: stabilization before the action (default 200).
	SettleMs int `yaml:"settle_ms,omitempty" json:"settle_ms,omitempty"`
}

type ResultsConfig struct {
	// Confirmation enables result detection + hold + highlight.
	Confirmation *bool `yaml:"confirmation,omitempty" json:"confirmation,omitempty"`
	// MinHoldMs floors result visibility (default 1000).
	MinHoldMs int `yaml:"min_hold_ms,omitempty" json:"min_hold_ms,omitempty"`
}

type EditingConfig struct {
	// CompressDeadTime enables smart wait compression (default true).
	CompressDeadTime *bool `yaml:"compress_dead_time,omitempty" json:"compress_dead_time,omitempty"`
	// MaxSpeed caps wait fast-forward (default 6).
	MaxSpeed float64 `yaml:"max_speed,omitempty" json:"max_speed,omitempty"`
}

type CalloutConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// MaxPerScene caps callouts per scene (default 3).
	MaxPerScene int `yaml:"max_per_scene,omitempty" json:"max_per_scene,omitempty"`
}

type SoundConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Click/success are intensity hints: "", "subtle". Default "" (off).
	Click   string `yaml:"click,omitempty" json:"click,omitempty"`
	Success string `yaml:"success,omitempty" json:"success,omitempty"`
	// Typing sounds are never enabled by default.
	Typing bool `yaml:"typing,omitempty" json:"typing,omitempty"`
}

type QAConfig struct {
	// MaxUnintentionalStaticMs caps unintentional static speech windows.
	MaxUnintentionalStaticMs int `yaml:"max_unintentional_static_ms,omitempty" json:"max_unintentional_static_ms,omitempty"`
	// MinTargetVisibleMs floors target visibility before action.
	MinTargetVisibleMs int `yaml:"min_target_visible_ms,omitempty" json:"min_target_visible_ms,omitempty"`
	// MinResultVisibleMs floors result visibility after action.
	MinResultVisibleMs int `yaml:"min_result_visible_ms,omitempty" json:"min_result_visible_ms,omitempty"`
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func (c *CinematicConfig) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.Attention.MaxDim == 0 {
		c.Attention.MaxDim = 0.18
	}
	if c.Attention.MaxDim > 0.28 {
		c.Attention.MaxDim = 0.28
	}
	if c.Camera.MaxZoom == 0 {
		c.Camera.MaxZoom = 1.25
	}
	if c.Camera.MaxZoom > 1.5 {
		c.Camera.MaxZoom = 1.5
	}
	if c.Camera.MinTransitionMs == 0 {
		c.Camera.MinTransitionMs = 250
	}
	if c.Anticipation.RevealMs == 0 {
		c.Anticipation.RevealMs = 350
	}
	if c.Anticipation.ApproachMs == 0 {
		c.Anticipation.ApproachMs = 500
	}
	if c.Anticipation.SettleMs == 0 {
		c.Anticipation.SettleMs = 200
	}
	if c.Results.MinHoldMs == 0 {
		c.Results.MinHoldMs = 1000
	}
	if c.Editing.MaxSpeed == 0 {
		c.Editing.MaxSpeed = 6
	}
	if c.Callouts.MaxPerScene == 0 {
		c.Callouts.MaxPerScene = 3
	}
	if c.QA.MaxUnintentionalStaticMs == 0 {
		c.QA.MaxUnintentionalStaticMs = 2500
	}
	if c.QA.MinTargetVisibleMs == 0 {
		c.QA.MinTargetVisibleMs = 400
	}
	if c.QA.MinResultVisibleMs == 0 {
		c.QA.MinResultVisibleMs = 800
	}
}

// DefaultCinematic returns safe V2 defaults (director on, callouts off,
// sound off). Used when the storyboard carries no cinematic block.
func DefaultCinematic() CinematicConfig {
	c := CinematicConfig{}
	c.ApplyDefaults()
	return c
}

// EffectiveCinematic resolves nil (absent block) to safe defaults.
func EffectiveCinematic(c *CinematicConfig) CinematicConfig {
	if c == nil {
		return DefaultCinematic()
	}
	out := *c
	out.ApplyDefaults()
	return out
}

func (c CinematicConfig) DirectorOn() bool { return boolOr(c.Director, true) }
func (c CinematicConfig) AttentionOn() bool {
	return c.DirectorOn() && boolOr(c.Attention.Enabled, true)
}
func (c CinematicConfig) SpotlightOn() bool {
	return c.AttentionOn() && boolOr(c.Attention.Spotlight, true)
}
func (c CinematicConfig) ContinuityOn() bool {
	return c.DirectorOn() && boolOr(c.Camera.Continuity, true)
}
func (c CinematicConfig) ContextRestoreOn() bool {
	return c.DirectorOn() && boolOr(c.Camera.ContextRestore, true)
}
func (c CinematicConfig) AnticipationOn() bool {
	return c.DirectorOn() && boolOr(c.Anticipation.Enabled, true)
}
func (c CinematicConfig) ConfirmationOn() bool {
	return c.DirectorOn() && boolOr(c.Results.Confirmation, true)
}
func (c CinematicConfig) CompressOn() bool {
	return c.DirectorOn() && boolOr(c.Editing.CompressDeadTime, true)
}
func (c CinematicConfig) CalloutsOn() bool {
	return c.DirectorOn() && boolOr(c.Callouts.Enabled, false)
}
func (c CinematicConfig) SoundOn() bool { return boolOr(c.Sound.Enabled, false) }

type Config struct {
	Cursor    CursorConfig    `yaml:"cursor" json:"cursor"`
	Click     ClickConfig     `yaml:"click" json:"click"`
	Typing    TypingConfig    `yaml:"typing" json:"typing"`
	Camera    CameraConfig    `yaml:"camera" json:"camera"`
	Pacing    PacingConfig    `yaml:"pacing" json:"pacing"`
	SyncTol   Tolerances      `yaml:"sync" json:"sync"`
	Cinematic CinematicConfig `yaml:"cinematic,omitempty" json:"cinematic,omitempty"`
}

func Default() Config {
	return Config{
		Cursor:  CursorConfig{Enabled: true, Smooth: true, MinMs: 200, MaxMs: 450, ParkX: -1, ParkY: -1, ParkMs: 200},
		Click:   ClickConfig{Ripple: true, Highlight: true, RippleMs: 550},
		Typing:  TypingConfig{Progressive: true, CharDelayMs: 45, Highlight: true, FocusZoom: true},
		Camera:  CameraConfig{Enabled: true, ClickZoom: 1.08, TypingZoom: 1.15, FormZoom: 1.12, MaxZoom: 1.25, TransitionMs: 300, MinTargetPx: 24, MaxCover: 0.6},
		Pacing:  PacingConfig{SpeechActionGapMs: 250, ActionSpeechGapMs: 450, HighlightMs: 120, ClickSettleMs: 350},
		SyncTol: Tolerances{SpeechBoundaryMs: 100, ActionCueMs: 150, FinalAVMs: 250},
	}
}

func (c *Config) ApplyDefaults() {
	d := Default()
	if c.Cursor.MinMs == 0 {
		c.Cursor.MinMs = d.Cursor.MinMs
	}
	if c.Cursor.MaxMs == 0 {
		c.Cursor.MaxMs = d.Cursor.MaxMs
	}
	if c.Cursor.ParkMs == 0 {
		c.Cursor.ParkMs = d.Cursor.ParkMs
	}
	if !c.Cursor.Disabled && !c.Cursor.Enabled && !c.Click.Disabled {
		c.Cursor.Enabled = true
	}
	if c.Click.RippleMs == 0 {
		c.Click.RippleMs = d.Click.RippleMs
	}
	if !c.Click.Disabled && !c.Click.Ripple && !c.Click.Highlight {
		c.Click.Ripple, c.Click.Highlight = true, true
	}
	if c.Typing.CharDelayMs == 0 {
		c.Typing.CharDelayMs = d.Typing.CharDelayMs
	}
	if c.Typing.CharDelayMs < 30 {
		c.Typing.CharDelayMs = 30
	}
	if c.Typing.CharDelayMs > 80 {
		c.Typing.CharDelayMs = 80
	}
	if !c.Typing.Disabled && !c.Typing.Progressive && !c.Typing.Highlight && !c.Typing.FocusZoom {
		c.Typing.Progressive, c.Typing.Highlight, c.Typing.FocusZoom = true, true, true
	}
	if c.Camera.ClickZoom == 0 {
		c.Camera.ClickZoom = d.Camera.ClickZoom
	}
	if c.Camera.TypingZoom == 0 {
		c.Camera.TypingZoom = d.Camera.TypingZoom
	}
	if c.Camera.FormZoom == 0 {
		c.Camera.FormZoom = d.Camera.FormZoom
	}
	if c.Camera.MaxZoom == 0 {
		c.Camera.MaxZoom = d.Camera.MaxZoom
	}
	if c.Camera.TransitionMs == 0 {
		c.Camera.TransitionMs = d.Camera.TransitionMs
	}
	if c.Camera.MinTargetPx == 0 {
		c.Camera.MinTargetPx = d.Camera.MinTargetPx
	}
	if c.Camera.MaxCover == 0 {
		c.Camera.MaxCover = d.Camera.MaxCover
	}
	if !c.Camera.Disabled && !c.Camera.Enabled {
		c.Camera.Enabled = true
	}
	if c.Pacing.SpeechActionGapMs == 0 {
		c.Pacing.SpeechActionGapMs = d.Pacing.SpeechActionGapMs
	}
	if c.Pacing.ActionSpeechGapMs == 0 {
		c.Pacing.ActionSpeechGapMs = d.Pacing.ActionSpeechGapMs
	}
	if c.Pacing.HighlightMs == 0 {
		c.Pacing.HighlightMs = d.Pacing.HighlightMs
	}
	if c.Pacing.ClickSettleMs == 0 {
		c.Pacing.ClickSettleMs = d.Pacing.ClickSettleMs
	}
	if c.SyncTol.SpeechBoundaryMs == 0 {
		c.SyncTol.SpeechBoundaryMs = d.SyncTol.SpeechBoundaryMs
	}
	if c.SyncTol.ActionCueMs == 0 {
		c.SyncTol.ActionCueMs = d.SyncTol.ActionCueMs
	}
	if c.SyncTol.FinalAVMs == 0 {
		c.SyncTol.FinalAVMs = d.SyncTol.FinalAVMs
	}
}

func (c Config) CursorOn() bool { return c.Cursor.Enabled && !c.Cursor.Disabled }
func (c Config) ClickOn() bool  { return !c.Click.Disabled }
func (c Config) TypingOn() bool { return !c.Typing.Disabled }
func (c Config) CameraOn() bool { return c.Camera.Enabled && !c.Camera.Disabled }
func (c Config) RippleOn() bool { return c.ClickOn() && c.Click.Ripple }
func (c Config) HiliteOn() bool { return c.ClickOn() && c.Click.Highlight }

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (b BBox) Valid() bool { return b.Width > 0 && b.Height > 0 }

func (b BBox) Center() Point { return Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2} }

func (b BBox) Coverage(vw, vh float64) float64 {
	if vw <= 0 || vh <= 0 {
		return 1
	}
	return (b.Width * b.Height) / (vw * vh)
}

func EaseInOut(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func CursorMoveMs(from, to Point, cfg CursorConfig) int {
	dx, dy := to.X-from.X, to.Y-from.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist < 2 {
		return 0
	}
	ms := int(180 + dist*0.25)
	if ms < cfg.MinMs {
		ms = cfg.MinMs
	}
	if ms > cfg.MaxMs {
		ms = cfg.MaxMs
	}
	return ms
}

func CursorAt(from, to Point, elapsedMs, totalMs int) Point {
	if totalMs <= 0 || elapsedMs >= totalMs {
		return to
	}
	if elapsedMs <= 0 {
		return from
	}
	k := EaseInOut(float64(elapsedMs) / float64(totalMs))
	return Point{X: from.X + (to.X-from.X)*k, Y: from.Y + (to.Y-from.Y)*k}
}

type CameraShot struct {
	Zoom   float64 `json:"zoom"`
	CropX  float64 `json:"crop_x"`
	CropY  float64 `json:"crop_y"`
	CropW  float64 `json:"crop_w"`
	CropH  float64 `json:"crop_h"`
	Apply  bool    `json:"apply"`
	Reason string  `json:"reason,omitempty"`
}

func PlanCamera(bbox BBox, vw, vh int, wantZoom float64, cfg CameraConfig) CameraShot {
	W, H := float64(vw), float64(vh)
	if !bbox.Valid() || W <= 0 || H <= 0 {
		return CameraShot{Zoom: 1, Apply: false, Reason: "no-bbox"}
	}
	if bbox.Width < float64(cfg.MinTargetPx) && bbox.Height < float64(cfg.MinTargetPx) {
		return CameraShot{Zoom: 1, Apply: false, Reason: "target-too-small"}
	}
	if bbox.Coverage(W, H) > cfg.MaxCover {
		return CameraShot{Zoom: 1, Apply: false, Reason: "target-too-large"}
	}
	z := wantZoom
	if z < 1 {
		z = 1
	}
	if z > cfg.MaxZoom {
		z = cfg.MaxZoom
	}
	if z <= 1.01 {
		return CameraShot{Zoom: 1, Apply: false, Reason: "zoom-negligible"}
	}
	cw, ch := W/z, H/z
	c := bbox.Center()
	cx, cy := c.X-cw/2, c.Y-ch/2
	if cx < 0 {
		cx = 0
	}
	if cy < 0 {
		cy = 0
	}
	if cx+cw > W {
		cx = W - cw
	}
	if cy+ch > H {
		cy = H - ch
	}
	return CameraShot{Zoom: z, CropX: cx, CropY: cy, CropW: cw, CropH: ch, Apply: true}
}

func ZoomForAction(actionType string, cfg CameraConfig) float64 {
	switch actionType {
	case "fill", "type":
		return cfg.TypingZoom
	case "select":
		return cfg.FormZoom
	default:
		return cfg.ClickZoom
	}
}

type VisualEvent struct {
	Type           string  `json:"type"`
	Interaction    string  `json:"interaction,omitempty"`
	SpeechID       string  `json:"speech_id,omitempty"`
	BBox           *BBox   `json:"bbox,omitempty"`
	NormBBox       *BBox   `json:"norm_bbox,omitempty"`
	CursorFrom     *Point  `json:"cursor_from,omitempty"`
	CursorTo       *Point  `json:"cursor_to,omitempty"`
	Zoom           float64 `json:"zoom,omitempty"`
	StartedAtMs    int64   `json:"started_at_ms,omitempty"`
	FocusAtMs      int64   `json:"focus_at_ms,omitempty"`
	ActionAtMs     int64   `json:"action_at_ms,omitempty"`
	EndedAtMs      int64   `json:"ended_at_ms,omitempty"`
	TypingChars    int     `json:"typing_chars,omitempty"`
	TypingMs       int64   `json:"typing_ms,omitempty"`
	Progressive    bool    `json:"progressive,omitempty"`
	Instant        bool    `json:"instant,omitempty"`
	Sensitive      bool    `json:"sensitive,omitempty"`
	DurationMs     int64   `json:"duration_ms,omitempty"`
	CompressedFrom float64 `json:"compressed_from_s,omitempty"`
	CompressedTo   float64 `json:"compressed_to_s,omitempty"`
	// Cinematic V2 direction evidence (all optional, V1 readers ignore).
	Anticipated  bool   `json:"anticipated,omitempty"`
	Keyboard     bool   `json:"keyboard,omitempty"`
	Attention    string `json:"attention,omitempty"`
	Spotlight    bool   `json:"spotlight,omitempty"`
	Callout      string `json:"callout,omitempty"`
	ResultHoldMs int    `json:"result_hold_ms,omitempty"`
}

func NormBBox(b BBox, vw, vh int) *BBox {
	if vw <= 0 || vh <= 0 {
		return nil
	}
	return &BBox{X: b.X / float64(vw), Y: b.Y / float64(vh), Width: b.Width / float64(vw), Height: b.Height / float64(vh)}
}

const OverlayJS = `(function(){
  if (window.__autodocCues) return;
  var NS='data-autodoc-overlay';
  function mk(tag, id, css){
    var el=document.createElement(tag);
    el.setAttribute(NS,'1'); el.setAttribute('aria-hidden','true');
    el.id=id;
    el.style.cssText='position:fixed;pointer-events:none;z-index:2147483646;'+css;
    document.documentElement.appendChild(el);
    return el;
  }
  var cursor=mk('div','__autodoc_cursor','left:0;top:0;width:22px;height:22px;opacity:0;transition:opacity .15s;');
  cursor.innerHTML='<svg width="22" height="22" viewBox="0 0 22 22"><path d="M5 2 L5 17 L9.5 13 L12 19 L14.5 17.8 L12 12 L17 12 Z" fill="#111" stroke="#fff" stroke-width="1.6"/></svg>';
  var hl=mk('div','__autodoc_hl','left:0;top:0;width:0;height:0;opacity:0;border:2px solid rgba(79,140,255,.65);border-radius:6px;box-shadow:0 0 10px rgba(79,140,255,.3);');
  var rp=mk('div','__autodoc_rp','left:0;top:0;width:36px;height:36px;margin:-18px 0 0 -18px;opacity:0;border-radius:50%;border:2px solid rgba(79,140,255,.7);');
  var halo=mk('div','__autodoc_halo','left:0;top:0;width:36px;height:36px;margin:-18px 0 0 -18px;opacity:0;border-radius:50%;border:2px solid rgba(79,140,255,.55);box-shadow:0 0 12px rgba(79,140,255,.25);');
  var haloOn=false;
  var st={cx:0,cy:0,raf:0};
  function placeCursor(x,y){ st.cx=x; st.cy=y; cursor.style.transform='translate('+x+'px,'+y+'px)'; if(haloOn){ halo.style.transform='translate('+(x+5)+'px,'+(y+4)+'px)'; } }
  function modalOpen(){ try{ var d=document.querySelector('dialog[open]'); if(d) return d; var m=document.querySelector('.modal.open,.modal.show,[role="dialog"]'); if(m){ var r=m.getBoundingClientRect(); if(r.width>0&&r.height>0) return m; } }catch(e){} return null; }
  function inside(el,x,y,w,h){ try{ if(!el) return false; var r=el.getBoundingClientRect(); return x>=r.left-8&&y>=r.top-8&&(x+w)<=r.right+8&&(y+h)<=r.bottom+8; }catch(e){ return false; } }
  var spots=[];
  function spotEl(){ var el=document.createElement('div'); el.setAttribute(NS,'1'); el.setAttribute('aria-hidden','true'); el.style.cssText='position:fixed;pointer-events:none;z-index:2147483645;background:rgba(10,12,20,0.18);opacity:0;transition:opacity .25s;'; document.documentElement.appendChild(el); spots.push(el); return el; }
  for(var i=0;i<4;i++) spotEl();
  var callout=mk('div','__autodoc_callout','left:0;top:0;opacity:0;max-width:280px;padding:6px 10px;border-radius:8px;background:rgba(17,20,32,.92);color:#fff;font:600 12px/1.4 system-ui,sans-serif;box-shadow:0 2px 10px rgba(0,0,0,.25);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;');
  window.__autodocCues={
    halo:function(on){ haloOn=!!on; halo.style.opacity=on?'1':'0'; if(on){ halo.style.transform='translate('+(st.cx+5)+'px,'+(st.cy+4)+'px)'; } },
    cursorShow:function(x,y){ cursor.style.opacity='1'; placeCursor(x,y); },
    cursorMove:function(x0,y0,x1,y1,dur){
      cursor.style.opacity='1';
      cancelAnimationFrame(st.raf);
      var t0=null;
      function ease(t){ return t*t*(3-2*t); }
      function frame(ts){
        if(t0===null) t0=ts;
        var k=Math.min(1,(ts-t0)/Math.max(1,dur));
        var e=ease(k);
        placeCursor(x0+(x1-x0)*e, y0+(y1-y0)*e);
        if(k<1) st.raf=requestAnimationFrame(frame);
      }
      placeCursor(x0,y0);
      st.raf=requestAnimationFrame(frame);
    },
    highlight:function(x,y,w,h,ms){
      hl.style.opacity='1';
      hl.style.transform='translate('+x+'px,'+y+'px)';
      hl.style.width=w+'px'; hl.style.height=h+'px';
      setTimeout(function(){ hl.style.opacity='0'; }, ms);
    },
    ripple:function(x,y,ms){
      rp.style.opacity='1';
      rp.style.transform='translate('+x+'px,'+y+'px) scale(.4)';
      rp.style.transition='none';
      void rp.offsetWidth;
      rp.style.transition='transform '+ms+'ms ease-out,opacity '+ms+'ms ease-out';
      rp.style.transform='translate('+x+'px,'+y+'px) scale(3.2)';
      rp.style.opacity='0';
    },
    spotlight:function(x,y,w,h,alpha){
      var m=modalOpen();
      if(m && !inside(m,x,y,w,h)) return false;
      var pad=14;
      x-=pad; y-=pad; w+=pad*2; h+=pad*2;
      var W=window.innerWidth,H=window.innerHeight;
      var pos=[[0,0,W,y],[0,y,x,h],[x+w,y,W-(x+w),h],[0,y+h,W,H-(y+h)]];
      for(var i=0;i<4;i++){ var s=spots[i],p=pos[i]; s.style.opacity='1'; s.style.background='rgba(10,12,20,'+alpha+')'; s.style.transform='translate('+Math.max(0,p[0])+'px,'+Math.max(0,p[1])+'px)'; s.style.width=Math.max(0,p[2])+'px'; s.style.height=Math.max(0,p[3])+'px'; }
      return true;
    },
    spotlightHide:function(){ for(var i=0;i<spots.length;i++) spots[i].style.opacity='0'; },
    callout:function(text,x,y,w,h,place,success){
      var cw=Math.min(280,Math.max(90,text.length*7+28));
      var chh=30;
      var cx=x+w/2-cw/2;
      var cy=(place==='below')?y+h+10:y-chh-10;
      if(cy<8) cy=y+h+10;
      if(cy+chh>window.innerHeight-8) cy=y-chh-10;
      if(cx<8) cx=8;
      if(cx+cw>window.innerWidth-8) cx=window.innerWidth-8-cw;
      callout.textContent=text;
      callout.style.opacity='1';
      callout.style.transform='translate('+cx+'px,'+cy+'px)';
      callout.style.width=cw+'px';
      if(success) callout.style.background='rgba(22,101,52,.94)';
      else callout.style.background='rgba(17,20,32,.92)';
    },
    calloutHide:function(){ callout.style.opacity='0'; }
  };
})();`

func EnsureOverlayJS() string { return OverlayJS }

func CursorMoveJS(from, to Point, ms int) string {
	return `window.__autodocCues&&window.__autodocCues.cursorMove(` +
		f2(from.X) + `,` + f2(from.Y) + `,` + f2(to.X) + `,` + f2(to.Y) + `,` + itoa(ms) + `)`
}

func CursorShowJS(p Point) string {
	return `window.__autodocCues&&window.__autodocCues.cursorShow(` + f2(p.X) + `,` + f2(p.Y) + `)`
}

// HaloJS toggles the discreet cursor halo (training/onboarding emphasis).
func HaloJS(on bool) string {
	if on {
		return `window.__autodocCues&&window.__autodocCues.halo(true)`
	}
	return `window.__autodocCues&&window.__autodocCues.halo(false)`
}

func HighlightJS(b BBox, ms int) string {
	return `window.__autodocCues&&window.__autodocCues.highlight(` +
		f2(b.X) + `,` + f2(b.Y) + `,` + f2(b.Width) + `,` + f2(b.Height) + `,` + itoa(ms) + `)`
}

func RippleJS(p Point, ms int) string {
	return `window.__autodocCues&&window.__autodocCues.ripple(` + f2(p.X) + `,` + f2(p.Y) + `,` + itoa(ms) + `)`
}

// SpotlightJS dims everything outside bbox+pad at alpha (0..0.28).
// Returns false when modal protection skips the overlay.
func SpotlightJS(b BBox, alpha float64) string {
	a := int(alpha*100 + 0.5)
	return `window.__autodocCues&&window.__autodocCues.spotlight(` +
		f2(b.X) + `,` + f2(b.Y) + `,` + f2(b.Width) + `,` + f2(b.Height) + `,` + f2(float64(a)/100) + `)`
}

func SpotlightHideJS() string { return `window.__autodocCues&&window.__autodocCues.spotlightHide()` }

// CalloutJS shows a small semantic pill attached to bbox.
func CalloutJS(text string, b BBox, place string, success bool) string {
	s := "false"
	if success {
		s = "true"
	}
	q := "`" + strings.ReplaceAll(text, "`", "'") + "`"
	_ = q
	esc := strings.ReplaceAll(text, `\`, `\\`)
	esc = strings.ReplaceAll(esc, `"`, `\"`)
	return `window.__autodocCues&&window.__autodocCues.callout("` + esc + `",` +
		f2(b.X) + `,` + f2(b.Y) + `,` + f2(b.Width) + `,` + f2(b.Height) + `,"` + place + `",` + s + `)`
}

func CalloutHideJS() string { return `window.__autodocCues&&window.__autodocCues.calloutHide()` }

func f2(f float64) string {
	return itoaFloat(f)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [16]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

func itoaFloat(f float64) string {
	i := int(math.Round(f * 100))
	neg := i < 0
	if neg {
		i = -i
	}
	q, r := i/100, i%100
	s := itoa(q) + "."
	if r < 10 {
		s += "0"
	}
	s += itoa(r)
	if neg {
		s = "-" + s
	}
	return s
}
