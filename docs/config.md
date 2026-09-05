# Configuration

Every command resolves config as project, then global, then built-in
defaults. A project file always wins; delete it (or run outside the project)
to fall back to global.

1. `./autodoc.toml`, walking upward from the cwd (project root).
2. `~/.config/autodoc/autodoc.toml` (`$AUTODOC_CONFIG_HOME`, else
   `$XDG_CONFIG_HOME/autodoc/`).
3. Built-in defaults (used only when neither file exists).

`doctor` reports which source is active (`project` or `global`).
`init --global` writes the machine file and never writes `storyboard.yml`.
Only settings that differ per machine belong in global (TTS endpoint, voice,
browser profile); storyboards always live in the project.

## Reference

```toml
[project]
name = "my-tutorial"
language = "pt-BR"          # default: pt-BR

[tts]
provider = "openai-compatible"  # or "disabled"
base_url = "http://localhost:8880/v1"
api_key_env = ""            # env var holding the key, when needed
model = "kokoro"
voice = "af_bella"
language = "pt-BR"
speed = 1.0                 # (0, 4]
response_format = "wav"

[media]
resolution = "1920x1080"    # default
fps = 30                    # default
codec = "libx264"           # default

[browser]
profile = "docs"            # default
headless = true             # default
viewport_width = 1280       # default
viewport_height = 720       # default
storage_state = ""          # optional path to storage-state.json

[paths]
storyboard = "storyboard.yml"   # default
work_dir = ".autodoc/_work"     # default
output_dir = "docs/autodoc"     # default
```

Validation rules: `tts.provider` must be `openai-compatible` or `disabled`;
`openai-compatible` requires `base_url` and `model`; `speed` must be within
`(0, 4]`. `init` refuses to write invalid config.

## Environment overrides

- `AUTODOC_FFMPEG` / `AUTODOC_FFPROBE`: binary paths when they are not on `PATH`.
- `AUTODOC_CONFIG_HOME`: relocation of the global config file.
- `AUTODOC_SKILL_SOURCE`: override of the canonical skill path (dev only).

Secrets stay in env vars (`tts.api_key_env`, `secret_ref: env:NAME` in
storyboards). `doctor` redacts secrets in all output. Never commit keys.
