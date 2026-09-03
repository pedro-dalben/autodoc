# Antigravity

AutoDoc support: full (detect → install → verify → own → uninstall-safe).

Probed locations (first hit wins for status; skill installs to all primaries):

- `~/.config/antigravity/config.json`
- `~/.antigravity/config.json`
- `~/.config/google/antigravity.json`
- `~/.antigravity/argv.json` (VS Code-family IDE install marker — counts as
  "found" so `doctor` reports Antigravity honestly instead of "not found")
- `~/.gemini/config/mcp_config.json` (where the `agy` CLI actually reads MCP)
- skill → `~/.config/antigravity/skills/autodoc/SKILL.md`
- skill → `~/.gemini/antigravity/skills/autodoc/SKILL.md` (read by `agy -p`)
- skill → `~/.agents/skills/autodoc/SKILL.md` (shared agent skills dir)
- MCP → `mcpServers.autodoc` in `~/.gemini/config/mcp_config.json`
  (`{"command":"<autodoc>","args":["mcp"]}`, owned flag; user servers untouched;
  manageable via `agy mcp list`)

```bash
autodoc init --harness antigravity
autodoc doctor     # found → configured once skill+MCP land
autodoc uninstall --harness antigravity
```

If none of the probe paths exist, `doctor` reports `not found` with the exact
paths checked, and `init` still installs the skill to the primary locations so
a later Antigravity install picks it up.
