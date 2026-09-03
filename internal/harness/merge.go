package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MarkerBegin = "<!-- AUTODOC:BEGIN -->"
	MarkerEnd   = "<!-- AUTODOC:END -->"
	JSONBegin   = "AUTODOC:BEGIN"
	JSONEnd     = "AUTODOC:END"
)

type Status string

const (
	StatusConfigured Status = "configured"
	StatusPartial    Status = "partial"
	StatusInvalid    Status = "invalid"
	StatusNotFound   Status = "not-found"
)

type Harness struct {
	Name        string
	DetectPaths []string
	SkillPaths  []string
	MCPPath     string
	Kind        string
	Probe       func(home string) ProbeResult
	Install     func(home, skillSource, mcpCommand string) (changed []string, err error)
	Uninstall   func(home string) (removed []string, err error)
}

type ProbeResult struct {
	Found      bool
	Configured bool
	Partial    bool
	Invalid    string
	Paths      []string
}

func HomeDir() string {
	if v := os.Getenv("AUTODOC_TEST_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return h
}

func expandHome(home, p string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func backupOnce(path string, backups map[string]string) error {
	if _, ok := backups[path]; ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	bak := path + ".autodoc-bak"
	if err := os.WriteFile(bak, data, 0o644); err != nil {
		return err
	}
	backups[path] = bak
	return nil
}

func mergeMarkdownBlock(original, block string) (string, bool) {
	if strings.Contains(original, MarkerBegin) && strings.Contains(original, MarkerEnd) {
		start := strings.Index(original, MarkerBegin)
		end := strings.Index(original, MarkerEnd) + len(MarkerEnd)
		inner := original[start:end]
		if strings.Contains(inner, strings.TrimSpace(blockInner(block))) {
			return original, false
		}
		merged := original[:start] + block + original[end:]
		return merged, true
	}
	if original != "" && !strings.HasSuffix(original, "\n") {
		original += "\n"
	}
	return original + "\n" + block, true
}

func blockInner(block string) string {
	s := strings.ReplaceAll(block, MarkerBegin, "")
	s = strings.ReplaceAll(s, MarkerEnd, "")
	return strings.TrimSpace(s)
}

func removeMarkdownBlock(original string) (string, bool) {
	if !strings.Contains(original, MarkerBegin) {
		return original, false
	}
	start := strings.Index(original, MarkerBegin)
	endIdx := strings.Index(original, MarkerEnd)
	if endIdx < 0 {
		return original, false
	}
	end := endIdx + len(MarkerEnd)
	out := original[:start] + original[end:]
	out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	return strings.TrimSpace(out) + "\n", true
}

func mergeJSONMCPServers(original []byte, serverName string, serverCfg map[string]any) ([]byte, bool, error) {
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
	existing, ok := servers[serverName].(map[string]any)
	if ok {
		if owned, _ := existing["_"+JSONBegin].(bool); owned {
			nb, _ := json.Marshal(serverCfg)
			ob, _ := json.Marshal(existingFiltered(existing))
			_ = nb
			_ = ob
		}
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

func existingFiltered(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		if !strings.HasPrefix(k, "_") {
			out[k] = v
		}
	}
	return out
}

func removeJSONMCPServer(original []byte, serverName string) ([]byte, bool, error) {
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
