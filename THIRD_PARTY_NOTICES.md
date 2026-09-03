# Third-party notices

Direct Go dependencies (see `go.mod` / `go.sum` for exact versions):

- `github.com/mxschmitt/playwright-go` (pinned `>= v0.6100.0`, currently
  `v0.6201.1`) — Apache-2.0 / MIT (dual, per upstream). Drives the bundled
  Playwright driver + browser assets at runtime.
- `github.com/spf13/cobra`, `github.com/spf13/pflag`,
  `github.com/inconshreveable/mousetrap` — Apache-2.0.
- `github.com/pelletier/go-toml/v2` — MIT.
- `gopkg.in/yaml.v3` — MIT/Apache-2.0 (dual, per upstream).

External runtime dependencies (NOT redistributed):

- **FFmpeg** (`ffmpeg`/`ffprobe`, LGPL/GPL depending on build) — must be
  installed on the host; AutoDoc shells out to it.
- **Playwright browsers/driver** — downloaded on demand by
  `autodoc browser install`; governed by Playwright/Microsoft licenses.
- **TTS models/weights** (e.g. Kokoro) and any TTS server — governed by their
  own licenses; AutoDoc only speaks HTTP to them.

Run `go-licenses`-style audits before releases; do not vendor browser builds
or model weights into release artifacts without confirming redistribution rights.
