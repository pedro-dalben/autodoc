package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func All() []*Harness {
	return []*Harness{Codex(), Claude(), Antigravity(), Gemini(), Cursor()}
}

func ByName(name string) *Harness {
	for _, h := range All() {
		if h.Name == name {
			return h
		}
	}
	return nil
}

func skillBlock(harnessName string) string {
	return MarkerBegin + "\n## AutoDoc — Audiovisual Documentation\n\n" +
		"AutoDoc skill installed. See canonical skill for the audiovisual documentation workflow.\n\n" +
		"- Skill: `autodoc`\n" +
		"- Harness: " + harnessName + "\n" +
		"- MCP server: `autodoc` (stdio)\n\n" +
		"Keep this block managed by `autodoc init` / `autodoc uninstall`.\n" +
		MarkerEnd + "\n"
}

func Codex() *Harness {
	return &Harness{
		Name:        "codex",
		Kind:        "file+config",
		DetectPaths: []string{"~/.codex/config.toml"},
		SkillPaths:  []string{"~/.codex/skills/autodoc/SKILL.md"},
		MCPPath:     "~/.codex/config.toml",
		Probe: func(home string) ProbeResult {
			return probeMarkdownSkill(home,
				[]string{"~/.codex/config.toml", "~/.codex/AGENTS.md"},
				[]string{"~/.codex/skills/autodoc/SKILL.md"},
				"~/.codex/config.toml")
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.codex/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			ap := expandHome(home, "~/.codex/AGENTS.md")
			if ok, err := installMarkdownBlock(ap, skillBlock("codex")); err != nil {
				return changed, err
			} else if ok {
				changed = append(changed, ap)
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			return uninstallPaths(home, []string{
				"~/.codex/skills/autodoc/SKILL.md",
			}, []string{"~/.codex/AGENTS.md"})
		},
	}
}

func Claude() *Harness {
	return &Harness{
		Name:        "claude",
		Kind:        "file+config",
		DetectPaths: []string{"~/.claude.json", "~/.claude/settings.json"},
		SkillPaths:  []string{"~/.claude/skills/autodoc/SKILL.md"},
		MCPPath:     "~/.claude.json",
		Probe: func(home string) ProbeResult {
			return probeMarkdownSkill(home,
				[]string{"~/.claude.json", "~/.claude/CLAUDE.md"},
				[]string{"~/.claude/skills/autodoc/SKILL.md"},
				"~/.claude.json")
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.claude/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			if mcpCommand != "" {
				mp := expandHome(home, "~/.claude.json")
				ok, err := installClaudeMCP(mp, mcpCommand)
				if err != nil {
					return changed, err
				}
				if ok {
					changed = append(changed, mp)
				}
			}
			cp := expandHome(home, "~/.claude/CLAUDE.md")
			if ok, err := installMarkdownBlock(cp, skillBlock("claude")); err != nil {
				return changed, err
			} else if ok {
				changed = append(changed, cp)
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			var removed []string
			r1, err := uninstallPaths(home, []string{"~/.claude/skills/autodoc/SKILL.md"}, []string{"~/.claude/CLAUDE.md"})
			if err != nil {
				return removed, err
			}
			removed = append(removed, r1...)
			mp := expandHome(home, "~/.claude.json")
			if data, err := os.ReadFile(mp); err == nil {
				if out, changed, err := removeJSONMCPServer(data, "autodoc"); err == nil && changed {
					_ = os.WriteFile(mp, out, 0o644)
					removed = append(removed, mp)
				}
			}
			return removed, nil
		},
	}
}

func Antigravity() *Harness {
	return &Harness{
		Name: "antigravity",
		Kind: "probe+skill",
		DetectPaths: []string{
			"~/.config/antigravity/config.json",
			"~/.antigravity/config.json",
			"~/.config/google/antigravity.json",
			"~/.antigravity/argv.json",
		},
		SkillPaths: []string{"~/.config/antigravity/skills/autodoc/SKILL.md"},
		MCPPath:    "~/.config/antigravity/mcp.json",
		Probe: func(home string) ProbeResult {
			var found []string
			for _, p := range []string{
				"~/.config/antigravity/config.json",
				"~/.antigravity/config.json",
				"~/.config/google/antigravity.json",
				"~/.antigravity/argv.json",
			} {
				if ep := expandHome(home, p); fileExists(ep) {
					found = append(found, ep)
				}
			}
			sp := expandHome(home, "~/.config/antigravity/skills/autodoc/SKILL.md")
			skill := fileExists(sp)
			return ProbeResult{Found: len(found) > 0, Configured: len(found) > 0 && skill, Partial: len(found) > 0 && !skill, Paths: found}
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.config/antigravity/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			return uninstallPaths(home, []string{"~/.config/antigravity/skills/autodoc/SKILL.md"}, nil)
		},
	}
}

func Gemini() *Harness {
	return &Harness{
		Name:        "gemini",
		Kind:        "file+config",
		DetectPaths: []string{"~/.gemini/settings.json"},
		SkillPaths:  []string{"~/.gemini/skills/autodoc/SKILL.md"},
		MCPPath:     "~/.gemini/settings.json",
		Probe: func(home string) ProbeResult {
			return probeMarkdownSkill(home,
				[]string{"~/.gemini/settings.json", "~/.gemini/GEMINI.md"},
				[]string{"~/.gemini/skills/autodoc/SKILL.md"},
				"~/.gemini/settings.json")
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.gemini/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			if mcpCommand != "" {
				mp := expandHome(home, "~/.gemini/settings.json")
				ok, err := installGeminiMCP(mp, mcpCommand)
				if err != nil {
					return changed, err
				}
				if ok {
					changed = append(changed, mp)
				}
			}
			gp := expandHome(home, "~/.gemini/GEMINI.md")
			if ok, err := installMarkdownBlock(gp, skillBlock("gemini")); err != nil {
				return changed, err
			} else if ok {
				changed = append(changed, gp)
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			var removed []string
			r1, err := uninstallPaths(home, []string{"~/.gemini/skills/autodoc/SKILL.md"}, []string{"~/.gemini/GEMINI.md"})
			if err != nil {
				return removed, err
			}
			removed = append(removed, r1...)
			mp := expandHome(home, "~/.gemini/settings.json")
			if data, err := os.ReadFile(mp); err == nil {
				if out, changed, err := removeJSONMCPServer(data, "autodoc"); err == nil && changed {
					_ = os.WriteFile(mp, out, 0o644)
					removed = append(removed, mp)
				}
			}
			return removed, nil
		},
	}
}

func Cursor() *Harness {
	return &Harness{
		Name:        "cursor",
		Kind:        "best-effort",
		DetectPaths: []string{"~/.cursor/mcp.json"},
		SkillPaths:  []string{"~/.cursor/skills/autodoc/SKILL.md"},
		MCPPath:     "~/.cursor/mcp.json",
		Probe: func(home string) ProbeResult {
			return probeMarkdownSkill(home,
				[]string{"~/.cursor/mcp.json"},
				[]string{"~/.cursor/skills/autodoc/SKILL.md"},
				"~/.cursor/mcp.json")
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.cursor/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			return uninstallPaths(home, []string{"~/.cursor/skills/autodoc/SKILL.md"}, nil)
		},
	}
}

func probeMarkdownSkill(home string, detect, skills []string, mcp string) ProbeResult {
	var found []string
	for _, p := range detect {
		if ep := expandHome(home, p); fileExists(ep) || fileExists(filepath.Dir(ep)) {
			found = append(found, ep)
		}
	}
	skillFound := false
	for _, p := range skills {
		if fileExists(expandHome(home, p)) {
			skillFound = true
		}
	}
	mcpFound := false
	if mcp != "" {
		if c := readFile(expandHome(home, mcp)); strings.Contains(c, "autodoc") {
			mcpFound = true
		}
	}
	pr := ProbeResult{Found: len(found) > 0, Paths: found}
	switch {
	case !pr.Found && !skillFound:
		pr.Found = false
	case skillFound && (mcp == "" || mcpFound || len(found) == 0):
		pr.Configured = true
		pr.Found = true
	case skillFound || mcpFound:
		pr.Partial = true
		pr.Found = true
	}
	_ = mcpFound
	return pr
}

func installSkillFile(dest, source string) (bool, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return false, fmt.Errorf("read canonical skill %s: %w", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false, err
	}
	if existing, err := os.ReadFile(dest); err == nil && string(existing) == string(data) {
		return false, nil
	}
	return true, os.WriteFile(dest, data, 0o644)
}

func installMarkdownBlock(path, block string) (bool, error) {
	orig := ""
	if data, err := os.ReadFile(path); err == nil {
		orig = string(data)
	} else if !os.IsNotExist(err) {
		return false, err
	}
	merged, changed := mergeMarkdownBlock(orig, block)
	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(merged), 0o644)
}

func uninstallPaths(home string, files, markdowns []string) ([]string, error) {
	var removed []string
	for _, p := range files {
		ep := expandHome(home, p)
		if err := os.Remove(ep); err == nil {
			removed = append(removed, ep)
		} else if !os.IsNotExist(err) {
			return removed, err
		}
	}
	for _, p := range markdowns {
		ep := expandHome(home, p)
		data, err := os.ReadFile(ep)
		if err != nil {
			continue
		}
		if out, changed := removeMarkdownBlock(string(data)); changed {
			if err := os.WriteFile(ep, []byte(out), 0o644); err != nil {
				return removed, err
			}
			removed = append(removed, ep)
		}
	}
	return removed, nil
}

func installClaudeMCP(path, command string) (bool, error) {
	return installGenericSettingsMCP(path, command)
}

func installGeminiMCP(path, command string) (bool, error) {
	return installGenericSettingsMCP(path, command)
}

func installGenericSettingsMCP(path, command string) (bool, error) {
	var orig []byte
	if data, err := os.ReadFile(path); err == nil {
		orig = data
	}
	cfg := map[string]any{"command": command, "args": []string{"mcp"}}
	out, changed, err := mergeJSONMCPServers(orig, "autodoc", cfg)
	if err != nil {
		return false, fmt.Errorf("merge %s: %w", path, err)
	}
	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}
