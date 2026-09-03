# Getting started

Division of labor — be explicit about it:

- **Harness (you/agent) thinks:** explore the app with Playwright MCP, discover
  locators/waits/states, author `storyboard.yml`.
- **AutoDoc executes:** validates, synthesizes TTS, replays deterministically,
  captures per scene, renders with FFmpeg. AutoDoc calls no LLM.

## 1. Init

```bash
autodoc init
autodoc doctor
```

## 2. Authenticate (off-camera, never recorded)

```bash
autodoc browser login --profile docs
```

Sign in with a fixture/test account in the dedicated AutoDoc profile window.
Recording starts after this. Never use your everyday browser profile.
Credentials never enter storyboards, logs, or video.

## 3. Explore with Playwright MCP (discovery, never recorded)

Navigate, inspect accessible roles/names, confirm waits and loading states.
Locator preference: `test_id` → `role`+`name` → `label` → stable attribute →
CSS fallback. Never DOM-position selectors.

## 4. Author `storyboard.yml`

Beats are explicit ordered sequences — see [storyboard.md](storyboard.md):

```yaml
- speech: {text: "Clique em Novo para cadastrar um material."}
- action: {type: click, target: {test_id: new-material-btn}}
- wait: {state: visible, target: {role: dialog}}
```

## 5. Validate → TTS → record → render → export

```bash
autodoc storyboard validate --storyboard storyboard.yml
autodoc compile --storyboard storyboard.yml
autodoc tts --storyboard storyboard.yml     # cached per speech segment
autodoc record --storyboard storyboard.yml  # scene-scoped; --retake scene-002
autodoc render --storyboard storyboard.yml
autodoc export --storyboard storyboard.yml
autodoc validate --storyboard storyboard.yml
```

Each `speech` event is an independent TTS/cache unit. `record` replays the
compiled recipe: setup prefix runs without capture, expected state is checked,
then the scene is captured to `scene-<id>.webm` + `events-<id>.jsonl`.
`render` reconciles the timeline (real TTS durations + settle/hold padding)
into the final MP4; `export` writes the publishable bundle.

## 6. Deliver

`docs/autodoc/<tutorial>/` contains `tutorial.mp4`, `tutorial.md`,
`subtitles.srt/.vtt`, `thumbnail.png`, `storyboard.yml`, `timeline.json`,
`metadata.json`, `screenshots/`. Intermediates stay in `.autodoc/_work/`.
