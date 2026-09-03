# Development

```bash
gofmt -l ./cmd ./internal ./test
go vet ./...
go test ./...
go test ./test/e2e/ -v -timeout 10m   # needs ffmpeg+ffprobe+browsers, localhost only
git diff --check
git status
```

Layout: `cmd/autodoc` (CLI) · `internal/storyboard|recipe|config|tts|
timeline|capture|media|pipeline|harness|install|doctor|mcp|cli|version` ·
`src/skill/autodoc/SKILL.md` (canonical skill; adapters are copies) ·
`test/fixture` (Go/HTML app: login, dashboard, sidebar, list, form, select,
modal, validation error, success, delayed loading) · `test/faketts`
(deterministic fake OpenAI TTS) · `test/e2e` (full pipeline test).

Conventions: single Go binary; `CaptureBackend` interface over
`playwright-go >= v0.6100.0` (`recordVideo`, no Node sidecar); per-scene
recovery (`scene-*.webm` + `events-*.jsonl` before final render); intermediates
in `.autodoc/_work/<run-id>/`, publishables in `docs/autodoc/<tutorial>/`;
installer owns only manifest-tracked, marker-delimited entries (idempotent
re-run = zero diff; uninstall removes only AutoDoc-owned).
