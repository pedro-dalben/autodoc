package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func All() []*Harness {
	return []*Harness{Codex(), Claude(), Antigravity(), Gemini(), Cursor(), OpenCode()}
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
			if mcpCommand != "" {
				mp := expandHome(home, "~/.codex/config.toml")
				ok, err := installCodexMCP(mp, mcpCommand)
				if err != nil {
					return changed, err
				}
				if ok {
					changed = append(changed, mp)
				}
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
			var removed []string
			r1, err := uninstallPaths(home, []string{
				"~/.codex/skills/autodoc/SKILL.md",
			}, []string{"~/.codex/AGENTS.md"})
			if err != nil {
				return removed, err
			}
			removed = append(removed, r1...)
			mp := expandHome(home, "~/.codex/config.toml")
			if out, changed, err := removeCodexMCPServer(mp, "autodoc"); err == nil && changed {
				_ = os.WriteFile(mp, out, 0o600)
				removed = append(removed, mp)
			}
			return removed, nil
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
			"~/.gemini/config/mcp_config.json",
		},
		SkillPaths: []string{
			"~/.config/antigravity/skills/autodoc/SKILL.md",
			"~/.gemini/antigravity/skills/autodoc/SKILL.md",
			"~/.agents/skills/autodoc/SKILL.md",
		},
		MCPPath: "~/.gemini/config/mcp_config.json",
		Probe: func(home string) ProbeResult {
			var found []string
			for _, p := range []string{
				"~/.config/antigravity/config.json",
				"~/.antigravity/config.json",
				"~/.config/google/antigravity.json",
				"~/.antigravity/argv.json",
				"~/.gemini/config/mcp_config.json",
			} {
				if ep := expandHome(home, p); fileExists(ep) {
					found = append(found, ep)
				}
			}
			skill := false
			for _, p := range []string{
				"~/.config/antigravity/skills/autodoc/SKILL.md",
				"~/.gemini/antigravity/skills/autodoc/SKILL.md",
				"~/.agents/skills/autodoc/SKILL.md",
			} {
				if fileExists(expandHome(home, p)) {
					skill = true
				}
			}
			mcp := false
			if data, err := os.ReadFile(expandHome(home, "~/.gemini/config/mcp_config.json")); err == nil {
				if strings.Contains(string(data), "autodoc") {
					mcp = true
				}
			}
			pr := ProbeResult{Found: len(found) > 0, Paths: found}
			switch {
			case skill && mcp:
				pr.Configured = true
			case skill || mcp:
				pr.Partial = true
			}
			return pr
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			for _, p := range []string{
				"~/.config/antigravity/skills/autodoc/SKILL.md",
				"~/.gemini/antigravity/skills/autodoc/SKILL.md",
				"~/.agents/skills/autodoc/SKILL.md",
			} {
				sp := expandHome(home, p)
				if did, err := installSkillFile(sp, skillSource); err != nil {
					return changed, err
				} else if did {
					changed = append(changed, sp)
				}
			}
			if mcpCommand != "" {
				mp := expandHome(home, "~/.gemini/config/mcp_config.json")
				ok, err := installAntigravityMCP(mp, mcpCommand)
				if err != nil {
					return changed, err
				}
				if ok {
					changed = append(changed, mp)
				}
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			removed, err := uninstallPaths(home, []string{
				"~/.config/antigravity/skills/autodoc/SKILL.md",
				"~/.gemini/antigravity/skills/autodoc/SKILL.md",
				"~/.agents/skills/autodoc/SKILL.md",
			}, nil)
			if err != nil {
				return removed, err
			}
			mp := expandHome(home, "~/.gemini/config/mcp_config.json")
			if data, err := os.ReadFile(mp); err == nil {
				if out, changed, err := removeAntigravityMCPServer(data, "autodoc"); err == nil && changed {
					_ = os.WriteFile(mp, out, 0o644)
					removed = append(removed, mp)
				}
			}
			return removed, nil
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

func OpenCode() *Harness {
	return &Harness{
		Name:        "opencode",
		Kind:        "file+config",
		DetectPaths: []string{"~/.config/opencode/opencode.json", "~/.config/opencode/opencode.jsonc"},
		SkillPaths:  []string{"~/.config/opencode/skills/autodoc/SKILL.md"},
		MCPPath:     "~/.config/opencode/opencode.json",
		Probe: func(home string) ProbeResult {
			return probeMarkdownSkill(home,
				[]string{"~/.config/opencode/opencode.json", "~/.config/opencode/opencode.jsonc"},
				[]string{"~/.config/opencode/skills/autodoc/SKILL.md"},
				"~/.config/opencode/opencode.json")
		},
		Install: func(home, skillSource, mcpCommand string) ([]string, error) {
			var changed []string
			sp := expandHome(home, "~/.config/opencode/skills/autodoc/SKILL.md")
			if did, err := installSkillFile(sp, skillSource); err != nil {
				return changed, err
			} else if did {
				changed = append(changed, sp)
			}
			if mcpCommand != "" {
				mp := expandHome(home, "~/.config/opencode/opencode.json")
				ok, err := installOpenCodeMCP(mp, mcpCommand)
				if err != nil {
					return changed, err
				}
				if ok {
					changed = append(changed, mp)
				}
			}
			return changed, nil
		},
		Uninstall: func(home string) ([]string, error) {
			var removed []string
			r1, err := uninstallPaths(home, []string{"~/.config/opencode/skills/autodoc/SKILL.md"}, nil)
			if err != nil {
				return removed, err
			}
			removed = append(removed, r1...)
			mp := expandHome(home, "~/.config/opencode/opencode.json")
			if data, err := os.ReadFile(mp); err == nil {
				if out, changed, err := removeOpenCodeMCPServer(data, "autodoc"); err == nil && changed {
					_ = os.WriteFile(mp, out, 0o644)
					removed = append(removed, mp)
				}
			}
			return removed, nil
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

func installCodexMCP(path, command string) (bool, error) {
	var orig []byte
	if data, err := os.ReadFile(path); err == nil {
		orig = data
	}
	out, changed, err := mergeCodexMCPTOML(orig, "autodoc", command)
	if err != nil {
		return false, fmt.Errorf("merge %s: %w", path, err)
	}
	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o600)
}

func mergeCodexMCPTOML(original []byte, serverName, command string) ([]byte, bool, error) {
	text := string(original)
	section := "[mcp_servers." + serverName + "]"
	if strings.Contains(text, section) {
		if codexTOMLHasCommand(text, section, command) {
			return nil, false, nil
		}
		updated, ok := codexTOMLReplaceCommand(text, section, command)
		if !ok {
			return nil, false, fmt.Errorf("existing [mcp_servers.%s] managed externally; refusing to overwrite", serverName)
		}
		return []byte(updated), true, nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	var b strings.Builder
	b.WriteString(text)
	b.WriteString("\n" + section + "\n")
	b.WriteString("command = " + tomlQuote(command) + "\n")
	b.WriteString("args = [\"mcp\"]\n")
	b.WriteString("# _autodoc_owned = true\n")
	return []byte(b.String()), true, nil
}

func codexTOMLSectionBody(text, section string) string {
	idx := strings.Index(text, section)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(section):]
	next := strings.Index(rest, "\n[")
	if next >= 0 {
		return rest[:next]
	}
	return rest
}

func codexTOMLHasCommand(text, section, command string) bool {
	body := codexTOMLSectionBody(text, section)
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "command") && strings.Contains(t, command) {
			return true
		}
	}
	return false
}

func codexTOMLReplaceCommand(text, section, command string) (string, bool) {
	idx := strings.Index(text, section)
	if idx < 0 {
		return text, false
	}
	rest := text[idx+len(section):]
	next := strings.Index(rest, "\n[")
	var body, tail string
	if next >= 0 {
		body = rest[:next]
		tail = rest[next:]
	} else {
		body = rest
	}
	if !strings.Contains(body, "_autodoc_owned") {
		return text, false
	}
	lines := strings.Split(body, "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "command") {
			lines[i] = "command = " + tomlQuote(command)
			replaced = true
			break
		}
	}
	if !replaced {
		return text, false
	}
	return text[:idx+len(section)] + strings.Join(lines, "\n") + tail, true
}

func tomlQuote(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
}

func removeCodexMCPServer(path, serverName string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, nil
	}
	text := string(data)
	section := "[mcp_servers." + serverName + "]"
	idx := strings.Index(text, section)
	if idx < 0 {
		return nil, false, nil
	}
	body := codexTOMLSectionBody(text, section)
	if !strings.Contains(body, "_autodoc_owned") {
		return nil, false, fmt.Errorf("refusing to remove non-AutoDoc-owned server %q", serverName)
	}
	rest := text[idx+len(section):]
	next := strings.Index(rest, "\n[")
	var tail string
	if next >= 0 {
		tail = rest[next:]
	}
	out := strings.TrimRight(text[:idx], "\n") + "\n" + strings.TrimLeft(tail, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out), true, nil
}

func installGeminiMCP(path, command string) (bool, error) {
	return installGenericSettingsMCP(path, command)
}

func installAntigravityMCP(path, command string) (bool, error) {
	var orig []byte
	if data, err := os.ReadFile(path); err == nil {
		orig = data
	}
	cfg := map[string]any{"command": command, "args": []string{"mcp"}}
	out, changed, err := mergeAntigravityMCPServers(orig, "autodoc", cfg)
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

func mergeAntigravityMCPServers(original []byte, serverName string, serverCfg map[string]any) ([]byte, bool, error) {
	var root map[string]any
	if len(original) == 0 {
		root = map[string]any{}
	} else {
		if err := json.Unmarshal(original, &root); err != nil {
			return nil, false, fmt.Errorf("invalid JSON: %w", err)
		}
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		root["mcpServers"] = servers
	}
	if existing, ok := servers[serverName].(map[string]any); ok {
		if fmt.Sprint(existing["command"]) == fmt.Sprint(serverCfg["command"]) {
			return nil, false, nil
		}
	}
	owned := map[string]any{}
	for k, v := range serverCfg {
		owned[k] = v
	}
	owned["_autodoc_owned"] = true
	servers[serverName] = owned
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

func removeAntigravityMCPServer(original []byte, serverName string) ([]byte, bool, error) {
	if len(original) == 0 {
		return original, false, nil
	}
	var root map[string]any
	if err := json.Unmarshal(original, &root); err != nil {
		return nil, false, fmt.Errorf("invalid JSON: %w", err)
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		return original, false, nil
	}
	entry, ok := servers[serverName].(map[string]any)
	if !ok {
		return original, false, nil
	}
	if owned, _ := entry["_autodoc_owned"].(bool); !owned {
		return nil, false, fmt.Errorf("refusing to remove non-AutoDoc-owned server %q", serverName)
	}
	delete(servers, serverName)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
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

func installOpenCodeMCP(path, command string) (bool, error) {
	var orig []byte
	if data, err := os.ReadFile(path); err == nil {
		orig = data
	}
	cfg := map[string]any{"type": "local", "command": []string{command, "mcp"}}
	out, changed, err := mergeOpenCodeMCPServers(orig, "autodoc", cfg)
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

func mergeOpenCodeMCPServers(original []byte, serverName string, serverCfg map[string]any) ([]byte, bool, error) {
	var root map[string]any
	if len(original) == 0 {
		root = map[string]any{}
	} else {
		if err := json.Unmarshal(original, &root); err != nil {
			return nil, false, fmt.Errorf("invalid JSON: %w", err)
		}
	}
	mcp, _ := root["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
		root["mcp"] = mcp
	}
	servers, _ := mcp["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		mcp["servers"] = servers
	}
	if existing, ok := servers[serverName].(map[string]any); ok {
		if fmt.Sprint(existing["command"]) == fmt.Sprint(serverCfg["command"]) &&
			fmt.Sprint(existing["type"]) == fmt.Sprint(serverCfg["type"]) {
			return nil, false, nil
		}
	}
	owned := map[string]any{}
	for k, v := range serverCfg {
		owned[k] = v
	}
	owned["_autodoc_owned"] = true
	servers[serverName] = owned
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

func removeOpenCodeMCPServer(original []byte, serverName string) ([]byte, bool, error) {
	if len(original) == 0 {
		return original, false, nil
	}
	var root map[string]any
	if err := json.Unmarshal(original, &root); err != nil {
		return nil, false, fmt.Errorf("invalid JSON: %w", err)
	}
	mcp, _ := root["mcp"].(map[string]any)
	if mcp == nil {
		return original, false, nil
	}
	servers, _ := mcp["servers"].(map[string]any)
	if servers == nil {
		return original, false, nil
	}
	entry, ok := servers[serverName].(map[string]any)
	if !ok {
		return original, false, nil
	}
	if owned, _ := entry["_autodoc_owned"].(bool); !owned {
		return nil, false, fmt.Errorf("refusing to remove non-AutoDoc-owned server %q", serverName)
	}
	delete(servers, serverName)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
