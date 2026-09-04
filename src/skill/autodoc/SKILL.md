---
name: autodoc
description: >
  AutoDoc audiovisual documentation workflow. Use when the user asks to
  create, record, re-record, or publish a narrated UI tutorial video
  (walkthrough, demo, onboarding, feature overview) for a web application.
  Teaches low-context discovery via Playwright MCP, stable storyboard
  authoring, TTS synthesis, deterministic replay, and artifact delivery.
version: "2"
---

# AutoDoc — Audiovisual Documentation Skill

You are helping produce a **narrated UI tutorial video** with AutoDoc.
AutoDoc does not call an LLM. **You** are the thinking agent; AutoDoc
validates, synthesizes, replays, captures, and renders deterministically.

AutoDoc now records in **cinematic mode by default**: a synthetic cursor
moves to each target, clicks emit a ripple + highlight, typing is
progressive and didactic, the camera eases into a close-up of small
targets, and the recorder paces itself on the narration clock so audio
and video reconcile exactly (`autodoc validate --sync`). You do **not**
author coordinates, zooms, or timings — the executor discovers bounding
boxes at replay time. Keep the storyboard semantic.

## Golden rules

1. **Never record discovery.** Exploration clicks never appear in the video.
   The storyboard is authored first, then executed by `autodoc`.
2. **Login is setup, never content** unless the tutorial is *about* login.
   Reuse AutoDoc auth state (see below); never explore the login UI as a
   feature. Never use the user's everyday browser profile.
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
   sequence inside each beat. Speech segments determine audio timing; the
   recorder reserves their exact WAV durations on a monotonic clock.
8. **No secrets in artifacts.** Credentials travel as `secret_ref: env:NAME`
   (resolved from the environment at record time). `type=password` fields are
   auto-masked and always filled instantly, never progressively.

## Discovery protocol (low-context, mandatory)

A user-given URL is a **strong target**. Treat it as the answer, not as a
starting point for a repository tour.

### Narrow-first flow

1. `autodoc project_info` (or `doctor`) — confirm the project setup.
2. Look for **existing artifacts first**: an existing `storyboard.yml`,
   `.autodoc/discovery/`, prior timelines/screenshots, and `git diff`.
   If the request is an update, diff scopes the work to affected scenes —
   never re-discover the whole tutorial.
3. Resolve the URL to the **route/controller/view or component** behind it.
   Search narrowly for that path (`admin/chat`, not `Chat` across the repo).
   Read at most ~5 directly relevant files: route, controller, primary
   view/component, related JS.
4. Open the target in the browser **once**. One full snapshot to learn
   locators and waits. Reuse the returned locators; take another snapshot
   only after the UI state changed materially.
5. Stop discovery as soon as the happy path is validated. Do not inspect
   unrelated modules, list all screens, or explore login/password reset
   unless the tutorial is about them.

### Discovery budget

- No broad repository search (`rg` over `app/`, `spec/`, …) unless blocked
   on the narrow path. Prefer exact-path search, then follow references.
- Initial code reads: at most ~5 directly relevant files.
- Browser: 1 initial snapshot, then only state-change snapshots. Never
   `snapshot → click → snapshot → click → snapshot` loops to "see what
   happens" — the action result already tells you.
- Browser actions before storyboard: as few as validate the happy path.
- Login: zero exploration. Either an AutoDoc profile/storage-state exists
   (reuse it) or you run the minimal setup once (see Auth).

### Auth without exploration

1. Check for reusable auth first: `.autodoc/cache/auth/`, configured
   `storage_state`, or an existing profile. Reuse; do not re-login to learn.
2. Otherwise automate login **off-camera** in the storyboard:
   ```yaml
   setup:
     start_url: /login
     sequence:
       - action: {type: fill, target: {test_id: login-user}, secret_ref: "env:DEMO_USER"}
       - action: {type: fill, target: {test_id: login-pass}, secret_ref: "env:DEMO_PASS"}
       - action: {type: click, target: {test_id: login-btn}}
       - wait: {state: visible, target: {test_id: home-root}, timeout_ms: 10000}
   ```
   Run once with `autodoc auth bootstrap --storyboard storyboard.yml` to
   cache the session; `record` reuses it automatically.
3. Never open a second interactive browser and wait for ENTER while
   holding Playwright MCP context. If interactive login is unavoidable,
   finish it in one step, save the state, and continue.

## Workflow

### 1. Understand the feature (narrow)

- Follow the discovery protocol above. Existing storyboard + `git diff`
  first; narrow route lookup; ≤5 files; one browser pass.

### 2. Start the app and authenticate (off-camera, no exploration)

- Start the application under test.
- Reuse cached auth state or declare `setup.sequence` with `secret_ref`.
  Recording starts only after setup passes. Never use the daily browser.

### 3. Explore with Playwright MCP (discovery, never recorded)

- Navigate to the target, inspect accessible roles/names, confirm waits
  and loading states — then stop.
