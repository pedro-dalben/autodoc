# Troubleshooting

## `autodoc.toml not found`

Run `autodoc init` in the project root first, or `autodoc init --global`
once per machine so projects without `./autodoc.toml` inherit the global
file. `doctor` shows the active source.

## `ffmpeg not found` / missing `libx264` / `aac`

Install FFmpeg 6+ (`ffmpeg` and `ffprobe` on `PATH`) and re-run
`autodoc doctor`:

```bash
sudo apt install ffmpeg          # Debian/Ubuntu
brew install ffmpeg              # macOS
winget install Gyan.FFmpeg       # Windows
```

Custom paths without touching `PATH`:

```bash
export AUTODOC_FFMPEG=/opt/ffmpeg/bin/ffmpeg
export AUTODOC_FFPROBE=/opt/ffmpeg/bin/ffprobe
```

## Playwright: `chromium launch` / driver errors

Browser assets are separate from the Go binary:

```bash
autodoc browser check
autodoc browser install
```

`doctor` reports driver version, cached browsers, and launch health.
Headless CI works; `browser login` needs a display. On a headless server,
run it under `xvfb-run` or provision the profile's `storage-state.json`
before recording.

## TTS: endpoint errors / non-WAV

`openai-compatible` requires `<base_url>/audio/speech` returning WAV.
Check `base_url`, model and voice names, and `api_key_env`:

```bash
autodoc tts check
autodoc doctor
```

Use `provider = "disabled"` to isolate TTS from the rest of the pipeline.
Full setup: [local-tts.md](local-tts.md).

## `storyboard invalid` / secret scan failures

Read the bullet list: fix schema, locator, or timeout issues, remove any
credential-like values (use fixtures and `secret_ref: env:NAME`), then
re-validate.

## `timeline stale`

The storyboard changed after TTS. Re-run `autodoc tts`.

## `no rendered mp4 found`

`render` writes into the current run dir; `export` finds the latest matching
storyboard hash automatically. If you passed `--out` to `render`, pass the
same path via `export --mp4`.

## Flaky waits / missing elements

Explore with Playwright MCP first: confirm the locator (prefer `test_id`),
raise `timeout_ms`, add `settle_ms`, or wait on URL or networkidle after
submits.

## Installer failures

- `unsupported OS/architecture`: the installer supports Linux and macOS on
  amd64/arm64, plus Windows via `install.ps1`. No Go fallback is needed;
  on an unusual platform, build from source instead.
- `download failed ... not found`: bad version or no release published yet.
  Check the tag exists on the releases page.
- `checksum mismatch`: the download is corrupt or tampered with. The
  installer aborts before running anything. Retry; if it persists, report
  it with the version and platform.
- `could not resolve latest release`: network error or no release
  published yet. Pass an explicit `--version`.
