# Cinematic

AutoDoc directs every video automatically: camera, zoom, cursor, clicks,
typing, spotlight, callouts, result holds. Storyboards stay semantic (no
coordinates, no zoom levels, no timings); the recorder and renderer decide
the visuals at replay time from your targets.

## Directing in plain language

You only speak up when you have a preference. Unspecified controls stay
`auto`. Tell your coding agent things like:

```text
"Make the tutorial without zoom."
"Use strong zoom while editing the profile."
"Hide the cursor."
"Make clicks easier to see."
"Show keyboard shortcuts."
"Hold the result longer so it can be read."
```

## What you can direct

| Control     | Values                                                     |
|-------------|------------------------------------------------------------|
| camera      | auto, static, subtle, dynamic, follow, focus, wide         |
| zoom        | auto, off, subtle, medium, strong, extreme                 |
| cursor      | auto, show, hide                                           |
| click       | auto, off, subtle, strong (ripple, ring, pulse, highlight) |
| typing      | auto, instant, natural, slow, fast                         |
| keyboard    | auto, off, shortcuts, all                                  |
| spotlight   | auto, off, subtle, medium, strong                          |
| focus       | auto, off, outline, pulse                                  |
| callout     | auto, off, on                                              |
| result      | auto, off, emphasize                                       |
| hold        | auto, short, normal, long                                  |
| transition  | auto, cut, smooth, none                                    |
| camera lock | auto, off, on (pin the shot for a whole scene)             |

Negative directives win at their scope: "no zoom" disables zoom, while "no
zoom, but strong zoom on the login form" keeps the form zoomed.

Styles: "calmer" gives minimal motion, "more dynamic" gives more of it,
"step-by-step training" enables keyboard hints, focus, and longer holds.
Anything specific you add overrides the style where they overlap.

## Precedence

Action beats scene beats tutorial beats style beats the automatic director.
`autodoc explain` shows every decision with its source.

## Visual-only changes reuse the recording

Changing zoom, spotlight, callouts, keyboard, focus, result, or hold only
re-renders the existing capture. Changing cursor visibility, click capture,
or typing cadence needs a new browser recording.

## Storyboard `direction:` blocks

```yaml
direction:            # tutorial level
  spotlight: off
  keyboard: shortcuts
scenes:
  - id: login
    direction:        # scene level
      zoom: strong
      camera_lock: on
```

Storyboards without `direction:` behave exactly as before.

## Recording behavior (V1)

The recorder directs each action: a synthetic cursor glides to the target
(200-450 ms eased motion, parked neutral during narration), the target gets
a transient highlight, clicks emit a ripple, and typing is progressive and
didactic (instant for secrets). The camera eases into close-ups of small
targets at render time (click ~1.08x, typing ~1.15x, safe-area clamped,
modals never cropped). Recording runs on the narration clock (exact WAV
windows, monotonic events); long loading waits fast-forward instead of
freezing; `autodoc validate --sync` reports per-scene drift against
100/150/250 ms tolerances.

## AI Director (V2)

On top of recording, the director plans semantic beats: anticipation
envelopes (reveal, approach, settle) precede important actions, a subtle
spotlight can de-emphasize non-relevant regions (never covering modals),
the camera keeps continuity across beats and restores full context
afterwards, declared `result_target:` outcomes are confirmed and held, and
long loading waits are classified and compressed. Storyboards without a
`cinematic:` block get safe defaults (director on, callouts and sound off).

```yaml
cinematic:
  director: true
  attention: {enabled: true, spotlight: true}
  camera: {continuity: true, context_restore: true, max_zoom: 1.25}
  anticipation: {enabled: true}
  results: {confirmation: true, min_hold_ms: 1000}
  editing: {compress_dead_time: true}
  callouts: {enabled: false}  # opt-in; per-action `callout: "1. ..."`
  sound: {enabled: false}     # infra only
```

Per-action: `camera: stay|focus|contextual|none`, `attention: ...|none`,
`result_target:`, `result_hold_ms:`, `callout:`, `no_anticipation:`.
Per-speech pacing (never content rewriting): `pause_before_ms:`,
`pause_after_ms:`, `anchor:`.

Pipeline: storyboard compiles to a recipe, scenes are planned
(`scene_plan.json`), the browser executes with bounding boxes discovered at
replay, the timeline reconciles planned and actual clocks
(`final_timeline.json`), direction produces `cinematic_plan.json` and
`edit_plan.json`, then FFmpeg renders. Record once; direction and QA run
deterministically off the events afterwards.

## QA

```bash
autodoc validate --storyboard storyboard.yml --cinematic
```

This reports 12 gates (sync, scene planning, visual anchors, action
anticipation, result confirmation, static narration, camera continuity,
context restoration, target visibility, overlay collisions, text legibility,
dead-time editing) plus a diagnostic score. Thresholds: target visible at
least 400 ms, result at least 800 ms, camera moves at least 250 ms, max zoom
1.25, unintentional stillness at most 2.5 s, zero collisions, clipping,
safe-area violations, speech compression, or causality issues.

Debug artifacts live in the run dir (never published): `scene_plan.json`,
`cinematic_plan.json`, `edit_plan.json`, `cinematic_report.json`.

## Principle

Every effect must help the viewer understand the software. The app stays
the protagonist: no excessive zoom, no constant motion, no giant
arrows or text, no flashy transitions, no overlays covering the product, no
decorative effects.
