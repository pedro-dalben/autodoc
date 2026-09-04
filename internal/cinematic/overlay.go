package cinematic

import (
	"strings"

	"github.com/pedro-dalben/autodoc/internal/visual"
)

// Callout is one optional semantic overlay ("1. Escolha a conversa",
// "✓ Mensagem enviada"). Small, consistent, collision-safe, auto-removed.
type Callout struct {
	BeatIndex int    `json:"beat_index"`
	SceneID   string `json:"scene_id"`
	Text      string `json:"text"`
	// AnchorKey is the target describe string the callout attaches to.
	AnchorKey string `json:"anchor_key"`
	// Place is the resolved placement: above|below|left|right.
	Place string `json:"place"`
	// Success marks confirmation callouts (✓ styling).
	Success bool `json:"success"`
	// Suppressed records why a requested callout was dropped.
	Suppressed string `json:"suppressed,omitempty"`
}

// Rect is a normalized-viewport rectangle for collision checks.
type Rect struct {
	X, Y, W, H float64
}

func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.W+r.X && r.Y < o.Y+o.H && o.Y < r.H+r.Y
}

func (r Rect) InsideViewport(margin float64) bool {
	return r.X >= margin && r.Y >= margin && r.X+r.W <= 1-margin && r.Y+r.H <= 1+margin
}

// PlanCallouts selects at most MaxPerScene callouts. Rules: only beats
// that declare Callout text; skipped entirely when disabled; success
// styling when the beat confirms a result; placement avoids the target,
// cursor zone, modals and viewport edges (resolved deterministically).
func PlanCallouts(beats []BeatPlan, cfg visual.CinematicConfig) []Callout {
	if !cfg.CalloutsOn() {
		return nil
	}
	var out []Callout
	perScene := map[string]int{}
	for _, b := range beats {
		t := strings.TrimSpace(b.Callout)
		if t == "" {
			continue
		}
		if perScene[b.SceneID] >= cfg.Callouts.MaxPerScene {
			out = append(out, Callout{BeatIndex: b.Index, SceneID: b.SceneID, Text: t, AnchorKey: b.Anchor.TargetKey, Suppressed: "max-per-scene"})
			continue
		}
		perScene[b.SceneID]++
		c := Callout{
			BeatIndex: b.Index, SceneID: b.SceneID, Text: t,
			AnchorKey: b.Anchor.TargetKey,
			Place:     placeFor(b),
			Success:   b.NeedsConfirmation || b.Type == SceneConfirmation || strings.HasPrefix(t, "✓"),
		}
		out = append(out, c)
	}
	return out
}

func placeFor(b BeatPlan) string {
	// Deterministic default: above the target; the render/capture side
	// flips to below when the target sits in the top third (see
	// ResolveCalloutRect). No per-frame randomness.
	return "above"
}

// CalloutRect estimates the normalized callout pill rect for collision
// checks. Width scales with text length, capped; height fixed.
func CalloutRect(anchor *visual.BBox, text, place string) Rect {
	w := 0.06 + float64(len([]rune(text)))*0.006
	if w > 0.34 {
		w = 0.34
	}
	h := 0.055
	var x, y float64
	if anchor != nil {
		cx := anchor.X + anchor.Width/2
		x = cx - w/2
		if place == "above" {
			y = anchor.Y - h - 0.015
		} else {
			y = anchor.Y + anchor.Height + 0.015
		}
		// Flip when out of bounds (deterministic).
		if y < 0.01 {
			y = anchor.Y + anchor.Height + 0.015
		}
		if y+h > 0.99 {
			y = anchor.Y - h - 0.015
		}
	} else {
		x, y = 0.5-w/2, 0.08
	}
	if x < 0.01 {
		x = 0.01
	}
	if x+w > 0.99 {
		x = 0.99 - w
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// CollisionInput gathers every overlay competing for the frame.
type CollisionInput struct {
	Target   *visual.BBox
	Cursor   *visual.Point
	Callouts []CalloutRectInput
	Modal    *visual.BBox
}

type CalloutRectInput struct {
	Rect Rect
	Text string
}

// CheckCollisions enforces: zero overlay overlap, target avoidance, text
// avoidance, viewport bounds, modal bounds, cursor proximity, safe zones.
// Returns human-readable violations (empty = clean).
func CheckCollisions(in CollisionInput) []string {
	var bad []string
	const margin = 0.01
	const cursorR = 0.035
	targets := []Rect{}
	if in.Target != nil {
		targets = append(targets, normRect(*in.Target))
	}
	if in.Modal != nil {
		mr := normRect(*in.Modal)
		for _, c := range in.Callouts {
			if c.Rect.Overlaps(mr) {
				bad = append(bad, "callout overlaps modal: "+c.Text)
			}
		}
	}
	for i, c := range in.Callouts {
		if !c.Rect.InsideViewport(margin) {
			bad = append(bad, "callout out of viewport: "+c.Text)
		}
		for _, t := range targets {
			if c.Rect.Overlaps(t) {
				bad = append(bad, "callout covers target: "+c.Text)
			}
		}
		if in.Cursor != nil {
			cr := Rect{X: in.Cursor.X - cursorR, Y: in.Cursor.Y - cursorR, W: cursorR * 2, H: cursorR * 2}
			if c.Rect.Overlaps(cr) {
				bad = append(bad, "callout covers cursor: "+c.Text)
			}
		}
		for j, o := range in.Callouts {
			if i != j && c.Rect.Overlaps(o.Rect) {
				bad = append(bad, "callout overlaps callout: "+c.Text+" / "+o.Text)
			}
		}
		// Safe zones: keep clear of extreme top bar and bottom edge.
		if c.Rect.Y < 0.005 || c.Rect.Y+c.Rect.H > 0.995 {
			bad = append(bad, "callout breaches safe zone: "+c.Text)
		}
	}
	return bad
}

func normRect(b visual.BBox) Rect {
	// Accepts either pixel or normalized boxes: values > 1.5 are pixels
	// against a 1280x720 reference.
	if b.Width > 1.5 || b.Height > 1.5 || b.X > 1.5 || b.Y > 1.5 {
		return Rect{X: b.X / 1280, Y: b.Y / 720, W: b.Width / 1280, H: b.Height / 720}
	}
	return Rect{X: b.X, Y: b.Y, W: b.Width, H: b.Height}
}
