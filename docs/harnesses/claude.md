# Claude Code

AutoDoc support: full (detect → install → verify → own → uninstall-safe).

| Aspect | Detail |
| ------ | ------ |
| Detected | `~/.claude.json`, `~/.claude/settings.json`, `CLAUDE.md` |
| Skill | `~/.claude/skills/autodoc/SKILL.md` (copy of canonical skill) |
| MCP | `autodoc` server merged into `mcpServers` in `~/.claude.json` (owned flag; user servers untouched) |
| Rules block | `<!-- AUTODOC:BEGIN -->…<!-- AUTODOC:END -->` in `~/.claude/CLAUDE.md` |

```bash
autodoc init --harness claude
autodoc doctor
autodoc uninstall --harness claude
```

Uninstall refuses to remove MCP entries it does not own, and keeps every line
you added outside the marker block.
