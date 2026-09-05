# Getting started

The split matters: your coding agent thinks, AutoDoc executes. The agent
explores the app, finds locators and waits, and authors `storyboard.yml`.
AutoDoc validates, synthesizes TTS, replays deterministically, captures per
scene, and renders with FFmpeg. AutoDoc calls no LLM.

## 1. Init

```bash
autodoc init --global   # once per machine: TTS, voice, browser defaults
autodoc init            # once per project: ./autodoc.toml + storyboard.yml
autodoc doctor          # shows which config source is active
```

Projects without `./autodoc.toml` inherit the global file automatically.

## 2. Dependencies

```bash
autodoc browser install
autodoc tts check
autodoc doctor
```

`doctor` must pass before you continue. If FFmpeg is missing, install it
([install.md](install.md)); if the TTS endpoint is unreachable, start the
server ([local-tts.md](local-tts.md)) or switch to silent mode:

```toml
[tts]
provider = "disabled"
```

## 3. Authenticate (off-camera, never recorded)

```bash
autodoc browser login --profile docs
```

Sign in with a fixture/test account in the dedicated AutoDoc profile window.
Recording starts after this. Never use your everyday browser profile.
Credentials never enter storyboards, logs, or video. See
[browser.md](browser.md).

## 4. Explore with Playwright MCP (discovery, never recorded)

Navigate, inspect accessible roles and names, confirm waits and loading
states. Locator preference: `test_id`, then `role` plus `name`, then
`label`, then stable attribute, CSS only as fallback. Never DOM-position
selectors.

`autodoc ui query --url <path> --intent "<what the user wants>"` returns the
relevant controls without dumping the whole page.

## 5. Author `storyboard.yml`

Beats are explicit ordered sequences. See [storyboard.md](storyboard.md):

```yaml
- speech: {text: "Click New to register a material."}
- action: {type: click, target: {test_id: new-material-btn}}
- wait: {state: visible, target: {role: dialog}}
```

## 6. Produce the tutorial

```bash
autodoc storyboard validate --storyboard storyboard.yml
autodoc tts --storyboard storyboard.yml        # cached per speech segment
autodoc record --storyboard storyboard.yml     # scene-scoped; --retake scene-002
autodoc render --storyboard storyboard.yml
autodoc export --storyboard storyboard.yml
autodoc validate --storyboard storyboard.yml
```

Each `speech` event is an independent TTS and cache unit. `record` replays
the compiled recipe: the setup prefix runs without capture, expected state
is checked, then the scene is captured to `scene-<id>.webm` plus
`events-<id>.jsonl`. `render` reconciles the timeline (real TTS durations
plus settle and hold padding) into the final MP4; `export` writes the
publishable bundle.

A cinematic QA pass is available:

```bash
autodoc validate --storyboard storyboard.yml --cinematic
```

## 7. Deliver

`docs/autodoc/<tutorial>/` holds `tutorial.mp4`, `tutorial.md`,
`subtitles.srt` and `.vtt`, `thumbnail.png`, the frozen `storyboard.yml`,
`timeline.json`, `metadata.json`, and `screenshots/`. Intermediates stay in
`.autodoc/_work/`. Never publish `_work/`, profiles, or `storage-state.json`.

A real rendered example ships in
[autodoc/gerenciando-materiais/](autodoc/gerenciando-materiais/).
