# Gemini CLI

AutoDoc support: full.

| Aspect | Detail |
| ------ | ------ |
| Detected | `~/.gemini/settings.json`, `GEMINI.md` |
| Skill | `~/.gemini/skills/autodoc/SKILL.md` (copy of canonical skill) |
| MCP | `autodoc` server merged into `mcpServers` in `~/.gemini/settings.json` |
| Rules block | marker block in `~/.gemini/GEMINI.md` |

```bash
autodoc init --harness gemini
autodoc doctor
autodoc uninstall --harness gemini
```

Idempotent re-runs; uninstall preserves non-AutoDoc settings and user prose.
