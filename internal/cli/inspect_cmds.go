package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/agent"
	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/doctor"
	"github.com/pedro-dalben/autodoc/internal/evidence"
	"github.com/pedro-dalben/autodoc/internal/mcp"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/ui"
	"github.com/pedro-dalben/autodoc/internal/usage"
	"github.com/spf13/cobra"
)

// evidenceDir / usageDir resolve once per process for the current project.
var (
	evidenceDir, usageDir string
)

func init() { evidenceDir, usageDir = localStateDirs() }

// localStateDirs resolves the evidence/usage cache directories for the
// current project (falling back to cwd when no autodoc.toml exists).
func localStateDirs() (string, string) {
	cwd, _ := os.Getwd()
	root := cwd
	if lc, err := config.FindConfig(cwd); err == nil && lc.Source != "default" {
		root = lc.Root
	}
	return filepath.Join(root, ".autodoc", "cache", "evidence"),
		filepath.Join(root, ".autodoc", "cache", "usage")
}

// resolveAuthState maps --auth to a storage-state.json path (profile name or file).
func resolveAuthState(auth string) string {
	if auth == "" {
		return ""
	}
	if info, err := os.Stat(auth); err == nil {
		if !info.IsDir() {
			return auth
		}
		// A project can legitimately contain a directory named like a
		// profile (notably docs/). Directories are never storage-state files.
		return filepath.Join(doctor.ProfileDir(auth), "storage-state.json")
	}
	p := filepath.Join(doctor.ProfileDir(auth), "storage-state.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return auth
}

func newUICmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "ui",
		Short: "Focused UI inspection for tutorial discovery (compact, recoverable)",
		Long: "Domain-scoped browser inspection: returns only the controls relevant to an intent,\n" +
			"plus a retrieval handle (evidence ref) for the raw inventory. Discovery aid only —\n" +
			"recording still runs through the deterministic capture backend.",
	}
	var qURL, qIntent, qAuth, qBase string
	var qLimit int
	var qJSON bool
	query := &cobra.Command{
		Use:   "query",
		Short: "Inspect a page for controls relevant to an intent",
		Example: `autodoc ui query --base-url http://localhost:3000 --url /admin/chat --intent "enviar mensagem"
autodoc ui query --url http://localhost:8099/materiais --limit 8`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if qURL == "" {
				return fmt.Errorf("--url is required (path or absolute)")
			}
			opts := ui.InspectOptions{
				BaseURL:      qBase,
				URL:          qURL,
				Intent:       qIntent,
				StorageState: resolveAuthState(qAuth),
				Limit:        qLimit,
			}
			inv, raw, err := ui.Inspect(opts)
			if err != nil {
				return err
			}
			store := evidence.NewStore(evidenceDir)
			summary := fmt.Sprintf("%d controls, %s", len(inv.Controls), inv.RegionSummary())
			ref, err := store.Put("ui_inventory", evidence.Redact(raw), summary,
				map[string]string{"intent": qIntent, "title": inv.Title}, inv.URL)
			if err != nil {
				return err
			}
			out := evidence.Render(ref, raw, evidence.CompactInventory(inv))
			if qJSON {
				fmt.Fprintln(cmd.OutOrStdout(), raw)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), out.Text)
			}
			return usage.Log(usageDir, usage.Entry{
				Phase:  usage.PhaseBrowser,
				Bytes:  out.SentBytes,
				Source: "autodoc",
				Label:  "ui_query " + inv.URL,
			})
		},
	}
	query.Flags().StringVar(&qURL, "url", "", "page path (with --base-url) or absolute URL")
	query.Flags().StringVar(&qIntent, "intent", "", "what you are looking for, e.g. \"enviar mensagem\"")
	query.Flags().StringVar(&qAuth, "auth", "", "auth profile name or storage-state.json path")
	query.Flags().StringVar(&qBase, "base-url", "", "base URL for relative paths (e.g. http://localhost:3000)")
	query.Flags().IntVar(&qLimit, "limit", 12, "max controls returned")
	query.Flags().BoolVar(&qJSON, "json", false, "print full inventory JSON instead of compact view")

	var dURL, dAuth, dBase, dClick, dFill string
	var dJSON bool
	diff := &cobra.Command{
		Use:   "diff",
		Short: "Inspect BEFORE, perform ONE action, inspect AFTER, print semantic delta",
		Example: `autodoc ui diff --base-url http://localhost:3000 --url /admin/chat --fill "Digite uma mensagem=Texto"
autodoc ui diff --base-url http://localhost:3000 --url /admin/chat --click "Enviar mensagem"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dURL == "" {
				return fmt.Errorf("--url is required")
			}
			if dClick == "" && dFill == "" {
				return fmt.Errorf("one action is required: --click NAME or --fill NAME=VALUE")
			}
			action := ui.Action{Type: "click", Label: dClick}
			if dFill != "" {
				action = ui.Action{Type: "fill", Label: dFill}
			}
			opts := ui.InspectOptions{
				BaseURL:      dBase,
				URL:          dURL,
				StorageState: resolveAuthState(dAuth),
			}
			res, before, after, err := ui.Diff(opts, action)
			if err != nil {
				return err
			}
			store := evidence.NewStore(evidenceDir)
			beforeRef, err := store.Put("ui_inventory", evidence.Redact(before), "state before "+action.Type, nil, res.URLFrom)
			if err != nil {
				return err
			}
			afterRef, err := store.Put("ui_inventory", evidence.Redact(after), "state after "+action.Type, nil, res.URL)
			if err != nil {
				return err
			}
			res.BeforeRef, res.AfterRef = beforeRef.ID, afterRef.ID
			rawPayload := fmt.Sprintf("before:\n%s\nafter:\n%s\nresult:\n%s", before, after, mustJSON(res))
			ref, err := store.Put("ui_diff", evidence.Redact(rawPayload), res.Compact(), nil, res.URL)
			if err != nil {
				return err
			}
			compact := res.Compact() + fmt.Sprintf("evidence: %s (before %s, after %s)\n", ref.ID, beforeRef.ID, afterRef.ID)
			if dJSON {
				b, _ := json.MarshalIndent(res, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				out := evidence.Render(ref, rawPayload, compact)
				fmt.Fprint(cmd.OutOrStdout(), out.Text)
				return usage.Log(usageDir, usage.Entry{
					Phase:  usage.PhaseBrowser,
					Bytes:  out.SentBytes,
					Source: "autodoc",
					Label:  "ui_diff " + res.URL,
				})
			}
			return nil
		},
	}
	diff.Flags().StringVar(&dURL, "url", "", "page path or absolute URL")
	diff.Flags().StringVar(&dClick, "click", "", "click the element with this accessible name/text")
	diff.Flags().StringVar(&dFill, "fill", "", "fill NAME=VALUE on the element named NAME")
	diff.Flags().StringVar(&dAuth, "auth", "", "auth profile name or storage-state.json path")
	diff.Flags().StringVar(&dBase, "base-url", "", "base URL for relative paths")
	diff.Flags().BoolVar(&dJSON, "json", false, "print diff JSON instead of compact view")

	c.AddCommand(query, diff)
	return c
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

func newEvidenceCmd() *cobra.Command {
	c := &cobra.Command{Use: "evidence", Short: "Recoverable evidence store (raw payloads behind compact refs)"}
	get := &cobra.Command{
		Use:   "get <ref>",
		Short: "Print the raw payload behind an evidence ref",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := evidence.NewStore(evidenceDir)
			payload, err := store.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), payload)
			return nil
		},
	}
	stats := &cobra.Command{
		Use:   "stats",
		Short: "Evidence store size summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			st := evidence.NewStore(evidenceDir).Stats()
			fmt.Fprintf(cmd.OutOrStdout(), "refs: %d  raw bytes: %d\n", st.Refs, st.RawBytes)
			return nil
		},
	}
	var pruneOlder string
	prune := &cobra.Command{
		Use:   "prune",
		Short: "Drop evidence older than --older-than (default 168h)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := time.ParseDuration(pruneOlder)
			if err != nil {
				return fmt.Errorf("invalid --older-than %q: %w", pruneOlder, err)
			}
			n := evidence.NewStore(evidenceDir).Prune(d)
			fmt.Fprintf(cmd.OutOrStdout(), "pruned %d refs\n", n)
			return nil
		},
	}
	prune.Flags().StringVar(&pruneOlder, "older-than", "168h", "drop refs older than this (Go duration)")
	var invKind, invURL string
	invalidate := &cobra.Command{
		Use:   "invalidate",
		Short: "Invalidate evidence by --kind and/or --url (empty = all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			n := evidence.NewStore(evidenceDir).Invalidate(invKind, invURL)
			fmt.Fprintf(cmd.OutOrStdout(), "invalidated %d refs\n", n)
			return nil
		},
	}
	invalidate.Flags().StringVar(&invKind, "kind", "", "evidence kind (ui_inventory|ui_diff|page_state)")
	invalidate.Flags().StringVar(&invURL, "url", "", "page URL scope")
	c.AddCommand(get, stats, prune, invalidate)
	return c
}

func newContextCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "context",
		Short: "Agent context budget accounting (bytes per phase)",
		Long: "Records how many agent-visible bytes each phase consumed. AutoDoc logs its own\n" +
			"outputs; log external observations (browser snapshots, skill tokens) with `context log`.\n" +
			"Token numbers are estimates (bytes/4) unless the source reports real usage.",
	}
	var logPhase, logLabel, logSource string
	var logBytes int
	log := &cobra.Command{
		Use:   "log",
		Short: "Log one context observation (--phase --bytes [--label] [--source])",
		RunE: func(cmd *cobra.Command, args []string) error {
			if logPhase == "" || logBytes <= 0 {
				return fmt.Errorf("--phase and --bytes are required")
			}
			_, usageDir := localStateDirs()
			if err := usage.Log(usageDir, usage.Entry{
				Phase: logPhase, Bytes: logBytes, Label: logLabel, Source: logSource,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged %d bytes to %s\n", logBytes, logPhase)
			return nil
		},
	}
	log.Flags().StringVar(&logPhase, "phase", "", "skill|tool_catalog|repo|browser|cli|storyboard|evidence")
	log.Flags().IntVar(&logBytes, "bytes", 0, "agent-visible bytes observed")
	log.Flags().StringVar(&logLabel, "label", "", "optional label")
	log.Flags().StringVar(&logSource, "source", "agent", "who measured it (agent|autodoc)")
	stats := &cobra.Command{
		Use:   "stats",
		Short: "Show context budget by phase",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, usageDir := localStateDirs()
			totals, err := usage.Report(usageDir)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), usage.Format(totals))
			return nil
		},
	}
	reset := &cobra.Command{
		Use:   "reset",
		Short: "Erase the context usage log",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, usageDir := localStateDirs()
			return usage.Reset(usageDir)
		},
	}
	c.AddCommand(log, stats, reset)
	return c
}

func newAgentCmd() *cobra.Command {
	var goal, storyboardPath string
	c := &cobra.Command{Use: "agent", Short: "Compact task-specific protocol for coding agents"}
	boot := &cobra.Command{
		Use: "bootstrap", Short: "Show existing knowledge and only the next workflow steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			root, sbPath := cwd, "storyboard.yml"
			if storyboardPath != "" {
				var err error
				root, _, sbPath, err = resolveFromCwd(storyboardPath)
				if err != nil {
					return err
				}
				if rel, err := filepath.Rel(root, sbPath); err == nil {
					sbPath = rel
				}
			}
			fmt.Fprint(cmd.OutOrStdout(), agent.Bootstrap(root, goal, sbPath))
			return nil
		},
	}
	boot.Flags().StringVar(&goal, "goal", "", "user tutorial request")
	boot.Flags().StringVar(&storyboardPath, "storyboard", "", "existing storyboard path")
	var flow, capsuleStoryboard string
	var sources []string
	create := &cobra.Command{Use: "capsule-create", Short: "Save a compact warm-run capsule", RunE: func(cmd *cobra.Command, args []string) error {
		if flow == "" || capsuleStoryboard == "" {
			return fmt.Errorf("--flow and --storyboard are required")
		}
		root, _, sbPath, err := resolveFromCwd(capsuleStoryboard)
		if err != nil {
			return err
		}
		sb, err := storyboard.LoadFile(sbPath)
		if err != nil {
			return err
		}
		for i, source := range sources {
			if !filepath.IsAbs(source) {
				sources[i] = filepath.Join(root, source)
			}
		}
		capsule, err := agent.NewCapsule(flow, sb, sources)
		if err != nil {
			return err
		}
		path, err := agent.SaveCapsule(filepath.Join(root, ".autodoc", "cache", "capsules"), capsule)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "capsule: saved %s\n", path)
		return nil
	}}
	create.Flags().StringVar(&flow, "flow", "", "stable flow name")
	create.Flags().StringVar(&capsuleStoryboard, "storyboard", "", "storyboard path")
	create.Flags().StringSliceVar(&sources, "source", nil, "relevant source file (repeatable)")
	var capsulePath string
	status := &cobra.Command{Use: "capsule-status", Short: "Report whether a capsule's relevant sources changed", RunE: func(cmd *cobra.Command, args []string) error {
		if capsulePath == "" {
			return fmt.Errorf("--capsule is required")
		}
		capsule, err := agent.LoadCapsule(capsulePath)
		if err != nil {
			return err
		}
		changed := capsule.ChangedSources()
		if len(changed) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "capsule: REUSE")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "capsule: REVALIDATE\nchanged: %s\n", strings.Join(changed, ", "))
		}
		return nil
	}}
	status.Flags().StringVar(&capsulePath, "capsule", "", "capsule JSON path")
	guide := &cobra.Command{Use: "guide <topic>", Short: "Read one small on-demand workflow module", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		text, ok := agent.Guide(args[0])
		if !ok {
			return fmt.Errorf("unknown guide %q (discovery|storyboard|auth|recording|cinematic|tts|publishing|debug|security)", args[0])
		}
		fmt.Fprint(cmd.OutOrStdout(), text)
		return nil
	}}
	var stateJSON bool
	var stateSb string
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Report current tutorial lifecycle state and next recommended commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			root := cwd
			sbPath := stateSb
			if sbPath != "" {
				var err error
				root, _, sbPath, err = resolveFromCwd(sbPath)
				if err != nil {
					return err
				}
				if rel, err := filepath.Rel(root, sbPath); err == nil {
					sbPath = rel
				}
			}
			st := agent.DetectState(root, sbPath)
			if stateJSON {
				b, err := json.MarshalIndent(st, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "state: %s\n", st.State)
			if st.StoryboardPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "storyboard: %s\n", st.StoryboardPath)
			}
			if st.LatestRunDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "latest_run: %s\n", st.LatestRunDir)
			}
			if st.VideoPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "video: %s\n", st.VideoPath)
			}
			if st.QAPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "qa_report: %s\n", st.QAPath)
			}
			if st.EvidenceCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "evidence: %d items\n", st.EvidenceCount)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "next_action: %s\n", st.NextAction)
			fmt.Fprintln(cmd.OutOrStdout(), "next_commands:")
			for _, c := range st.NextCommands {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", c)
			}
			return nil
		},
	}
	stateCmd.Flags().BoolVar(&stateJSON, "json", false, "output JSON format")
	stateCmd.Flags().StringVar(&stateSb, "storyboard", "", "storyboard path")

	c.AddCommand(boot, stateCmd, create, status, guide)
	return c
}

// registerInspectMCPTools adds the focused-inspection tools to the MCP server.
func registerInspectMCPTools(s *mcp.Server) {
	s.Register("ui_query", "Focused UI inspection: returns only controls relevant to intent + evidence ref (compact)",
		map[string]any{"type": "object", "properties": map[string]any{
			"url":    map[string]any{"type": "string", "description": "path or absolute URL"},
			"intent": map[string]any{"type": "string"},
			"auth":   map[string]any{"type": "string", "description": "profile name or storage-state path"},
			"limit":  map[string]any{"type": "integer"},
		}, "required": []string{"url"}},
		func(args map[string]any) (any, error) {
			limit := 12
			if v, ok := args["limit"].(float64); ok && v > 0 {
				limit = int(v)
			}
			opts := ui.InspectOptions{
				URL:          mcp.StrArg(args, "url", ""),
				Intent:       mcp.StrArg(args, "intent", ""),
				StorageState: resolveAuthState(mcp.StrArg(args, "auth", "")),
				Limit:        limit,
			}
			inv, raw, err := ui.Inspect(opts)
			if err != nil {
				return nil, err
			}
			evidenceDir, usageDir := localStateDirs()
			store := evidence.NewStore(evidenceDir)
			ref, err := store.Put("ui_inventory", evidence.Redact(raw),
				fmt.Sprintf("%d controls, %s", len(inv.Controls), inv.RegionSummary()),
				map[string]string{"intent": opts.Intent, "title": inv.Title}, inv.URL)
			if err != nil {
				return nil, err
			}
			out := evidence.Render(ref, raw, evidence.CompactInventory(inv))
			_ = usage.Log(usageDir, usage.Entry{Phase: usage.PhaseBrowser, Bytes: out.SentBytes, Label: "mcp ui_query " + inv.URL})
			return out.Text, nil
		})
	s.Register("ui_diff", "UI state delta: inspect BEFORE, one action, inspect AFTER; prints added/removed/changed controls",
		map[string]any{"type": "object", "properties": map[string]any{
			"url":   map[string]any{"type": "string"},
			"click": map[string]any{"type": "string", "description": "accessible name/text of element to click"},
			"fill":  map[string]any{"type": "string", "description": "NAME=VALUE"},
			"auth":  map[string]any{"type": "string"},
		}, "required": []string{"url"}},
		func(args map[string]any) (any, error) {
			click, fill := mcp.StrArg(args, "click", ""), mcp.StrArg(args, "fill", "")
			if click == "" && fill == "" {
				return nil, fmt.Errorf("click or fill is required")
			}
			action := ui.Action{Type: "click", Label: click}
			if fill != "" {
				action = ui.Action{Type: "fill", Label: fill}
			}
			opts := ui.InspectOptions{
				URL:          mcp.StrArg(args, "url", ""),
				StorageState: resolveAuthState(mcp.StrArg(args, "auth", "")),
			}
			res, before, after, err := ui.Diff(opts, action)
			if err != nil {
				return nil, err
			}
			evidenceDir, usageDir := localStateDirs()
			store := evidence.NewStore(evidenceDir)
			beforeRef, err := store.Put("ui_inventory", evidence.Redact(before), "before "+action.Type, nil, res.URLFrom)
			if err != nil {
				return nil, err
			}
			afterRef, err := store.Put("ui_inventory", evidence.Redact(after), "after "+action.Type, nil, res.URL)
			if err != nil {
				return nil, err
			}
			res.BeforeRef, res.AfterRef = beforeRef.ID, afterRef.ID
			rawPayload := fmt.Sprintf("before:\n%s\nafter:\n%s\nresult:\n%s", before, after, mustJSON(res))
			ref, err := store.Put("ui_diff", evidence.Redact(rawPayload), res.Compact(), nil, res.URL)
			if err != nil {
				return nil, err
			}
			compact := res.Compact() + fmt.Sprintf("evidence: %s (before %s, after %s)\n", ref.ID, beforeRef.ID, afterRef.ID)
			out := evidence.Render(ref, rawPayload, compact)
			_ = usage.Log(usageDir, usage.Entry{Phase: usage.PhaseBrowser, Bytes: out.SentBytes, Label: "mcp ui_diff " + res.URL})
			return out.Text, nil
		})
	s.Register("evidence_get", "Retrieve the raw payload behind an evidence ref (recoverable compression)",
		map[string]any{"type": "object", "properties": map[string]any{
			"ref": map[string]any{"type": "string"},
		}, "required": []string{"ref"}},
		func(args map[string]any) (any, error) {
			store := evidence.NewStore(evidenceDir)
			payload, err := store.Get(mcp.StrArg(args, "ref", ""))
			if err != nil {
				return nil, err
			}
			return payload, nil
		})
}
