package visual

import "math"

type CursorConfig struct {
	Enabled  bool `yaml:"enabled" json:"enabled"`
	Smooth   bool `yaml:"smooth" json:"smooth"`
	MinMs    int  `yaml:"min_ms" json:"min_ms"`
	MaxMs    int  `yaml:"max_ms" json:"max_ms"`
	ParkX    int  `yaml:"park_x" json:"park_x"`
	ParkY    int  `yaml:"park_y" json:"park_y"`
	ParkMs   int  `yaml:"park_ms" json:"park_ms"`
	Disabled bool `yaml:"disabled" json:"disabled"`
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

type Config struct {
	Cursor  CursorConfig `yaml:"cursor" json:"cursor"`
	Click   ClickConfig  `yaml:"click" json:"click"`
	Typing  TypingConfig `yaml:"typing" json:"typing"`
	Camera  CameraConfig `yaml:"camera" json:"camera"`
	Pacing  PacingConfig `yaml:"pacing" json:"pacing"`
	SyncTol Tolerances   `yaml:"sync" json:"sync"`
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
  var hl=mk('div','__autodoc_hl','left:0;top:0;opacity:0;border:2.5px solid #4f8cff;border-radius:8px;box-shadow:0 0 0 4px rgba(79,140,255,.22),0 0 18px rgba(79,140,255,.35);transition:opacity .15s;');
  var rp=mk('div','__autodoc_ripple','left:0;top:0;width:14px;height:14px;margin:-7px 0 0 -7px;opacity:0;border-radius:50%;border:3px solid #4f8cff;');
  var st={cx:0,cy:0,raf:0};
  function placeCursor(x,y){ st.cx=x; st.cy=y; cursor.style.transform='translate('+x+'px,'+y+'px)'; }
  window.__autodocCues={
    cursorShow:function(x,y){ cursor.style.opacity='1'; placeCursor(x,y); },
    cursorHide:function(){ cursor.style.opacity='0'; },
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
    }
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

func HighlightJS(b BBox, ms int) string {
	return `window.__autodocCues&&window.__autodocCues.highlight(` +
		f2(b.X) + `,` + f2(b.Y) + `,` + f2(b.Width) + `,` + f2(b.Height) + `,` + itoa(ms) + `)`
}

func RippleJS(p Point, ms int) string {
	return `window.__autodocCues&&window.__autodocCues.ripple(` + f2(p.X) + `,` + f2(p.Y) + `,` + itoa(ms) + `)`
}

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
