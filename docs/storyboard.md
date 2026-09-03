# Storyboard

`storyboard.yml` is the ONLY hand-authored source. The compiler emits the
deterministic `_work/recipe.json` (schema version 1); never edit it by hand.

## Layout

```yaml
version: 1
meta: {title, description, language, resolution, fps}
config: {base_url, viewport_width, viewport_height}
setup: {start_url, storage_state, preconditions, expected_states}
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
`hold: {duration_ms}` (max 30000) freezes the visual while silence plays.

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
