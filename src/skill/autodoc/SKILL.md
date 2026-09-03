---
name: autodoc
description: >
  AutoDoc audiovisual documentation workflow. Use when the user asks to
  create, record, re-record, or publish a narrated UI tutorial video
  (walkthrough, demo, onboarding, feature overview) for a web application.
  Teaches exploration via Playwright MCP, stable storyboard authoring,
  TTS synthesis, deterministic replay, and artifact delivery.
version: "1"
---

# AutoDoc — Audiovisual Documentation Skill

You are helping produce a **narrated UI tutorial video** with AutoDoc.
AutoDoc does not call an LLM. **You** are the thinking agent; AutoDoc
validates, synthesizes, replays, captures, and renders deterministically.

## Golden rules

1. **Never record discovery.** Exploration clicks never appear in the video.
   The storyboard is authored first, then executed by `autodoc`.
2. **Login is never part of the video** unless the tutorial is *about* login.
   Authenticate via `autodoc browser login --profile <name>` first; recording
   starts after authentication. Never use the user's everyday browser profile.
3. **Fixtures over real data.** Use fixture/test credentials and fixture content
   whenever available. Never put real user data or secrets in storyboards,
   narration, logs, screenshots, or video frames.
4. **Stabilize the storyboard before TTS.** TTS is cached per speech segment;
   churning narration wastes time. Freeze the text, then synthesize.
5. **Never drive the final recording interactively.** The final capture is
   always `autodoc record` (deterministic replay of the storyboard).
6. **One source of truth.** Only `storyboard.yml` is hand-edited. The compiled
   `_work/recipe.json` is generated and must never be edited by hand.
7. **No 25%-timing hacks.** Narration and actions are an explicit ordered
   sequence inside each beat. Speech segments determine audio timing.
8. **No secrets in artifacts.** Scan narration, selectors, and values for
   passwords, API keys, tokens before recording. `type=password` fields are
   auto-masked; configure `redact.selectors` for anything else sensitive.

## Workflow

### 1. Understand the feature (code + git)

- Read the relevant code and recent commits for the feature to document.
- Identify the user-visible flow: entry point, key screens, success state,
  error/validation states worth showing.

### 2. Start the app and authenticate (off-camera)

- Start the application under test (dev server, preview, or fixture app).
- Run `autodoc browser login --profile docs` and sign in with a fixture/test
  account. Recording starts only after this step.
- If the tutorial is specifically about login, use fixture credentials and
  confirm password masking is active.

### 3. Explore with Playwright MCP (discovery, never recorded)

- Use Playwright MCP (or equivalent browser tooling) to explore the flow:
  navigate, inspect accessible roles/names, try waits, find loading states.
- Prefer robust locators in this order: `test_id` > `role` + accessible
  `name` > `label` > stable attribute > CSS fallback. Never use
  DOM-position selectors (`:nth-child`, positional XPath).
- Note every wait the flow needs (visible headings, URL changes, networkidle
  after submits, delayed/loading states).

### 4. Author `storyboard.yml`

Structure (single source):

```yaml
version: 1
meta: {title, description, language, resolution, fps}
config: {base_url, viewport_width, viewport_height}
setup: {start_url, storage_state, expected_states}
redact: {selectors, mask_password_inputs}
scenes:
  - id: scene-001
    title: "..."
    url: "/path"
    beats:
      - id: beat-01
        sequence:
          - speech: {text: "..."}
          - action: {type: click, target: {role: link, name: Materiais}}
          - wait: {state: visible, target: {...}, timeout_ms: 8000}
          - speech: {text: "..."}
          - hold: {duration_ms: 600}
```

- Each beat `sequence` is an **explicitly ordered** list of
  `speech` / `action` / `wait` / `hold` events.
- Supported actions: `goto, click, fill, type, press, select, check,
  uncheck, hover, reload, goback, expect, screenshot, scroll`.
- Wait states: `visible, hidden, attached, detached, networkidle,
  load, url, timeout, settle`.
- Keep each `speech.text` focused (1–2 sentences). Split long explanations
  into multiple speech events around the actions they describe.

### 5. Validate and compile

```bash
autodoc storyboard validate --storyboard storyboard.yml
autodoc compile --storyboard storyboard.yml
```

Fix every validation error. The compiler emits the deterministic
`_work/recipe.json` plus the per-segment speech inventory.

### 6. Synthesize narration (TTS)

```bash
autodoc tts --storyboard storyboard.yml
```

- Each `speech` event is an independent TTS/cache unit keyed by the hash of
  (text, voice, model, language, speed). Unchanged segments are cache hits.
- `provider = "disabled"` produces silent pacing audio for dry runs.
- Never commit or publish API keys; the key lives in the environment variable
  named by `tts.api_key_env`.

### 7. Deterministic recording (scene-scoped, retakable)

```bash
autodoc record --storyboard storyboard.yml
autodoc record --storyboard storyboard.yml --retake scene-002
```

- Recording replays the recipe: setup prefix runs **without capture**,
  expected state is validated, then the scene is captured.
- Beat-level retakes are only valid when state can be reproduced exactly;
  prefer scene retakes.

### 8. Render and export

```bash
autodoc render --storyboard storyboard.yml
autodoc export --storyboard storyboard.yml
```

Outputs under `docs/autodoc/<tutorial>/`:
`tutorial.mp4` (H.264 + AAC), `tutorial.md`, `subtitles.srt/.vtt`,
`thumbnail.png`, `storyboard.yml`, `timeline.json`, `metadata.json`,
`screenshots/`.

### 9. Validate and deliver

```bash
autodoc validate --storyboard storyboard.yml
autodoc doctor
```

Check: MP4 plays (H.264/AAC), resolution correct, duration coherent with
narration, subtitles in sync, no secret sentinels in any artifact, screenshots
present, `timeline.json` matches the storyboard hash.

## Safety checklist (before every record)

- [ ] Authenticated via dedicated `autodoc` profile (not daily browser)
- [ ] Fixture data only; no real PII, keys, or tokens
- [ ] `redact.selectors` covers sensitive areas; password inputs masked
- [ ] No credentials in storyboard, narration, screenshots, or logs
- [ ] `autodoc storyboard validate` passes
- [ ] TTS cache status reviewed (`hit` for unchanged segments)

## Anti-patterns

- Recording the screen while "figuring out" the flow.
- Putting `fill: {value: real-password}` in a storyboard.
- Editing `_work/recipe.json` by hand to "fix" a run.
- Generating TTS before the narration text is final.
- Using CSS-position selectors that break on the next deploy.
- Publishing `storage_state` files, profiles, or `.autodoc/_work` contents.
