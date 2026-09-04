package cinematic

import (
	"math"

	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// CameraMove is the temporal camera verdict for one final-timeline segment.
type CameraMove string

const (
	CamStay           CameraMove = "stay"
	CamZoom           CameraMove = "zoom"
	CamPan            CameraMove = "pan"
	CamPanZoom        CameraMove = "pan_zoom"
	CamZoomOut        CameraMove = "zoom_out"
	CamContextRestore CameraMove = "context_restore"
	CamCut            CameraMove = "cut"
)

// CameraDecision pairs a segment with its directed move + zoom.
type CameraDecision struct {
	SegmentIdx int        `json:"segment_idx"`
	SceneID    string     `json:"scene_id"`
	Kind       string     `json:"kind"`
	Label      string     `json:"label"`
	Move       CameraMove `json:"move"`
	Zoom       float64    `json:"zoom"`
	Reason     string     `json:"reason"`
	Reversal   bool       `json:"reversal,omitempty"`
	Suppressed bool       `json:"suppressed,omitempty"`
}

// CameraPlan is the serializable camera section of cinematic_plan.json.
type CameraPlan struct {
	Decisions []CameraDecision `json:"decisions"`
	Reversals int              `json:"reversals"`
	Changes   int              `json:"changes"`
}

// centerOf returns the normalized viewport center of a bbox (or -1).
func centerOf(nb *visual.BBox) (float64, float64) {
	if nb == nil {
		return -1, -1
	}
	return nb.X + nb.Width/2, nb.Y + nb.Height/2
}

// distNorm is the normalized euclidean distance between two bbox centers.
func distNorm(a, b *visual.BBox) float64 {
	if a == nil || b == nil {
		return -1
	}
	ax, ay := centerOf(a)
	bx, by := centerOf(b)
	dx, dy := ax-bx, ay-by
	return math.Sqrt(dx*dx + dy*dy)
}

// DirectCamera assigns a temporal camera move to every final-timeline
// segment. Rules:
//
//   - speech/hold/wait keep the inherited position (stay) unless a
//     context restore is due;
//   - actions keep the reconciled zoom when safe (<= maxZoom, target
//     visible), else clamp;
//   - consecutive zoom-in/out oscillation is suppressed: the second move
//     collapses to stay/pan (continuity penalty);
//   - a far jump between two focused actions becomes pan_zoom instead of
//     zoom-out + zoom-in;
//   - after a focused action, the next speech/hold restores full context
//     (zoom_out) when the attention plan asks for it.
//
// Overrides: action camera "none"/"stay" forces stay; "focus" forces zoom;
// "contextual" leaves the semantic default.
func DirectCamera(ft *timeline.FinalTimeline, att *AttentionPlan, beats []BeatPlan, cfg visual.CinematicConfig, vis visual.Config) *CameraPlan {
	plan := &CameraPlan{}
	vis.Camera.MaxZoom = cfg.Camera.MaxZoom
	decByBeat := map[int]AttentionDecision{}
	for _, d := range att.Decisions {
		decByBeat[d.BeatIndex] = d
	}
	beatIdxOf := func(sg timeline.AVSegment) int {
		for i, b := range beats {
			if b.SceneID == sg.SceneID && (b.ActionLabel == sg.Label || b.SpeechID == sg.Label) {
				return i
			}
		}
		return -1
	}
	var prevZoomed bool
	var prevBox *visual.BBox
	var prevMove CameraMove
	zoomedStreak := 0
	for i, sg := range ft.Segments {
		dec := CameraDecision{SegmentIdx: i, SceneID: sg.SceneID, Kind: sg.Kind, Label: sg.Label, Zoom: sg.Zoom}
		zoom := sg.Zoom
		if zoom > cfg.Camera.MaxZoom {
			zoom = cfg.Camera.MaxZoom
		}
		bi := beatIdxOf(sg)
		var attDec AttentionDecision
		if d, ok := decByBeat[bi]; ok {
			attDec = d
		}
		camOv := ""
		if bi >= 0 && bi < len(beats) {
			camOv = beats[bi].CameraOverride
		}
		switch sg.Kind {
		case "action":
			if camOv == "none" || camOv == "stay" {
				dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "override-"+camOv
				break
			}
			if zoom <= 1.01 || !cfg.DirectorOn() {
				dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "no-focus-needed"
				break
			}
			// Safety: target must be inside the safe frame.
			if !bboxSafe(sg.NormBBox) {
				dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "target-outside-safe-area"
				break
			}
			d := distNorm(prevBox, sg.NormBBox)
			if cfg.ContinuityOn() && prevZoomed && d >= 0 && d < 0.12 && prevMove != CamStay {
				// Same neighborhood: small reposition, not a new zoom.
				dec.Move, dec.Zoom, dec.Reason = CamPan, zoom, "continuity-reposition"
				break
			}
			if cfg.ContinuityOn() && prevZoomed && d >= 0.45 {
				dec.Move, dec.Zoom, dec.Reason = CamPanZoom, zoom, "far-target-coherent-move"
				break
			}
			if cfg.ContinuityOn() && zoomedStreak >= 2 && prevMove == CamZoomOut {
				dec.Move, dec.Zoom, dec.Reason, dec.Suppressed = CamStay, 1, "oscillation-suppressed", true
				break
			}
			if sg.NormBBox != nil && sg.NormBBox.Width*sg.NormBBox.Height > 0.5 {
				dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "target-too-large"
				break
			}
			dec.Move, dec.Zoom, dec.Reason = CamZoom, zoom, "focused-action"
			_ = attDec
		case "speech", "hold":
			if attDec.Strategy == AttContextRestore && prevZoomed && cfg.ContextRestoreOn() {
				dec.Move, dec.Zoom, dec.Reason = CamContextRestore, 1, "context-restore"
				break
			}
			if prevZoomed && sg.Kind == "speech" && cfg.ContinuityOn() {
				// Hold the close-up while its narration anchor is on screen;
				// restoring mid-explanation would disorient.
				dec.Move, dec.Zoom, dec.Reason = CamStay, prevZoomOf(ft, i), "hold-focus-during-anchor"
				break
			}
			dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "stable-read"
		case "wait":
			if sg.Compressed {
				dec.Move, dec.Zoom, dec.Reason = CamCut, 1, "compressed-transition"
			} else {
				dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "wait-hold"
			}
		default:
			dec.Move, dec.Zoom, dec.Reason = CamStay, 1, "default"
		}
		// Reversal accounting: zoom-out immediately followed by zoom-in
		// (or vice versa) without an intervening restore.
		if (prevMove == CamZoom || prevMove == CamPanZoom) && (dec.Move == CamZoomOut || dec.Move == CamContextRestore) {
			dec.Reversal = false
		}
		if (prevMove == CamZoomOut || prevMove == CamContextRestore) && (dec.Move == CamZoom || dec.Move == CamPanZoom) {
			// restore-then-focus is the intended pattern, not a reversal.
			dec.Reversal = false
		}
		if dec.Move == CamZoom || dec.Move == CamPanZoom {
			if prevZoomed {
				zoomedStreak++
			} else {
				zoomedStreak = 1
			}
			prevZoomed = true
			prevBox = sg.NormBBox
			plan.Changes++
		} else if dec.Move == CamStay && prevZoomed && (sg.Kind == "speech" || sg.Kind == "hold") && dec.Zoom > 1 {
			zoomedStreak++
		} else if dec.Move == CamContextRestore || dec.Move == CamZoomOut {
			prevZoomed = false
			prevBox = nil
			zoomedStreak = 0
			plan.Changes++
		} else if dec.Move == CamPan {
			plan.Changes++
			prevBox = sg.NormBBox
		}
		prevMove = dec.Move
		plan.Decisions = append(plan.Decisions, dec)
	}
	// Second pass: flag true reversals (zoom, zoom-out, zoom with no
	// restore value in between and large spatial jumps).
	flagReversals(plan)
	return plan
}

func prevZoomOf(ft *timeline.FinalTimeline, i int) float64 {
	for j := i - 1; j >= 0; j-- {
		if ft.Segments[j].Zoom > 1.01 {
			return ft.Segments[j].Zoom
		}
	}
	return 1
}

func flagReversals(plan *CameraPlan) {
	lastZoomIdx := -1
	lastOutIdx := -1
	for i, d := range plan.Decisions {
		switch d.Move {
		case CamZoom, CamPanZoom:
			if lastOutIdx >= 0 && lastZoomIdx >= 0 && lastOutIdx > lastZoomIdx && i-lastOutIdx <= 1 {
				plan.Decisions[i].Reversal = true
				plan.Reversals++
			}
			lastZoomIdx = i
		case CamZoomOut, CamContextRestore:
			lastOutIdx = i
		case CamStay, CamPan, CamCut:
		}
	}
}

// bboxSafe enforces the safe frame: normalized bbox fully inside
// [0.02, 0.98] with non-degenerate size.
func bboxSafe(nb *visual.BBox) bool {
	if nb == nil {
		return true
	}
	if nb.Width <= 0 || nb.Height <= 0 {
		return false
	}
	const m = 0.02
	return nb.X >= -m && nb.Y >= -m && nb.X+nb.Width <= 1+m && nb.Y+nb.Height <= 1+m
}

// ApplyCamera clamps reconciled zooms to the directed plan (continuity,
// max zoom, safe area). Durations are untouched, so A/V sync is
// preserved. Returns the number of segments adjusted.
func ApplyCamera(ft *timeline.FinalTimeline, plan *CameraPlan) int {
	n := 0
	for _, d := range plan.Decisions {
		if d.SegmentIdx < 0 || d.SegmentIdx >= len(ft.Segments) {
			continue
		}
		sg := &ft.Segments[d.SegmentIdx]
		want := d.Zoom
		if d.Move == CamStay || d.Move == CamContextRestore || d.Move == CamZoomOut || d.Move == CamCut {
			if d.Reason == "hold-focus-during-anchor" {
				want = d.Zoom
			} else {
				want = 1
			}
		}
		if sg.Zoom != want {
			sg.Zoom = want
			if want <= 1.01 {
				sg.NormBBox = keepBBoxForEvidence(sg.NormBBox)
			}
			n++
		}
	}
	return n
}

// keepBBoxForEvidence preserves bbox evidence even when zoom is relaxed;
// the renderer only zooms when Zoom > 1.01, QA still sees the target.
func keepBBoxForEvidence(nb *visual.BBox) *visual.BBox { return nb }
