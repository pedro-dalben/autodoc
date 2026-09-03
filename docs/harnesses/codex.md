# Codex

AutoDoc support: full (detect → install → verify → own → uninstall-safe).

| Aspect | Detail |
| ------ | ------ |
| Detected | `~/.codex/config.toml` (or dir), `AGENTS.md` |
| Skill | `~/.codex/skills/autodoc/SKILL.md` (copy of canonical `src/skill/autodoc/SKILL.md`) |
| MCP | `autodoc` server merged into `[mcp_servers.autodoc]` in `~/.codex/config.toml` (owned flag; user servers untouched) |
| Rules block | `<!-- AUTODOC:BEGIN -->…<!-- AUTODOC:END -->` appended to `~/.codex/AGENTS.md` |

```bash
autodoc init --harness codex
autodoc doctor              # Harnesses → codex: configured
autodoc uninstall --harness codex
```

Re-running `init` is a zero-diff no-op. `uninstall` removes only the
AutoDoc-owned skill file, the marker block, and the owned `[mcp_servers.autodoc]`
entry (refuses entries it does not own), preserving your own edits.
