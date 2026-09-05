# Contributing

- `gofmt -l ./cmd ./internal ./test` must be clean.
- `go vet ./...` must pass.
- `go test ./...` must pass; `./test/e2e` needs ffmpeg/browsers (localhost only, no paid APIs).
- One storyboard source of truth: `storyboard.yml` in, `_work/recipe.json` generated. Never hand-edit generated files. Same rule for `docs/cli.md`: regenerate, do not hand-edit.
- Locator order in new fixtures/docs: `test_id` → `role`+`name` → `label` → stable attribute → CSS fallback.
- No secrets in code, storyboards, tests, or logs. Fixture credentials only (`demo`/`demo1234` style).
- Installer changes must keep: idempotent re-run (zero diff), uninstall removes only AutoDoc-owned entries, user edits preserved.
- User-facing prose stays plain: short sentences, no hype words, no emojis, no placeholder claims. Commands, flags, paths, and config in docs must match the CLI; `test/smoke` checks this.
- Small, semantic commits on feature branches; keep `git status` clean.
