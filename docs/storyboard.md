# Storyboard

`storyboard.yml` is the ONLY hand-authored source. The compiler emits the
deterministic `_work/recipe.json` (schema version 2); never edit it by hand.
Version 1 storyboards keep validating — cinematic defaults apply automatically.

## Layout

```yaml
version: 2
meta: {title, description, language, resolution, fps}
config: {base_url, viewport_width, viewport_height}
setup: {start_url, storage_state, sequence, preconditions, expected_states}
redact: {selectors, mask_password_inputs}
scenes:
  - id: scene-001
    title: "..."
    url: "/path"
    screenshot: true
    beats:
      - id: beat-01
        sequence:
          - speech: {text: "..."}
          - action: {type: click, target: {role: link, name: Materiais}}
          - wait: {state: visible, target: {...}, timeout_ms: 8000}
          - speech: {text: "..."}
          - hold: {duration_ms: 600}
```

## Sequencing (no percentage timing)

There is no `action_at = 25% of narration`. Each beat `sequence` is an
explicitly ordered list of `speech` / `action` / `wait` / `hold` events.
The timeline derives from it: real TTS durations fix speech extents, actions
take timestamps at their sequence position, waits contribute settle padding,
holds contribute freeze/silence padding. The renderer never cuts audio and
never speed-fits voice to video.

## Actions

`goto, click, fill, type, press, select, check, uncheck, hover, reload,
goback, expect, screenshot, scroll`.

`fill`/`type` take `value` (or `text`). `press` takes `key` (default Enter).
`goto` takes `url` (relative URLs resolve against `config.base_url`).
`fill` clears then types progressively on camera (30–80 ms/char, default 45);
`type` appends keystrokes. `instant: true` forces instant fill for fields with
no didactic value. Password/secret fields always fill instantly and masked.
Credentials use `secret_ref: env:VAR_NAME` (resolved at record time, never
stored in artifacts) instead of literal values.

## Targets (robust first)

```yaml
target: {test_id: new-material-btn}
target: {role: link, name: Materiais}
target: {label: "Nome"}
target: {css: ".modal .save"}   # fallback only — flagged by validation
```

Preference: `test_id` → `role`+accessible `name` → `label` → stable
attribute → CSS. CSS-only locators emit a validation warning; DOM-position
selectors (`:nth-child`, positional XPath) must not be used.

## Waits

`visible, hidden, attached, detached, networkidle, load, url, timeout, settle`,
with `timeout_ms` (default 15000, max 120000) and `settle_ms` padding.
`url` waits accept `value:` (or `url:`) as the pattern. Selector waits are
strict: a timeout fails the record. Long loading waits are compressible
(`compressible: false` opts out) — the final video fast-forwards dead loading
(up to 8x, capped at ~2s) instead of freezing; narration is never compressed.
`hold: {duration_ms}` (max 30000) freezes the visual while silence plays.

## Cinematic recording (default, no configuration needed)

The recorder directs each action like a tutorial author would: a synthetic
cursor glides to the target (200–450 ms eased motion, parked neutral during
narration), the target gets a transient highlight, clicks emit a ripple, and
the camera eases into a close-up of small targets at render time (click
~1.08x, typing ~1.15x, safe-area clamped, modals never cropped). Bounding
boxes are discovered at replay time from your semantic targets — storyboards
never carry coordinates. An optional top-level `visuals:` block overrides
cursor/click/typing/camera/pacing/sync defaults; omit it for the tuned
defaults.

## Cinematic V2 — AI Director (default, safe)

On top of V1, the AI Director plans semantic beats (see
`docs/cinematic-v2.md`): anticipation envelopes (reveal → approach →
settle) precede important actions, a subtle spotlight can de-emphasize
non-relevant regions (never covering modals, always `pointer-events:
none`), the camera keeps continuity across beats and restores full
context afterwards, declared `result_target:` outcomes are confirmed and
held, and long loading waits are classified and compressed. Storyboards
without a `cinematic:` block get safe defaults automatically (director
on, callouts/sound off). Overrides:

```yaml
cinematic:
  director: true
  attention: {enabled: true, spotlight: true}
  camera: {continuity: true, context_restore: true, max_zoom: 1.25}
  anticipation: {enabled: true}
  results: {confirmation: true, min_hold_ms: 1000}
  editing: {compress_dead_time: true}
  callouts: {enabled: false}  # opt-in; per-action `callout: "1. …"`
  sound: {enabled: false}     # infra only
```

Per-action: `camera: stay|focus|contextual|none`, `attention: …|none`,
`result_target:`, `result_hold_ms:`, `callout:`, `no_anticipation:`.
Per-speech pacing (never content rewriting): `pause_before_ms:`,
`pause_after_ms:`, `anchor:`. `autodoc validate --cinematic` reports the
12 director gates plus a diagnostic score.

## Off-camera setup

```yaml
setup:
  start_url: /login
  sequence:
    - action: {type: fill, target: {test_id: login-user}, secret_ref: "env:DEMO_USER"}
    - action: {type: click, target: {test_id: login-btn}}
    - wait: {state: visible, target: {test_id: home-root}, timeout_ms: 10000}
```

The sequence runs before capture starts (never recorded) and its storage
state is cached under `.autodoc/cache/auth/` for reuse. `autodoc auth
bootstrap` runs it once on demand.

## Redaction

```yaml
redact:
  selectors: ["[data-sensitive]"]
  mask_password_inputs: true   # input[type=password] always masked
```

Applied as DOM overlay/blur/value-replacement BEFORE/DURING capture —
not frame post-processing.

## Validation

`autodoc storyboard validate` checks schema, locator quality, timeout ranges,
duplicate ids, and scans for secret sentinels (credentials in speech/fill
values fail the run).
