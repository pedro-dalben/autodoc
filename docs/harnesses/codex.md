# Codex

AutoDoc support: full (detect → install → verify → own → uninstall-safe).

| Aspect | Detail |
| ------ | ------ |
| Detected | `~/.codex/config.toml` (or dir), `AGENTS.md` |
| Skill | `~/.codex/skills/autodoc/SKILL.md` (copy of canonical `src/skill/autodoc/SKILL.md`) |
| MCP | MCP command recorded at install; entry merged into Codex config where supported |
| Rules block | `<!-- AUTODOC:BEGIN -->…<!-- AUTODOC:END -->` appended to `~/.codex/AGENTS.md` |

```bash
autodoc init --harness codex
autodoc doctor              # Harnesses → codex: configured
autodoc uninstall --harness codex
```

Re-running `init` is a zero-diff no-op. `uninstall` removes only the
AutoDoc-owned skill file and the marker block, preserving your own edits.
