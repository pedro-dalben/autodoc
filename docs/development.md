# Development

```bash
gofmt -l ./cmd ./internal ./test
go vet ./...
go test ./...
go test ./test/e2e/ -v -timeout 10m   # needs ffmpeg+ffprobe+browsers, localhost only
git diff --check
git status
```

`go test ./...` includes `test/smoke` (installer runs against a local
`file://` release tree, doc link and quickstart checks) and the
`docs/cli.md` drift test in `internal/cli`. Regenerate the CLI reference
with `AUTODOC_UPDATE_CLI_DOCS=1 go test ./internal/cli/ -run TestCliDocs`.

Layout: `cmd/autodoc` (CLI) · `internal/storyboard|recipe|config|tts|
timeline|capture|media|pipeline|harness|install|doctor|mcp|cli|version` ·
`src/skill/autodoc/SKILL.md` (canonical skill; adapters are copies) ·
`test/fixture` (Go/HTML app: login, dashboard, sidebar, list, form, select,
modal, validation error, success, delayed loading) · `test/faketts`
(deterministic fake OpenAI TTS) · `test/e2e` (full pipeline test) ·
`test/smoke` (installer + docs checks, no network).

Conventions: single Go binary; `CaptureBackend` interface over
`playwright-go >= v0.6100.0` (`recordVideo`, no Node sidecar); per-scene
recovery (`scene-*.webm` + `events-*.jsonl` before final render); intermediates
in `.autodoc/_work/<run-id>/`, publishables in `docs/autodoc/<tutorial>/`;
installer owns only manifest-tracked, marker-delimited entries (idempotent
re-run = zero diff; uninstall removes only AutoDoc-owned).

Releases: tag `vX.Y.Z`, the `release` workflow gates then publishes via
GoReleaser. See [release.md](release.md).
