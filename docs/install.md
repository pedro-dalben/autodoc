# Install

## Requirements

- Go 1.24+ (to build) or a released `autodoc` binary
- FFmpeg 6+ with `libx264` + `aac` encoders (`ffmpeg`, `ffprobe` on PATH)
- Playwright browser assets (installed via `autodoc browser install`)
- Optional: OpenAI-compatible TTS endpoint (or local Kokoro-compatible server),
  or `provider = "disabled"` for silent dry runs

## From source

```bash
go build -o autodoc ./cmd/autodoc
sudo install -m755 autodoc /usr/local/bin/autodoc
```

## Setup

```bash
autodoc init
```

`init` probes coding agents (Codex, Claude Code, Antigravity, Gemini CLI;
Cursor best-effort), installs the canonical skill + MCP entries it owns,
checks TTS/FFmpeg/browser, and writes `autodoc.toml` + example `storyboard.yml`.
Re-running `init` is idempotent (zero diff).

## Playwright browsers

`go install` alone does NOT make Playwright functional — browser assets are separate:

```bash
autodoc browser check
autodoc browser install
autodoc doctor
```

## TTS

Default is an OpenAI-compatible endpoint (works with local Kokoro servers and
with OpenAI itself). For offline/silent runs:

```toml
[tts]
provider = "disabled"
```

See [tts.md](tts.md) and [local-tts.md](local-tts.md).

## Uninstall

```bash
autodoc uninstall
```

Removes only AutoDoc-owned entries (manifest-tracked, marker-delimited).
Your own edits are preserved. `.bak` files are recovery-only; automatic restore
requires explicit `--restore-backup`.
