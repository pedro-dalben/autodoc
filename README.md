# AutoDoc

AutoDoc turns a hand-authored `storyboard.yml` into a narrated product tutorial:
per-segment TTS plus deterministic Playwright replay plus FFmpeg render.
No LLM calls. No STT. No video APIs. The output is reproducible.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache--2.0-blue)](LICENSE)
[![Playwright](https://img.shields.io/badge/Playwright-recordVideo-45ba4b?logo=playwright&logoColor=white)](https://playwright.dev)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-H.264%2BAAC-darkgreen)](https://ffmpeg.org)

How it works in one sentence: you (or your coding agent) write what the
tutorial says and does, AutoDoc executes it the same way every time.

```
storyboard.yml  ->  TTS (cached per speech segment)  ->  Playwright replay
  ->  tutorial.mp4 + subtitles + thumbnail + tutorial.md
```

## Install

No Go toolchain needed.

> No stable release exists yet. Current pre-release: `v0.1.0-rc.2`.
> Install it explicitly pinned (a bare install resolves `latest stable`
> and refuses while only pre-releases exist):

```bash
curl -fsSL https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.sh | sh -s -- --version v0.1.0-rc.2
```

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.ps1 -OutFile install.ps1
.\install.ps1 -Version v0.1.0-rc.2
```

Details and FFmpeg setup: [docs/install.md](docs/install.md).
Developers can also build from source (`go build -o autodoc ./cmd/autodoc`).

## Quickstart

```bash
autodoc init              # writes ./autodoc.toml + example storyboard.yml
autodoc browser install   # Playwright browser assets (separate from the binary)
autodoc tts check         # one-sentence probe of the TTS endpoint
autodoc doctor            # full diagnostics; every failure names its fix
autodoc storyboard validate --storyboard storyboard.yml
autodoc record --storyboard storyboard.yml && autodoc render --storyboard storyboard.yml
autodoc export --storyboard storyboard.yml
```

The full walkthrough, including local TTS setup and your first real video,
is in [docs/getting-started.md](docs/getting-started.md).

## Who does what

- Your coding agent thinks: explores the app, finds locators and waits,
  authors the storyboard. Start it with `autodoc agent bootstrap`.
- AutoDoc executes: validates, synthesizes, replays, captures, renders.
  AutoDoc itself never calls an LLM.

## Capabilities

- Declarative storyboards: ordered `speech`/`action`/`wait`/`hold` events,
  no percentage-timing hacks. See [docs/storyboard.md](docs/storyboard.md).
- Cinematic recording by default: synthetic cursor, click cues, progressive
  typing, render-time close-ups. Directable in plain language or YAML.
  See [docs/cinematic.md](docs/cinematic.md).
- Segment TTS cache: only changed narration is re-synthesized.
  Local, remote, or disabled providers. See [docs/tts.md](docs/tts.md).
- Off-camera auth with dedicated profiles, secret scanning, DOM redaction.
  See [docs/security.md](docs/security.md).
- Skill plus MCP server for Codex, Claude Code, Antigravity, Gemini CLI,
  OpenCode (Cursor best-effort). See [docs/harnesses/](docs/harnesses/).

A rendered example ships in
[docs/autodoc/gerenciando-materiais/](docs/autodoc/gerenciando-materiais/).

## Docs

- [Install](docs/install.md) · [Getting started](docs/getting-started.md)
- [Storyboard](docs/storyboard.md) · [Cinematic](docs/cinematic.md)
- [TTS](docs/tts.md) · [Local TTS](docs/local-tts.md)
- [Browser and auth](docs/browser.md) · [Configuration](docs/config.md)
- [CLI reference](docs/cli.md) (generated from the CLI)
- [Troubleshooting](docs/troubleshooting.md) · [Security](docs/security.md)
- [Development](docs/development.md) · [Release](docs/release.md)
  · [Contributing](CONTRIBUTING.md)

## Development

```bash
gofmt -l ./cmd ./internal ./test
go vet ./...
go test ./...                        # unit + integration + smoke
go test ./test/e2e/ -count=1 -timeout 10m   # full pipeline, localhost only
```

Conventions: single Go binary, `CaptureBackend` interface over
`playwright-go`, per-scene recovery units, small semantic commits.
See [docs/development.md](docs/development.md).

---

*AutoDoc is the deterministic executor; your coding agent is the director.
Storyboard in, tutorial out, reproducibly.*
