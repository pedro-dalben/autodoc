package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Err   `json:"error,omitempty"`
}

type Err struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type Handler func(args map[string]any) (any, error)

type Server struct {
	tools    []Tool
	handlers map[string]Handler
}

func New() *Server {
	return &Server{handlers: map[string]Handler{}}
}

func (s *Server) Register(name, desc string, schema any, h Handler) {
	s.tools = append(s.tools, Tool{Name: name, Description: desc, InputSchema: schema})
	s.handlers[name] = h
}

func (s *Server) Tools() []Tool { return s.tools }

func schema(props map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func (s *Server) Serve() error {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResp(out, Response{JSONRPC: "2.0", Error: &Err{Code: -32700, Message: "parse error"}})
			continue
		}
		s.dispatch(out, req)
	}
	return sc.Err()
}

func writeResp(out *bufio.Writer, resp Response) {
	b, _ := json.Marshal(resp)
	out.Write(b)
	out.WriteByte('\n')
	out.Flush()
}

func (s *Server) dispatch(out *bufio.Writer, req Request) {
	ok := func(result any) { writeResp(out, Response{JSONRPC: "2.0", ID: req.ID, Result: result}) }
	fail := func(code int, msg string) {
		writeResp(out, Response{JSONRPC: "2.0", ID: req.ID, Error: &Err{Code: code, Message: msg}})
	}
	switch req.Method {
	case "initialize":
		ok(map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "autodoc", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		})
	case "notifications/initialized":
		return
	case "tools/list":
		ok(map[string]any{"tools": s.tools})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			fail(-32602, "invalid params")
			return
		}
		h, okH := s.handlers[p.Name]
		if !okH {
			fail(-32601, fmt.Sprintf("unknown tool %q", p.Name))
			return
		}
		res, err := h(p.Arguments)
		if err != nil {
			fail(-32000, compactErr(err.Error()))
			return
		}
		ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": toText(res)}}})
	case "ping":
		ok(map[string]any{})
	default:
		fail(-32601, fmt.Sprintf("unknown method %q", req.Method))
	}
}

func toText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	}
}

func StrArg(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

// compactErr bounds error text sent to the agent. Tool failures often wrap
// verbose backend output (ffmpeg logs, Playwright stacks); the exit cause is
// in the first bytes, so truncate instead of forwarding kilobytes.
func compactErr(s string) string {
	const maxErrBytes = 800
	if len(s) <= maxErrBytes {
		return s
	}
	return s[:maxErrBytes] + "\n…(truncated)"
}
