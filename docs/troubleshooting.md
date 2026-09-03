# Troubleshooting

## `autodoc.toml not found`

Run `autodoc init` in the project root first — or `autodoc init --global`
once per machine so projects without `./autodoc.toml` inherit the global
`~/.config/autodoc/autodoc.toml`. `doctor` shows the active source.

## `ffmpeg not found` / missing `libx264`/`aac`

Install FFmpeg 6+ (`ffmpeg` + `ffprobe` on PATH) and re-run `autodoc doctor`.
`AUTODOC_FFMPEG` / `AUTODOC_FFPROBE` env vars override the binary paths.

## Playwright: `chromium launch` / driver errors

Browser assets are separate from the Go binary:

```bash
autodoc browser check
autodoc browser install
```

`doctor` reports driver version, cached browsers, and launch health.
Headless CI works; `browser login` needs a display (or `xvfb-run`).

## TTS: endpoint errors / non-WAV

`openai-compatible` requires `<base_url>/audio/speech` returning WAV.
Check `base_url`, model/voice names, and `api_key_env`. Use
`provider = "disabled"` to isolate TTS from the rest of the pipeline.

## `storyboard invalid` / secret scan failures

Read the bullet list — fix schema/locator/timeout issues, remove any
credential-like values (use fixtures), then re-validate.

## `timeline stale`

The storyboard changed after TTS. Re-run `autodoc tts`.

## `no rendered mp4 found`

`render` writes into the current run dir; `export` finds the latest matching
storyboard hash automatically. If you passed `--out` to `render`, pass the
same path via `export --mp4`.

## Flaky waits / missing elements

Explore with Playwright MCP first: confirm the locator (prefer `test_id`),
raise `timeout_ms`, add `settle_ms`, or wait on URL/networkidle after submits.
