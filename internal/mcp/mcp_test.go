package mcp_test

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"testing"
)

func runMCP(t *testing.T, requests []map[string]any) []map[string]any {
	t.Helper()
	bin := buildMCP(t)
	cmd := exec.Command(bin, "mcp")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(stdin)
	for _, r := range requests {
		b, _ := json.Marshal(r)
		w.Write(b)
		w.WriteByte('\n')
	}
	w.Flush()
	stdin.Close()
	var out []map[string]any
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err == nil {
			out = append(out, m)
		}
		if len(out) == len(requests) {
			break
		}
	}
	cmd.Wait()
	return out
}

func buildMCP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir() + "/autodoc-mcp"
	cmd := exec.Command("go", "build", "-o", dir, "github.com/pedro-dalben/autodoc/cmd/autodoc")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return dir
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	resps := runMCP(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}},
	})
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if resps[0]["error"] != nil {
		t.Fatalf("init error: %v", resps[0])
	}
	res, _ := resps[1]["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	if len(tools) < 5 {
		t.Fatalf("expected >=5 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok {
			names[m["name"].(string)] = true
		}
	}
	for _, want := range []string{"project_info", "storyboard_validate", "tts_synthesize", "agent_state", "explain", "doctor"} {
		if !names[want] {
			t.Fatalf("missing tool %s in %v", want, names)
		}
	}
	for _, tl := range tools {
		m := tl.(map[string]any)
		n := m["name"].(string)
		if n == "exec" || n == "shell" || n == "browser" || n == "ffmpeg" {
			t.Fatalf("forbidden generic tool exposed: %s", n)
		}
	}
}

func TestMCPUnknownTool(t *testing.T) {
	resps := runMCP(t, []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "nope", "arguments": map[string]any{}}},
	})
	if len(resps) != 1 || resps[0]["error"] == nil {
		t.Fatalf("expected error: %v", resps)
	}
}