- Prefer robust locators: `test_id` > `role` + accessible `name` > `label`
  > stable attribute > CSS fallback. Never DOM-position selectors.
- Note every wait the flow needs (visible headings, URL changes,
  networkidle after submits, delayed/loading states).

### 4. Author `storyboard.yml` (semantic, cinematic by default)

```yaml
version: 2
meta: {title, description, language, resolution, fps}
config: {base_url, viewport_width, viewport_height}
setup: {start_url, storage_state, sequence}
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
  `fill` clears then types progressively; `type` appends keystrokes.
  Add `instant: true` only for fields with no didactic value.
- Wait states: `visible, hidden, attached, detached, networkidle,
  load, url, timeout, settle`. Long loading waits are auto-compressed
  (`compressible: false` opts out); narration is never compressed.
- You never write coordinates, zoom levels, or ripple timings. The
  recorder discovers bounding boxes and directs cursor/highlight/ripple/
  camera. Optional `visuals:` block overrides cinematic defaults — leave
  it out unless asked.
- Narration explains **intent + result**, never mouse movement: prefer
  "Para iniciar uma nova conversa, selecione Nova conversa." over
  "Agora vou clicar no botão azul no canto superior direito." The
  cursor/ripple already show the action.
- Keep each `speech.text` to 1–2 sentences (~2–7s). Split long
  explanations around the actions they describe. During long speech the
  camera holds focus; the cursor parks in a neutral corner.

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
autodoc auth bootstrap --storyboard storyboard.yml   # once, if setup.sequence exists
autodoc record --storyboard storyboard.yml
autodoc record --storyboard storyboard.yml --retake scene-002
```

- Recording replays the recipe: setup runs **without capture**, then the
  scene is captured on the narration clock (speech/hold windows reserved
  exactly, narration padding inserted). Beat-level retakes are only valid
  when state can be reproduced exactly; prefer scene retakes.
- After recording, `record` prints the **sync report**. All rows must PASS.

### 8. Render and export

```bash
autodoc render --storyboard storyboard.yml   # --no-zoom to disable camera focus
autodoc export --storyboard storyboard.yml
```

Outputs under `docs/autodoc/<tutorial>/`:
`tutorial.mp4` (H.264 + AAC), `tutorial.md`, `subtitles.srt/.vtt`,
`thumbnail.png`, `storyboard.yml`, `timeline.json`, `final_timeline.json`,
`metadata.json` (with sync status), `screenshots/`.

### 9. Validate and deliver

```bash
autodoc validate --storyboard storyboard.yml --sync
autodoc validate --storyboard storyboard.yml --cinematic
autodoc doctor
```

Check: MP4 plays (H.264/AAC), resolution correct, duration coherent with
`final_timeline.json`, subtitles in sync, no secret sentinels in any
artifact, screenshots present, sync report PASS, cinematic QA PASS
(12 director gates + diagnostic score). Workspace debug artifacts
(`scene_plan.json`, `cinematic_plan.json`, `edit_plan.json`,
`cinematic_report.json`) stay under `.autodoc/_work/` — never publish
them to `docs/autodoc/`.

## Cinematic V2 direction (automatic, safe defaults)

The recorder is an AI Director, not just a capture tool: anticipation
envelopes precede important actions, a subtle spotlight may emphasize the
anchor (never covering modals), the camera keeps continuity and restores
context, declared `result_target:` outcomes are confirmed and held, and
loading waits are classified/compressed. You stay semantic: no
coordinates, no zoom levels, no timings. Useful storyboard hints:

- `speech: {anchor: viewport}` for full-context narration.
- `action: {callout: "1. Escolha a conversa"}` (only when the tutorial
  enables `cinematic.callouts`).
- `action: {result_target: {test_id: …}}` for outcomes the viewer must see.
- `attention: none` / `camera: stay` only for special cases (tiny icons).
- Never narrate mouse movement or camera motion; explain intent + result.
- Close dialogs the way the app supports: if a modal ignores Escape,
  reach its close button from the focused field (`Shift+Tab`, `Enter`)
  instead of-record breaking pointer clicks.

## Safety checklist (before every record)

- [ ] Authenticated via dedicated `autodoc` state (not daily browser)
- [ ] Fixture data only; no real PII, keys, or tokens
- [ ] Credentials only as `secret_ref`; none in storyboard/narration/logs
- [ ] `redact.selectors` covers sensitive areas; password inputs masked
- [ ] `autodoc storyboard validate` passes
- [ ] TTS cache status reviewed (`hit` for unchanged segments)

## Anti-patterns

- Recording the screen while "figuring out" the flow.
- Putting `fill: {value: real-password}` in a storyboard.
- Editing `_work/recipe.json` by hand to "fix" a run.
- Generating TTS before the narration text is final.
- Using CSS-position selectors that break on the next deploy.
- Publishing `storage_state` files, profiles, or `.autodoc/_work` contents.
- Broad `rg`/snapshot sweeps when the user already gave you the URL.
- Narrating mouse movement ("click the blue button at the top right").
