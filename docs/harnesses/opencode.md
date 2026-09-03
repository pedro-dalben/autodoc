# OpenCode

AutoDoc support: full (detect → install → verify → own → uninstall-safe).

| Aspect | Detail |
| ------ | ------ |
| Detected | `~/.config/opencode/opencode.json`, `opencode.jsonc` |
| Skill | `~/.config/opencode/skills/autodoc/SKILL.md` (copy of canonical `src/skill/autodoc/SKILL.md`) |
| MCP | `autodoc` server merged into `mcp.servers` in `opencode.json` as `{"type":"local","command":["autodoc","mcp"]}` (owned flag; user servers untouched) |

```bash
autodoc init --harness opencode
autodoc doctor     # Harnesses → opencode: configured
autodoc uninstall --harness opencode
```

Re-running `init` is a zero-diff no-op. `uninstall` removes only the
AutoDoc-owned skill file and the owned `mcp.servers.autodoc` entry (refuses to
remove entries it does not own), preserving your own servers and settings.
