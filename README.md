# AutoDoc 🎬 — Deterministic Narrated UI Tutorial Videos

> Turn a hand-authored `storyboard.yml` into a polished, narrated product tutorial:
> per-segment TTS + deterministic Playwright replay + FFmpeg render.
> **No LLM calls. No STT. No video APIs. Fully reproducible.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache--2.0-blue)](LICENSE)
[![Playwright](https://img.shields.io/badge/Playwright-recordVideo-45ba4b?logo=playwright&logoColor=white)](https://playwright.dev)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-H.264%2BAAC-darkgreen)](https://ffmpeg.org)

---

## ✨ Why AutoDoc?

Screen-recording tutorials by hand is slow, flaky, and impossible to keep in sync with the product.
AutoDoc treats tutorials **as code**:

| Pain | AutoDoc answer |
|------|----------------|
| "Re-record the whole video for one changed sentence" | Per-speech-segment TTS cache — only changed narration is re-synthesized |
| "The click happened before the narration finished" | Explicit event sequencing — narration and actions are ordered events, never percentage-timing hacks |
| "Login credentials leaked into the video" | Off-camera auth in a dedicated profile + secret scanning + pre-capture DOM redaction |
| "One flaky step ruins the entire take" | Scene-scoped recovery — re-record a single scene with `--retake <scene-id>` |
| "Works on my machine" | Deterministic recipe (`_work/recipe.json`) compiled from the storyboard + content-hash timeline |

## 🧠 Mental model

```
storyboard.yml            ← YOU (or your coding agent) author this. The only source of truth.
      │ compile
_work/recipe.json         ← deterministic, generated, never hand-edited
      │ tts (cached per speech segment)
_work/audio/*.wav  +  timeline.json
      │ record (deterministic Playwright replay, scene by scene)
_work/raw/scene-*.webm + events-*.jsonl
      │ render (FFmpeg: real captures + narration mix)
tutorial.mp4  +  subtitles  +  thumbnail  +  tutorial.md
      │ export
docs/autodoc/<tutorial>/  ← publishable bundle
```

**Division of labor:**

- **You / your coding agent (the harness) think** — explore the app with Playwright MCP, discover locators/waits/states, author the storyboard.
- **AutoDoc executes** — validates, synthesizes, replays, captures, renders. AutoDoc itself never calls an LLM.

## 🚀 Quickstart

### Prerequisites

- Go 1.24+ (to build) **or** a released `autodoc` binary
- FFmpeg 6+ with `libx264` + `aac` (`ffmpeg` and `ffprobe` on `PATH`)
- A TTS endpoint — any OpenAI-compatible `/audio/speech` server (local Kokoro works great), or `provider = "disabled"` for silent dry runs

### Install

```bash
# From source (always works)
go install github.com/pedro-dalben/autodoc/cmd/autodoc@latest

# Or build locally
git clone https://github.com/pedro-dalben/autodoc.git && cd autodoc
go build -o autodoc ./cmd/autodoc
```

### Set up a project

```bash
autodoc init          # interactive: harnesses, TTS, browser, skill, MCP — idempotent
autodoc doctor        # full diagnostics (add --json for machines)
autodoc browser install   # Playwright browser assets (separate from the Go binary!)
```

`autodoc init` detects **Codex, Claude Code, Antigravity, Gemini CLI, OpenCode** (Cursor best-effort),
installs the canonical skill + MCP entries it owns, and writes `autodoc.toml` plus an
example `storyboard.yml` (`autodoc init --global` writes the machine-wide
`~/.config/autodoc/autodoc.toml` instead). Every command resolves config as
project → global → defaults. Re-running it is a zero-diff no-op.

### Produce a tutorial (the happy path)

```bash
# 1. Authenticate OFF-camera (never recorded, dedicated profile, fixture account)
autodoc browser login --profile docs

# 2. Author storyboard.yml with your coding agent (see the AutoDoc skill)

# 3. Validate → synthesize → record → render → export → verify
autodoc storyboard validate --storyboard storyboard.yml
autodoc tts        --storyboard storyboard.yml   # hit/miss per segment
autodoc record     --storyboard storyboard.yml   # scene-*.webm + events-*.jsonl
autodoc render     --storyboard storyboard.yml   # tutorial.mp4 (H.264 + AAC)
autodoc export     --storyboard storyboard.yml   # docs/autodoc/<tutorial>/
autodoc validate   --storyboard storyboard.yml   # coherence check
```

Re-record just one scene after a UI change:

```bash
autodoc record --storyboard storyboard.yml --retake scene-002
autodoc render --storyboard storyboard.yml && autodoc export --storyboard storyboard.yml
```

## 📝 Storyboard in 60 seconds

`storyboard.yml` is the **only** hand-authored file. Beats are explicit ordered
sequences of `speech` / `action` / `wait` / `hold` events — no `action_at = 25%` magic:

```yaml
version: 1
meta: { title: "Managing materials", language: "pt-BR", resolution: "1280x720", fps: 30 }
config: { base_url: "http://localhost:8099", viewport_width: 1280, viewport_height: 720 }
setup: { start_url: "/login" }
redact: { selectors: ["[data-sensitive]"], mask_password_inputs: true }
scenes:
  - id: scene-001
    title: "Listing materials"
    url: "/login"
    screenshot: true
    beats:
      - id: beat-01
        sequence:
          - speech: { text: "Sign in with the demo user." }
          - action: { type: click, target: { test_id: login-btn } }
          - wait:   { state: visible, target: { test_id: new-material-btn }, timeout_ms: 10000 }
          - speech: { text: "Here are the registered materials." }
          - hold:   { duration_ms: 600 }
```

Locator preference (enforced by validation): `test_id` → `role` + accessible `name`
→ `label` → stable attribute → CSS fallback. DOM-position selectors are rejected.
Full reference: [`docs/storyboard.md`](docs/storyboard.md) · machine schema: [`schemas/storyboard.schema.yml`](schemas/storyboard.schema.yml).

## 🗣️ TTS & caching

Two providers in V1 — the setup wizard only offers what actually works:

```toml
[tts]
provider = "openai-compatible"   # POST <base_url>/audio/speech, WAV response
base_url = "http://localhost:8880/v1"
model = "kokoro"
voice = "af_bella"
language = "pt-BR"
speed = 1.0
response_format = "wav"
```

```toml
[tts]
provider = "disabled"            # silent pacing audio: subtitles, docs, keyless CI
```

Each `speech` event is an independent cache unit keyed by
`hash(text, voice, model, language, speed)` — unchanged segments are never
re-synthesized (`hit`), edited ones are (`miss`). Details: [`docs/tts.md`](docs/tts.md) · [`docs/local-tts.md`](docs/local-tts.md).

## 🔒 Security by design

- **Login is never part of the video** (unless the tutorial *is* about login — then fixture credentials + deterministic password masking).
- **Dedicated browser profile** — never your everyday Chrome profile.
- **Secret scanning** — `compile`/`record` fail on sentinels (`password`, `api_key`, `ghp_`, `sk-live`, credential-like fill values…).
- **Pre-capture redaction** — DOM overlay/blur/value-replacement (`redact.selectors` + auto-mask of `input[type=password]`), not fragile frame post-processing.
- **Artifact boundaries** — publish `docs/autodoc/<tutorial>/`; never `_work/`, profiles, or `storage-state.json`.

Full policy: [`docs/security.md`](docs/security.md) · [`SECURITY.md`](SECURITY.md).

## 🤖 Coding-agent harnesses

| Harness | Skill | MCP | Notes |
|---------|-------|-----|-------|
| Codex | `~/.codex/skills/autodoc/SKILL.md` | config entry | rules block in `AGENTS.md` |
| Claude Code | `~/.claude/skills/autodoc/SKILL.md` | `mcpServers.autodoc` (owned-flag) | rules block in `CLAUDE.md` |
| Antigravity | `~/.config/antigravity/skills/autodoc/SKILL.md` | `mcpServers.autodoc` in `~/.gemini/config/mcp_config.json` | real path probes, incl. IDE install marker |
| Gemini CLI | `~/.gemini/skills/autodoc/SKILL.md` | settings merge | rules block in `GEMINI.md` |
| OpenCode | `~/.config/opencode/skills/autodoc/SKILL.md` | `mcp.servers.autodoc` local | owned-flag merge |
| Cursor | `~/.cursor/skills/autodoc/SKILL.md` | — | best-effort |

All installs are **idempotent** (re-run = zero diff) and **ownership-safe**
(`uninstall` removes only AutoDoc-owned, marker-delimited entries — your edits survive).
The canonical skill source is [`src/skill/autodoc/SKILL.md`](src/skill/autodoc/SKILL.md);
per-harness pages live in [`docs/harnesses/`](docs/harnesses/).

AutoDoc also exposes a thin MCP server (`autodoc mcp`, stdio) with 8 domain tools —
`project_info`, `storyboard_validate`, `tts_synthesize`, `timeline_build`,
`record`, `render`, `artifact_export`, `doctor`. No generic shell/browser/filesystem/LLM tools.

## 📦 What you get

```
docs/autodoc/<tutorial>/
├── tutorial.mp4      # H.264 + AAC, narration-synced
├── tutorial.md       # transcript + scene index
├── subtitles.srt
├── subtitles.vtt
├── thumbnail.png
├── storyboard.yml    # frozen source of this render
├── timeline.json     # content-hash timeline
├── metadata.json
└── screenshots/      # per-scene captures
```

Intermediates (recipe, audio, raw captures, events) stay in `.autodoc/_work/<run-id>/`.
A real rendered example ships in [`docs/autodoc/gerenciando-materiais/`](docs/autodoc/gerenciando-materiais/).

## 🧪 Development & testing

```bash
gofmt -l ./cmd ./internal ./test
go vet ./...
go test ./...                          # unit + integration (fast)
go test ./test/e2e/ -count=1 -timeout 10m   # full pipeline: fixture app + fake TTS +
                                            #   compile → TTS → replay → FFmpeg → ffprobe asserts
```

- `test/fixture` — pure Go/HTML app (login, dashboard, sidebar, list, form, select, modal, validation error, success, delayed loading). No React/Rails — it's here to test AutoDoc, not to be a product.
- `test/faketts` — deterministic fake OpenAI-compatible TTS (hash-derived sine WAVs). E2E is localhost-only; CI never touches a paid API.

Conventions: single Go binary · `CaptureBackend` interface over `playwright-go >= v0.6100.0`
(`recordVideo`, no Node sidecar) · per-scene recovery units · small semantic commits.
See [`docs/development.md`](docs/development.md) and [`CONTRIBUTING.md`](CONTRIBUTING.md).

## 🗺️ Project layout

```
cmd/autodoc            # CLI entrypoint (cobra)
internal/
  storyboard/          # schema, validation, secret sentinels
  recipe/              # deterministic compiler → _work/recipe.json
  config/              # autodoc.toml
  tts/                 # openai-compatible + disabled providers, segment cache
  timeline/            # explicit-sequence timeline builder
  capture/             # CaptureBackend interface + playwright-go backend
  media/               # FFmpeg composition, subtitles, thumbnails, probing
  pipeline/            # end-to-end orchestration (compile→tts→record→render→export)
  harness/ install/    # agent installers + ownership manifest
  doctor/              # diagnostics (autodoc/media/browser/tts/harness/skill/fs)
  mcp/  cli/  version/ # MCP server, commands, version pins
src/skill/autodoc/     # canonical SKILL.md (adapters are copies)
schemas/               # versioned storyboard JSON-schema for editors
test/fixture|faketts|e2e
docs/                  # user docs + harness pages + shipped example bundle
```

## 📚 Docs index

- [Install](docs/install.md) · [Getting started](docs/getting-started.md) · [Storyboard](docs/storyboard.md)
- [TTS](docs/tts.md) · [Local TTS](docs/local-tts.md) · [Security](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md) · [Development](docs/development.md) · [Homebrew](docs/homebrew.md)
- Harnesses: [Codex](docs/harnesses/codex.md) · [Claude Code](docs/harnesses/claude.md) · [Antigravity](docs/harnesses/antigravity.md) · [Gemini CLI](docs/harnesses/gemini.md) · [OpenCode](docs/harnesses/opencode.md)
- Open source: [LICENSE](LICENSE) · [NOTICE](NOTICE) · [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md) · [CONTRIBUTING](CONTRIBUTING.md) · [SECURITY](SECURITY.md) · [CODE_OF_CONDUCT](CODE_OF_CONDUCT.md)

## 📦 Distribution

Releases via [GoReleaser](https://goreleaser.com) ([`.goreleaser.yaml`](.goreleaser.yaml)):
Linux/macOS/Windows, amd64/arm64, checksums. Quick install: [`install.sh`](install.sh).
Homebrew tap is planned (see [`docs/homebrew.md`](docs/homebrew.md)) — no fake formula until a real release exists.

---

*AutoDoc is the deterministic executor; your coding agent is the director.
Storyboard in, tutorial out — reproducibly.*
