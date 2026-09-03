# Antigravity

AutoDoc support: full via real path probes (no assumed locations).

Probed locations (first hit wins for status; skill installs to the primary):

- `~/.config/antigravity/config.json`
- `~/.antigravity/config.json`
- `~/.config/google/antigravity.json`
- `~/.antigravity/argv.json` (VS Code-family IDE install marker — counts as
  "found" so `doctor` reports Antigravity honestly instead of "not found")
- skill → `~/.config/antigravity/skills/autodoc/SKILL.md`
- MCP → `~/.config/antigravity/mcp.json` (where the runtime reads it)

```bash
autodoc init --harness antigravity
autodoc doctor     # found → configured once the skill lands
autodoc uninstall --harness antigravity
```

If none of the probe paths exist, `doctor` reports `not found` with the exact
paths checked, and `init` still installs the skill to the primary location so
a later Antigravity install picks it up.
