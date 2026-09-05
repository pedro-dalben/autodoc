package cinematic

import (
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// reflowAfterHoldExtend reflows StartS/totals/speech extents after a
// hold extension (same deterministic reflow as EnsureResultHolds).
func reflowAfterHoldExtend(ft *timeline.FinalTimeline) {
	t := 0.0
	for i := range ft.Segments {
		ft.Segments[i].StartS = t
		t += ft.Segments[i].DurS
	}
	ft.TotalS = t
	si := 0
	for _, sg := range ft.Segments {
		if sg.Kind == "speech" && si < len(ft.Speeches) {
			ft.Speeches[si].StartS = sg.StartS
			ft.Speeches[si].EndS = sg.StartS + sg.DurS
			si++
		}
	}
	for i := range ft.SceneClips {
		var s0, s1 float64
		first := true
		for _, sg := range ft.Segments {
			if sg.SceneID == ft.SceneClips[i].SceneID {
				if first {
					s0 = sg.StartS
					first = false
				}
				s1 = sg.StartS + sg.DurS
			}
		}
		ft.SceneClips[i].StartS, ft.SceneClips[i].EndS, ft.SceneClips[i].Duration = s0, s1, s1-s0
	}
}

// applyResolvedToBeats threads resolved explicit direction into the
// semantic beats before the directors run. Only non-auto resolved
// values patch beats; everything else stays under automatic control.
// Legacy hand-authored camera/attention overrides are preserved unless
// explicit direction contradicts them (direction wins: it is the newer,
// more specific user intent).
func applyResolvedToBeats(beats []BeatPlan, res []ResolvedBeat) {
	byKey := map[string]*ResolvedBeat{}
	for i := range res {
		byKey[segmentKey(res[i].SceneID, res[i].BeatID)] = &res[i]
	}
	for i := range beats {
		r, ok := byKey[segmentKey(beats[i].SceneID, beats[i].BeatID)]
		if !ok {
			continue
		}
		if r.Zoom != "" && r.Zoom != "auto" {
			beats[i].ZoomMode = r.Zoom
		}
		if r.Spotlight != "" && r.Spotlight != "auto" {
			beats[i].SpotlightMode = r.Spotlight
		}
		if beats[i].CameraOverride == "" || r.UserDirected() {
			switch {
			case r.Camera == "static" || r.Zoom == "off":
				beats[i].CameraOverride = "stay"
			case r.Camera == "focus" || r.Camera == "follow":
				beats[i].CameraOverride = "focus"
			case r.Camera == "wide":
				beats[i].CameraOverride = "stay"
			}
		}
	}
}

// enforceCameraLock pins locked scenes to their first zoomed shot:
// later zoom_out/context_restore moves inside the scene collapse to a
// stable stay at the locked zoom + bbox. No zoom-form-then-full-then-
// field oscillation while the lock holds.
func enforceCameraLock(cam *CameraPlan, beats []BeatPlan, res []ResolvedBeat) {
	if cam == nil {
		return
	}
	locked := map[string]bool{}
	for _, r := range res {
		if r.CameraLock == "on" {
			locked[r.SceneID] = true
		}
	}
	if len(locked) == 0 {
		return
	}
	lockZoom := map[string]float64{}
	lockBox := map[string]*visual.BBox{}
	for i := range cam.Decisions {
		d := &cam.Decisions[i]
		if !locked[d.SceneID] {
			continue
		}
		if _, ok := lockZoom[d.SceneID]; !ok && d.Zoom > 1.01 && d.NormBBox != nil {
			lockZoom[d.SceneID] = d.Zoom
			lockBox[d.SceneID] = d.NormBBox
			continue
		}
		if z, ok := lockZoom[d.SceneID]; ok {
			if d.Move == CamZoomOut || d.Move == CamContextRestore || d.Zoom <= 1.01 {
				d.Move, d.Zoom, d.NormBBox = CamStay, z, lockBox[d.SceneID]
				d.Reason += "+camera-lock"
			}
		}
	}
	_ = beats
}

// filterCalloutsByDirection drops callouts in scopes where resolved
// direction says callout=off, unless the beat carries an action-level
// callout=on (explicit include wins at the narrowest scope).
func filterCalloutsByDirection(callouts []Callout, beats []BeatPlan, res []ResolvedBeat) []Callout {
	offScene := map[string]bool{}
	onBeat := map[string]bool{}
	keyOf := map[int]string{}
	for _, b := range beats {
		keyOf[b.Index] = segmentKey(b.SceneID, b.BeatID)
	}
	for _, r := range res {
		if r.Callout == "off" && (r.Sources["callout"] == SourceTutorial || r.Sources["callout"] == SourceScene) {
			offScene[r.SceneID] = true
		}
		if r.Callout == "on" && r.Sources["callout"] == SourceAction {
			onBeat[segmentKey(r.SceneID, r.BeatID)] = true
		}
	}
	if len(offScene) == 0 {
		return callouts
	}
	out := callouts[:0]
	for _, c := range callouts {
		if offScene[c.SceneID] && !onBeat[keyOf[c.BeatIndex]] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// EnsureResultHoldsWithDirection extends EnsureResultHolds with the
// resolved hold floor: hold=long stretches the confirmation window to
// ~1800ms, short relaxes toward 600ms. Auto behaves exactly as before.
func EnsureResultHoldsWithDirection(ft *timeline.FinalTimeline, beats []BeatPlan, cfg visual.CinematicConfig, res []ResolvedBeat) int {
	if !cfg.ConfirmationOn() {
		return 0
	}
	byKey := map[string]*ResolvedBeat{}
	for i := range res {
		byKey[segmentKey(res[i].SceneID, res[i].BeatID)] = &res[i]
	}
	if len(byKey) == 0 {
		return EnsureResultHolds(ft, beats, cfg)
	}
	added := 0
	idx := indexBeats(beats)
	for i := range ft.Segments {
		sg := &ft.Segments[i]
		if sg.Kind != "action" {
			continue
		}
		b := beatForAction(idx, sg.SceneID, sg.BeatID, sg.Label)
		if !b.NeedsConfirmation {
			continue
		}
		minHold := ReadableResultHold(b.ExpectedResult, cfg)
		if r, ok := byKey[segmentKey(sg.SceneID, sg.BeatID)]; ok && r.Hold != "" && r.Hold != "auto" {
			if h := HoldMsFor(r, cfg.Results.MinHoldMs); h > minHold {
				minHold = h
			} else if r.Hold == "short" {
				minHold = h
			}
		}
		cover := sg.DurS
		if i+1 < len(ft.Segments) && (ft.Segments[i+1].Kind == "hold" || ft.Segments[i+1].Kind == "wait") {
			cover += ft.Segments[i+1].DurS
		}
		want := float64(minHold) / 1000.0
		if cover < want {
			sg.DurS += want - cover
			added += int((want - cover) * 1000)
		}
	}
	if added > 0 {
		reflowAfterHoldExtend(ft)
	}
	return added
}

// ApplyDirectionToCapture folds record-time direction into the capture
// configs before the browser backend starts: cursor visibility, click
// capture, typing cadence, spotlight capture. Render-time fields are
// untouched (no retake needed for those). Mutates vis/cine in place;
// nil-tolerant; absent direction leaves configs unchanged.
func ApplyDirectionToCapture(sb *storyboard.Storyboard, vis *visual.Config, cine *visual.CinematicConfig) {
	if sb == nil || sb.Direction == nil || sb.Direction.Empty() {
		return
	}
	// Scene-level cursor/click/typing also affect capture, but capture
	// runs per scene without lookahead: apply the tutorial-level intent
	// plus any unanimous scene-level value.
	d := *sb.Direction
	if sb.Scenes != nil {
		d = unanimousSceneOverlay(sb, d)
	}
	if vis == nil {
		return
	}
	switch d.Cursor {
	case "hide":
		vis.Cursor.Disabled = true
	case "show":
		vis.Cursor.Disabled = false
		vis.Cursor.Enabled = true
	}
	switch d.CursorHalo {
	case "on":
		vis.Cursor.Halo = true
	case "off":
		vis.Cursor.Halo = false
	}
	switch d.Click {
	case "off":
		vis.Click.Disabled = true
	case "subtle":
		vis.Click.Disabled = false
		vis.Click.Ripple = true
		vis.Click.Highlight = false
	case "strong":
		vis.Click.Disabled = false
		vis.Click.Ripple = true
		vis.Click.Highlight = true
		vis.Click.RippleMs = 650
	}
	switch d.ClickEffect {
	case "none":
		vis.Click.Disabled = true
	case "ripple", "ring", "pulse", "highlight":
		vis.Click.Disabled = false
		vis.Click.Ripple = true
		vis.Click.Highlight = d.ClickEffect == "highlight" || d.ClickEffect == "pulse"
	}
	switch d.Typing {
	case "instant":
		vis.Typing.Progressive = false
		vis.Typing.Disabled = false
	case "natural", "slow", "fast":
		vis.Typing.Progressive = true
		vis.Typing.Disabled = false
		vis.Typing.Highlight = true
		switch d.Typing {
		case "slow":
			vis.Typing.CharDelayMs = 90
		case "fast":
			vis.Typing.CharDelayMs = 22
		default:
			vis.Typing.CharDelayMs = 45
		}
	}
	if cine == nil {
		return
	}
	switch d.Spotlight {
	case "off":
		f := false
		cine.Attention.Spotlight = &f
	case "subtle":
		t := true
		cine.Attention.Spotlight = &t
		cine.Attention.MaxDim = 0.12
	case "medium":
		t := true
		cine.Attention.Spotlight = &t
		cine.Attention.MaxDim = 0.18
	case "strong":
		t := true
		cine.Attention.Spotlight = &t
		cine.Attention.MaxDim = 0.25
	}
	if d.Callout == "off" {
		f := false
		cine.Callouts.Enabled = &f
	} else if d.Callout == "on" {
		t := true
		cine.Callouts.Enabled = &t
	}
}

// unanimousSceneOverlay folds scene-level record-time fields into the
// tutorial direction only when every directed scene agrees (otherwise
// per-scene capture would need per-scene configs the backend lacks;
// the finer scope still applies at direct/render time).
func unanimousSceneOverlay(sb *storyboard.Storyboard, d visual.DirectionConfig) visual.DirectionConfig {
	collect := map[string]map[string]int{}
	fields := func(sc *storyboard.Scene) map[string]string {
		if sc.Direction == nil {
			return nil
		}
		return map[string]string{"cursor": sc.Direction.Cursor, "click": sc.Direction.Click, "typing": sc.Direction.Typing}
	}
	n := 0
	for i := range sb.Scenes {
		m := fields(&sb.Scenes[i])
		if m == nil {
			continue
		}
		n++
		for f, v := range m {
			if v == "" {
				continue
			}
			if collect[f] == nil {
				collect[f] = map[string]int{}
			}
			collect[f][v]++
		}
	}
	if n == 0 {
		return d
	}
	set := func(cur string, f, v string) string {
		if cur != "" {
			return cur
		}
		if collect[f] != nil && collect[f][v] == n {
			return v
		}
		return cur
	}
	_ = set
	for f, counts := range collect {
		for v, c := range counts {
			if c == n {
				switch f {
				case "cursor":
					if d.Cursor == "" {
						d.Cursor = v
					}
				case "click":
					if d.Click == "" {
						d.Click = v
					}
				case "typing":
					if d.Typing == "" {
						d.Typing = v
					}
				}
			}
		}
	}
	return d
}
