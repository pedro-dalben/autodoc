# Cinematic V2 — AI Director & Automatic Editing

Cinematic V1 taught AutoDoc to **execute, record and synchronize** a
tutorial correctly (cursor, ripple, highlight, typing, camera close-ups,
paced capture, wait compression, `validate --sync`).

Cinematic V2 teaches AutoDoc to **direct** it: understand what is being
taught, aim the viewer's attention, move the camera with continuity,
confirm results, and cut dead time like a tutorial editor would.

## V1 vs V2

| Concern | V1 | V2 |
|---|---|---|
| Unit of meaning | actions + timeline | semantic scenes/beats (intent, anchor, action, result) |
| Attention | per-action highlight/ripple | Attention Director (stay/focus/spotlight/follow/…) |
| Spotlight | — | subtle de-emphasis overlay (≤0.28 alpha, modal-safe) |
| Anticipation | cursor glide | reveal → approach → settle envelope before actions |
| Camera | per-action zoom | temporal Camera Director with continuity + context restore |
| Dead time | wait compression | classified editing (semantic/technical/loading/transition/…) + EDL |
| Results | — | action → result → confirmation with minimum hold |
| Static narration | — | detected; intentional vs unintentional stillness |
| Callouts | — | optional, small, collision-safe |
| Sound | — | optional infra, default off, never over narration |
| QA | `validate --sync` | `validate --cinematic` (12 gates + diagnostic score) |

V1 storyboards validate and render unchanged: absent `cinematic:`
configuration resolves to safe defaults (director on, callouts off,
sound off, max zoom 1.25). No migration required.

## Pipeline

```text
storyboard (+ optional cinematic:)
    ↓  recipe.Compile
recipe (deterministic steps)
    ↓  cinematic.PlanScenes
scene_plan.json      (beats: type, intent, anchor, action, expected result)
    ↓  autodoc record (browser execution, bboxes discovered at replay)
events + raw video
    ↓  timeline.Reconcile (planned × actual clocks)
final_timeline.json
    ↓  cinematic.Direct
attention  → cinematic_plan.json (attention + camera + callouts + pauses)
camera     → directed final timeline (sync-safe zoom/hold adjustments)
edit plan  → edit_plan.json (EDL: establish/anticipation/action/…)
    ↓  media.RenderCinematic
tutorial.mp4
    ↓  validate --cinematic
cinematic_report.json + AUTODOC_CINEMATIC_QA block
```

Record once, analyze/render afterwards: browser execution never does
heavy processing; direction and QA run deterministically off the events.

## Scene model

```text
BEAT
 ├── type        (narration-only, explanation, click/typing/selection,
 │                submit, result, confirmation, loading, modal,
 │                navigation, page transition, completion)
 ├── intent      ("teach click on test-id=conv-alice")
 ├── narration   (borrowed from adjacent speech; never rewritten)
 ├── anchor      (target, form, container, result, viewport, modal…)
 ├── anticipation
 ├── action
 ├── expected result
 ├── confirmation
 └── exit / context restore
```

Classification is a pure function of the step sequence (`ClassifyBeat`),
so plans reproduce exactly. Overrides (`camera:`, `attention:` on scenes
and actions, `anchor:` on speech) exist for special cases; semantic
defaults cover the common path.

## Directors

**Attention Director** decides HOW each beat is presented: `stay`,
`focus`, `soft_zoom`, `pan`, `pan_zoom`, `spotlight`, `highlight`,
`follow`, `context_restore`, `none`. Narration without an anchor holds
still (flagged by QA, never masked with gratuitous motion).

**Camera Director V2** adds temporal context: current position, next
target, distance, narration duration, safe area, recent history. Nearby
targets reposition (`pan`) instead of re-zooming; far jumps move once
(`pan_zoom`); oscillation is suppressed; after focused work the camera
restores full context. Max zoom 1.25, transitions ≥250ms, targets
clamped to the safe frame.

**Anticipation** runs reveal (350ms) → cursor approach (≤500ms) →
settle (≥200ms) before important actions, with the target highlighted
through the whole envelope.

**Automatic editor** classifies every wait (`semantic`, `technical`,
`loading`, `transition`, `user-readable`, `narration-covered`) and
decides `keep | compress | speed_ramp | cut | freeze`. Speech is never
compressed; causality is never reversed. The EDL (`edit_plan.json`)
keeps "what happened" separate from "how it is edited".

**Result confirmation**: actions declaring `result_target:` are waited
on, highlighted and held (`min_hold_ms`, default 1000ms) by extending
video-only windows — audio timing never shifts.

## Configuration

```yaml
cinematic:
  director: true
  attention: {enabled: true, spotlight: true, max_dim: 0.18}
  camera: {continuity: true, context_restore: true, max_zoom: 1.25}
  anticipation: {enabled: true}
  results: {confirmation: true, min_hold_ms: 1000}
  editing: {compress_dead_time: true, max_speed: 6}
  callouts: {enabled: false}   # opt-in per tutorial
  sound: {enabled: false}      # infra only; never music by default
```

Per-action overrides: `camera: stay|focus|contextual|none`,
`attention: …|none`, `result_target:`, `result_hold_ms:`, `callout:`,
`no_anticipation:`. Per-speech pacing: `pause_before_ms:`,
`pause_after_ms:`, `anchor:` — pacing only, content preserved.

## QA

```bash
autodoc validate --cinematic --storyboard storyboard.yml
```

```text
AUTODOC_CINEMATIC_QA: PASS|FAIL
sync, scene-planning, visual-anchors, action-anticipation,
result-confirmation, static-narration, camera-continuity,
context-restoration, target-visibility, overlay-collisions,
text-legibility, dead-time-editing
Cinematic Score: 94/100 (debug/comparison only; gates decide)
```

Thresholds: target visible ≥400ms, result ≥800ms, camera ≥250ms,
max zoom ≤1.25, unintentional static ≤2.5s, zero
collisions/clipping/safe-area/speech-compression/causality issues.

## Debugging

Workspace artifacts (per run dir, never published to `docs/autodoc`):
`scene_plan.json`, `cinematic_plan.json`, `edit_plan.json`,
`cinematic_report.json`. Frame evidence: extract raw captures at
anticipation / ripple / typing / result / restore timestamps from
`final_timeline.json` (`ActionAtS`, `VideoStartS/EndS`) and compare
regions with `meanAbsDiff` (see `test/e2e/cinematic_v2_test.go`).

## Product principle

Every effect must answer yes to: *does this help the viewer understand
the software?* The app stays the protagonist: no excessive zoom, no
constant motion, no giant arrows/text, no flashy transitions, no
overlays covering the product, no decorative effects.
