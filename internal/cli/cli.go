package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/cinematic"
	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/doctor"
	"github.com/pedro-dalben/autodoc/internal/harness"
	"github.com/pedro-dalben/autodoc/internal/install"
	"github.com/pedro-dalben/autodoc/internal/mcp"
	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/pipeline"
	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/tts"
	"github.com/pedro-dalben/autodoc/internal/version"
	"github.com/spf13/cobra"
)

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "autodoc",
		Short: "AutoDoc — deterministic narrated UI tutorial videos",
		Long:  "AutoDoc compiles storyboard.yml into deterministic narrated tutorial videos (TTS + Playwright replay + FFmpeg render).",
	}
	root.AddCommand(
		newInitCmd(),
		newDoctorCmd(),
		newStoryboardCmd(),
		newCompileCmd(),
		newTTSCmd(),
		newRecordCmd(),
		newRenderCmd(),
		newExportCmd(),
		newValidateCmd(),
		newAuthCmd(),
		newBrowserCmd(),
		newUICmd(),
		newEvidenceCmd(),
		newContextCmd(),
		newAgentCmd(),
		newMCPCmd(),
		newUninstallCmd(),
		newVersionCmd(),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "autodoc %s (commit %s, %s)\n", version.Version, version.Commit, version.Date)
			fmt.Fprintf(cmd.OutOrStdout(), "playwright-go pin %s (min %s)\n", version.PlaywrightGoPin, version.PlaywrightGoMinimum)
		},
	}
}

func loadProject(cmd *cobra.Command, storyboardFlag string) (root string, cfg *config.Config, sbPath string, err error) {
	cwd, _ := os.Getwd()
	lc, err := config.FindConfig(cwd)
	if err != nil {
		return "", nil, "", err
	}
	if lc.Source == "default" {
		return "", nil, "", fmt.Errorf("autodoc.toml not found (run autodoc init first)")
	}
	root, cfg = lc.Root, lc.Config
	if storyboardFlag != "" {
		sbPath = storyboardFlag
		if !filepath.IsAbs(sbPath) {
			sbPath = filepath.Join(cwd, sbPath)
		}
	} else {
		sbPath = filepath.Join(root, cfg.Paths.Storyboard)
	}
	return root, cfg, sbPath, nil
}

func newInitCmd() *cobra.Command {
	var opts struct {
		nonInteractive bool
		global         bool
		ttsProvider    string
		ttsBaseURL     string
		ttsModel       string
		ttsVoice       string
		harness        []string
		skipBrowser    bool
	}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive setup: harnesses, TTS, browser, skill, MCP",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "AutoDoc Setup")
			fmt.Fprintln(out)
			cfg := config.Default()
			if opts.ttsProvider != "" {
				cfg.TTS.Provider = opts.ttsProvider
			}
			if opts.ttsBaseURL != "" {
				cfg.TTS.BaseURL = opts.ttsBaseURL
			}
			if opts.ttsModel != "" {
				cfg.TTS.Model = opts.ttsModel
			}
			if opts.ttsVoice != "" {
				cfg.TTS.Voice = opts.ttsVoice
			}
			if !opts.nonInteractive && isTTY() {
				askProvider(cfg, out)
			}
			if errs := cfg.Validate(); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(out, "config error: %v\n", e)
				}
				return fmt.Errorf("invalid configuration")
			}
			if opts.global {
				gp, err := config.GlobalPath()
				if err != nil {
					return err
				}
				cfg.Project.Name = "global"
				existed := fileExists(gp)
				if err := cfg.Save(gp); err != nil {
					return err
				}
				if existed {
					fmt.Fprintf(out, "updated %s\n", gp)
				} else {
					fmt.Fprintf(out, "wrote %s\n", gp)
				}
				fmt.Fprintln(out, "project configs keep priority: ./autodoc.toml overrides this global file")
			} else {
				cfgPath := filepath.Join(cwd, "autodoc.toml")
				existed := fileExists(cfgPath)
				if err := cfg.Save(cfgPath); err != nil {
					return err
				}
				if existed {
					fmt.Fprintln(out, "updated autodoc.toml (preserved project-local settings)")
				} else {
					fmt.Fprintln(out, "wrote autodoc.toml")
				}
				if _, err := os.Stat(filepath.Join(cwd, "storyboard.yml")); os.IsNotExist(err) {
					if err := storyboard.WriteExample(filepath.Join(cwd, "storyboard.yml")); err != nil {
						return err
					}
					fmt.Fprintln(out, "wrote storyboard.yml (example — edit me)")
				}
			}
			home := harness.HomeDir()
			exe, _ := os.Executable()
			mcpCmd := exe
			if mcpCmd == "" {
				mcpCmd = "autodoc"
			}
			skillSource := canonicalSkillPath()
			manifest, _ := install.LoadManifest()
			want := opts.harness
			if len(want) == 0 {
				for _, h := range harness.All() {
					if h.Name == "cursor" {
						continue
					}
					want = append(want, h.Name)
				}
			}
			fmt.Fprintln(out, "\nCoding agents found:")
			for _, h := range harness.All() {
				if h.Name == "cursor" {
					continue
				}
				pr := h.Probe(home)
				mark := "[ ]"
				for _, w := range want {
					if w == h.Name {
						mark = "[x]"
					}
				}
				state := "not found"
				switch {
				case pr.Configured:
					state = "configured"
				case pr.Partial:
					state = "partial"
				case pr.Found:
					state = "found"
				}
				fmt.Fprintf(out, "%s %s (%s)\n", mark, displayHarness(h.Name), state)
			}
			for _, name := range want {
				h := harness.ByName(name)
				if h == nil {
					fmt.Fprintf(out, "unknown harness %q — skipped\n", name)
					continue
				}
				before := snapshotFiles(home, h)
				changed, err := h.Install(home, skillSource, mcpCmd)
				if err != nil {
					fmt.Fprintf(out, "harness %s: error: %v\n", name, err)
					continue
				}
				after := snapshotFiles(home, h)
				_ = after
				if len(changed) == 0 {
					fmt.Fprintf(out, "harness %s: no changes (already installed)\n", name)
				} else {
					fmt.Fprintf(out, "harness %s: installed %d file(s)\n", name, len(changed))
				}
				_ = before
				for _, p := range changed {
					manifest.Upsert(install.OwnedItem{Kind: "harness-file", Harness: name, Path: p})
				}
			}
			if err := manifest.Save(version.Version); err != nil {
				return err
			}
			fmt.Fprintln(out, "\nTesting...")
			checkTTS(cfg, out)
			checkFFmpeg(out)
			checkBrowser(out)
			fmt.Fprintln(out, "skill: canonical "+skillSource)
			fmt.Fprintln(out, "\nAutoDoc ready. Run `autodoc doctor` for full diagnostics.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.nonInteractive, "non-interactive", false, "skip prompts")
	cmd.Flags().BoolVar(&opts.global, "global", false, "write machine-wide ~/.config/autodoc/autodoc.toml instead of ./autodoc.toml")
	cmd.Flags().StringVar(&opts.ttsProvider, "tts-provider", "", "tts provider (openai-compatible|disabled)")
	cmd.Flags().StringVar(&opts.ttsBaseURL, "tts-base-url", "", "tts base url")
	cmd.Flags().StringVar(&opts.ttsModel, "tts-model", "", "tts model")
	cmd.Flags().StringVar(&opts.ttsVoice, "tts-voice", "", "tts voice")
	cmd.Flags().StringArrayVar(&opts.harness, "harness", nil, "harness to configure (repeatable)")
	cmd.Flags().BoolVar(&opts.skipBrowser, "skip-browser", false, "skip browser check")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	var opts struct {
		restoreBackup bool
		harness       []string
	}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove only AutoDoc-owned installation artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			home := harness.HomeDir()
			manifest, _ := install.LoadManifest()
			names := opts.harness
			if len(names) == 0 {
				seen := map[string]bool{}
				for _, it := range manifest.Items {
					if it.Harness != "" {
						seen[it.Harness] = true
					}
				}
				for n := range seen {
					names = append(names, n)
				}
				if len(names) == 0 {
					for _, h := range harness.All() {
						names = append(names, h.Name)
					}
				}
			}
			for _, name := range names {
				h := harness.ByName(name)
				if h == nil {
					continue
				}
				removed, err := h.Uninstall(home)
				if err != nil {
					fmt.Fprintf(out, "harness %s: %v\n", name, err)
					continue
				}
				for _, p := range removed {
					manifest.RemoveByPath(p)
				}
				if len(removed) == 0 {
					fmt.Fprintf(out, "harness %s: nothing owned to remove\n", name)
				} else {
					fmt.Fprintf(out, "harness %s: removed %d AutoDoc-owned file(s), user edits preserved\n", name, len(removed))
				}
			}
			if opts.restoreBackup {
				for src, bak := range manifest.Backups {
					if data, err := os.ReadFile(bak); err == nil {
						_ = os.WriteFile(src, data, 0o644)
						fmt.Fprintf(out, "restored %s from backup\n", src)
					}
				}
			}
			return manifest.Save(version.Version)
		},
	}
	cmd.Flags().BoolVar(&opts.restoreBackup, "restore-backup", false, "explicitly restore pre-install backups")
	cmd.Flags().StringArrayVar(&opts.harness, "harness", nil, "only uninstall these harnesses")
	return cmd
}

func newDoctorCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose autodoc, media, browser, TTS, harnesses, skill",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep := doctor.Run()
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			rep.Print(cmd.OutOrStdout())
			if rep.Failed() {
				return fmt.Errorf("doctor found failing checks")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	return cmd
}

func newStoryboardCmd() *cobra.Command {
	c := &cobra.Command{Use: "storyboard", Short: "Storyboard operations"}
	var sbPath string
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate storyboard.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				sb, err2 := storyboard.LoadFile(nonEmpty(sbPath, "storyboard.yml"))
				if err2 != nil {
					return err2
				}
				_ = sb
				fmt.Fprintln(cmd.OutOrStdout(), "storyboard valid")
				return nil
			}
			sb, err := storyboard.LoadFile(resolved)
			if err != nil {
				return err
			}
			if err := pipeline.ScanSecrets(sb); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "storyboard valid: %d scenes, hash %s\n", len(sb.Scenes), sb.SourceHash())
			return nil
		},
	}
	validate.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	c.AddCommand(validate)
	return c
}

func newCompileCmd() *cobra.Command {
	var sbPath string
	cmd := &cobra.Command{
		Use:   "compile",
		Short: "Compile storyboard.yml to deterministic _work/recipe.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compiled %d scenes, %d speech segments -> %s\n",
				len(run.Recipe.Scenes), len(run.Recipe.SpeechSegments), filepath.Join(run.WorkDir, run.RunID, "recipe.json"))
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	return cmd
}

func newTTSCmd() *cobra.Command {
	var sbPath, backend string
	var headless bool
	_ = headless
	cmd := &cobra.Command{
		Use:   "tts",
		Short: "Synthesize TTS segments (cached per speech segment)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			_ = backend
			prov, err := ttsProviderFromConfig(cfg)
			if err != nil {
				return err
			}
			rep, err := run.SynthesizeTTS(context.Background(), prov, nil)
			if err != nil {
				return err
			}
			for _, res := range rep.Results {
				status := "miss"
				if res.Cached {
					status = "hit"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s (%.2fs)\n", status, res.ID, res.Duration)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tts: %d segments (%d hits, %d misses)\n", rep.Segments, rep.Hits, rep.Misses)
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	cmd.Flags().StringVar(&backend, "backend", "", "reserved")
	return cmd
}

func newRecordCmd() *cobra.Command {
	var sbPath, retake, backend string
	var headless bool
	var noTTS bool
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Deterministic browser replay + scene capture (recordVideo backend)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			if backend == "" {
				backend = "playwright"
			}
			if !noTTS {
				prov, err := ttsProviderFromConfig(cfg)
				if err != nil {
					return err
				}
				if _, err := run.SynthesizeTTS(context.Background(), prov, nil); err != nil {
					return err
				}
			} else if err := run.LoadTimeline(); err != nil {
				tl, err2 := timeline.Build(run.Recipe, func(id string) (float64, bool, string) {
					for _, s := range run.Recipe.SpeechSegments {
						if s.ID == id {
							return tts.EstimateDuration(s.Text, cfg.TTS.Speed), false, ""
						}
					}
					return 0, false, ""
				}, 600)
				if err2 != nil {
					return err2
				}
				run.Timeline = tl
			}
			scenes := run.Recipe.SceneIDs()
			if retake != "" {
				scenes = []string{retake}
			}
			for _, id := range scenes {
				fmt.Fprintf(cmd.OutOrStdout(), "recording scene %s...\n", id)
				art, err := run.RecordScene(context.Background(), id, backend, headless, func(k, l string) {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", k, l)
				})
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "scene %s: video=%s events=%d\n", id, art.VideoPath, len(art.Events))
			}
			if run.Final != nil && run.Final.Sync != nil {
				printSyncReport(cmd.OutOrStdout(), run.Final)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	cmd.Flags().StringVar(&retake, "retake", "", "re-record a single scene id")
	cmd.Flags().StringVar(&backend, "backend", "playwright", "capture backend")
	cmd.Flags().BoolVar(&headless, "headless", true, "headless browser")
	cmd.Flags().BoolVar(&noTTS, "no-tts", false, "skip TTS synthesis")
	return cmd
}

func newRenderCmd() *cobra.Command {
	var sbPath, outMP4 string
	var width, height, fps int
	var noZoom, debugCues, debugTimeline bool
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render final MP4 from the reconciled timeline (FFmpeg H.264+AAC)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			if err := run.LoadTimeline(); err != nil {
				prov, err2 := ttsProviderFromConfig(cfg)
				if err2 != nil {
					return err2
				}
				if _, err2 := run.SynthesizeTTS(context.Background(), prov, nil); err2 != nil {
					return err2
				}
			}
			if width == 0 {
				width = cfg.Browser.ViewportW
			}
			if height == 0 {
				height = cfg.Browser.ViewportH
			}
			if width == 0 {
				width = 1280
			}
			if height == 0 {
				height = 720
			}
			if fps == 0 {
				fps = 30
			}
			if outMP4 == "" {
				outMP4 = filepath.Join(run.WorkDir, run.RunID, "tutorial.mp4")
			}
			if err := os.MkdirAll(filepath.Dir(outMP4), 0o755); err != nil {
				return err
			}
			sceneVideos := map[string]string{}
			for _, sc := range run.Recipe.Scenes {
				if p, srcDir := run.FindSceneVideo(sc.ID); p != "" {
					sceneVideos[sc.ID] = p
					if srcDir != "" && srcDir != filepath.Join(run.WorkDir, run.RunID) {
						fmt.Fprintf(cmd.OutOrStdout(), "scene %s: using raw from %s\n", sc.ID, srcDir)
					}
				}
			}
			if err := run.LoadFinal(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "no final timeline (record with this binary first); legacy planned render")
				if err := media.BuildFinalMP4(run.Timeline, media.RenderOptions{
					Width: width, Height: height, FPS: fps,
					SceneVideo: sceneVideos, WorkDir: run.WorkDir, OutputMP4: outMP4,
				}); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "rendered %s (%.2fs)\n", outMP4, run.Timeline.TotalS)
				return nil
			}
			if debugTimeline {
				fmt.Fprintf(cmd.OutOrStdout(), "final timeline: %d segments, %.2fs total (%d speech)\n",
					len(run.Final.Segments), run.Final.TotalS, len(run.Final.Speeches))
				for _, sg := range run.Final.Segments {
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %-6s %-34s video %.2f-%.2f speed %.2fx zoom %.2f%s\n",
						sg.SceneID, sg.Kind, sg.Label, sg.VideoStartS, sg.VideoEndS, sg.Speed, sg.Zoom, estimatedMark(sg.Estimated))
				}
			}
			debugPath := ""
			if debugCues {
				debugPath = filepath.Join(run.WorkDir, run.RunID, "cues-debug.json")
			}
			if err := media.RenderCinematic(run.Final, media.CinematicOptions{
				Width: width, Height: height, FPS: fps,
				SceneVideo: sceneVideos, WorkDir: run.WorkDir, OutputMP4: outMP4,
				NoZoom: noZoom, DebugCuesPath: debugPath,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rendered %s (%.2fs)\n", outMP4, run.Final.TotalS)
			if debugPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "cue debug: %s\n", debugPath)
			}
			printSyncReport(cmd.OutOrStdout(), run.Final)
			if bundle, err := run.BuildCinematic(); err == nil && bundle.Report != nil {
				status := "PASS"
				if !bundle.Report.Pass {
					status = "FAIL"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "AUTODOC_CINEMATIC_QA: %s (score %d/100)\n", status, bundle.Report.Score)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	cmd.Flags().StringVar(&outMP4, "out", "", "output mp4 path")
	cmd.Flags().IntVar(&width, "width", 0, "output width")
	cmd.Flags().IntVar(&height, "height", 0, "output height")
	cmd.Flags().BoolVar(&noZoom, "no-zoom", false, "disable render-time camera focus")
	cmd.Flags().BoolVar(&debugCues, "debug-cues", false, "write cues-debug.json (bbox, zooms, segments)")
	cmd.Flags().BoolVar(&debugTimeline, "debug-timeline", false, "print reconciled segment table")
	cmd.Flags().IntVar(&fps, "fps", 0, "output fps")
	return cmd
}

func newExportCmd() *cobra.Command {
	var sbPath, outDir, mp4 string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export publishable bundle docs/autodoc/<tutorial>/",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			if err := run.LoadTimeline(); err != nil {
				return fmt.Errorf("no timeline; run tts/record/render first: %w", err)
			}
			name := strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
			if name == "" || name == "storyboard" {
				name = slugify(run.SB.Meta.Title)
			}
			if outDir == "" {
				outDir = filepath.Join(root, cfg.Paths.OutputDir, name)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			srcMP4 := mp4
			if srcMP4 == "" {
				cands := []string{
					filepath.Join(run.WorkDir, run.RunID, "tutorial.mp4"),
					filepath.Join(run.WorkDir, "tutorial.mp4"),
				}
				for _, c := range cands {
					if fileExists(c) {
						srcMP4 = c
						break
					}
				}
				entries, _ := filepath.Glob(filepath.Join(run.WorkDir, "*", "tutorial.mp4"))
				if srcMP4 == "" && len(entries) > 0 {
					srcMP4 = entries[len(entries)-1]
				}
			}
			if srcMP4 == "" || !fileExists(srcMP4) {
				return fmt.Errorf("no rendered mp4 found; run render first")
			}
			dstMP4 := filepath.Join(outDir, "tutorial.mp4")
			if err := copyFile(srcMP4, dstMP4); err != nil {
				return err
			}
			if err := media.WriteSRT(run.Timeline, filepath.Join(outDir, "subtitles.srt")); err != nil {
				return err
			}
			if err := media.WriteVTT(run.Timeline, filepath.Join(outDir, "subtitles.vtt")); err != nil {
				return err
			}
			_ = media.MakeThumbnail(dstMP4, filepath.Join(outDir, "thumbnail.png"), 0.5, 640)
			_ = copyFile(resolved, filepath.Join(outDir, "storyboard.yml"))
			_ = run.Timeline.WriteJSON(filepath.Join(outDir, "timeline.json"))
			if err := run.LoadFinal(); err == nil && run.Final != nil {
				_ = run.Final.WriteJSON(filepath.Join(outDir, "final_timeline.json"))
			}
			writeMetadata(outDir, run)
			writeTutorialMD(outDir, run)
			shotsSrc := filepath.Join(run.WorkDir, run.RunID, "screenshots")
			if _, err := os.Stat(shotsSrc); os.IsNotExist(err) {
				shotsSrc = filepath.Join(run.WorkDir, "screenshots")
			}
			if _, err := os.Stat(shotsSrc); os.IsNotExist(err) {
				if latest := run.LatestRunDirWithHash(); latest != "" {
					shotsSrc = filepath.Join(latest, "screenshots")
				}
			}
			shotsDst := filepath.Join(outDir, "screenshots")
			if st, err := os.Stat(shotsSrc); err == nil && st.IsDir() {
				_ = os.MkdirAll(shotsDst, 0o755)
				entries, _ := os.ReadDir(shotsSrc)
				for _, e := range entries {
					_ = copyFile(filepath.Join(shotsSrc, e.Name()), filepath.Join(shotsDst, e.Name()))
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported %s\n", outDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	cmd.Flags().StringVar(&outDir, "out", "", "output dir")
	cmd.Flags().StringVar(&mp4, "mp4", "", "source mp4")
	return cmd
}

func newValidateCmd() *cobra.Command {
	var sbPath string
	var syncOnly bool
	var cinematicOnly bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate storyboard + timeline + artifacts coherence",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			if err := run.LoadTimeline(); err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "validate: storyboard OK, no timeline yet (run tts)")
				return nil
			}
			if run.Timeline.StoryboardHash != run.Recipe.StoryboardHash {
				return fmt.Errorf("timeline stale: storyboard changed since tts (re-run tts)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "validate OK: %d segments, %.2fs total\n", len(run.Timeline.Segments), run.Timeline.TotalS)
			if err := run.LoadFinal(); err != nil {
				if syncOnly || cinematicOnly {
					return fmt.Errorf("no final timeline; run record first: %w", err)
				}
				return nil
			}
			if cinematicOnly {
				bundle, err := run.BuildCinematic()
				if err != nil {
					return err
				}
				_ = cinematic.WriteBundle(run.WorkDir, bundle)
				fmt.Fprint(cmd.OutOrStdout(), bundle.Report.Print())
				if !bundle.Report.Pass {
					return fmt.Errorf("cinematic check FAILED")
				}
				return nil
			}
			printSyncReport(cmd.OutOrStdout(), run.Final)
			if syncOnly && run.Final.Sync != nil && !run.Final.Sync.Pass {
				return fmt.Errorf("sync check FAILED")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	cmd.Flags().BoolVar(&syncOnly, "sync", false, "only report A/V synchronization (fails when drift exceeds tolerance)")
	cmd.Flags().BoolVar(&cinematicOnly, "cinematic", false, "report Cinematic V2 direction QA (fails when any gate fails)")
	return cmd
}

func newAuthCmd() *cobra.Command {
	var sbPath string
	c := &cobra.Command{Use: "auth", Short: "Off-camera authentication setup"}
	bootstrap := &cobra.Command{
		Use:   "bootstrap",
		Short: "Run setup.sequence off-camera and cache the session (secrets via env, never recorded)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cfg, resolved, err := loadProject(cmd, sbPath)
			if err != nil {
				return err
			}
			run, err := pipeline.NewRun(root, resolved, cfg)
			if err != nil {
				return err
			}
			if err := run.Compile(); err != nil {
				return err
			}
			state, err := run.BootstrapAuth(true)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "auth state cached: %s\n", state)
			fmt.Fprintln(cmd.OutOrStdout(), "record reuses it automatically; rotate by editing setup.sequence")
			return nil
		},
	}
	bootstrap.Flags().StringVar(&sbPath, "storyboard", "", "path to storyboard.yml")
	c.AddCommand(bootstrap)
	return c
}

func printSyncReport(out interface{ Write([]byte) (int, error) }, ft *timeline.FinalTimeline) {
	if ft == nil || ft.Sync == nil {
		return
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "AutoDoc Sync")
	fmt.Fprintln(out, "")
	for _, it := range ft.Sync.Items {
		status := "PASS"
		if !it.Pass {
			status = "FAIL"
		}
		if it.Kind == "coverage" {
			fmt.Fprintf(out, "%-42s %4.0f estimated  %s\n", it.Scope, it.DriftMs, status)
			continue
		}
		fmt.Fprintf(out, "%-42s %+7.0fms  %s\n", it.Scope, it.DriftMs, status)
	}
	fmt.Fprintln(out, "------------------------------")
	status := "PASS"
	if !ft.Sync.Pass {
		status = "FAIL"
	}
	fmt.Fprintf(out, "MAX %+33.0fms\nSTATUS %27s\n", ft.Sync.MaxDriftMs, status)
}

func estimatedMark(est bool) string {
	if est {
		return " (estimated)"
	}
	return ""
}

func newBrowserCmd() *cobra.Command {
	c := &cobra.Command{Use: "browser", Short: "Playwright browser subsystem"}
	check := &cobra.Command{
		Use:   "check",
		Short: "Check driver + browser status",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep := doctor.BrowserSection()
			rep.Print(cmd.OutOrStdout())
			if rep.Failed() {
				return fmt.Errorf("browser check failed")
			}
			return nil
		},
	}
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Playwright browsers (chromium)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "installing playwright chromium (this downloads browser assets)...")
			return doctor.InstallBrowsers(cmd.OutOrStdout())
		},
	}
	login := &cobra.Command{
		Use:   "login",
		Short: "Open dedicated profile browser for manual authentication (never recorded)",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _ := cmd.Flags().GetString("profile")
			url, _ := cmd.Flags().GetString("url")
			fmt.Fprintln(cmd.OutOrStdout(), "Login flow (off-camera, never recorded):")
			fmt.Fprintf(cmd.OutOrStdout(), "  profile dir: %s\n", doctor.ProfileDir(profile))
			fmt.Fprintln(cmd.OutOrStdout(), "  1. a Chromium window opens with the dedicated AutoDoc profile")
			fmt.Fprintln(cmd.OutOrStdout(), "  2. sign in with a FIXTURE/test account (never real credentials in storyboards)")
			fmt.Fprintln(cmd.OutOrStdout(), "  3. close the window; session persists for later `record` runs")
			return doctor.BrowserLogin(profile, url)
		},
	}
	login.Flags().String("profile", "docs", "profile name")
	login.Flags().String("url", "", "url to open for login")
	c.AddCommand(check, installCmd, login)
	return c
}

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run AutoDoc MCP server (stdio)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := mcp.New()
			registerMCPTools(s)
			return s.Serve()
		},
	}
}

func registerMCPTools(s *mcp.Server) {
	sbProp := map[string]any{"storyboard": map[string]any{"type": "string", "description": "path to storyboard.yml"}}
	s.Register("project_info", "AutoDoc project info (version, paths, config)", map[string]any{"type": "object", "properties": map[string]any{}},
		func(args map[string]any) (any, error) {
			cwd, _ := os.Getwd()
			return map[string]any{"version": version.Version, "cwd": cwd, "playwright_pin": version.PlaywrightGoPin}, nil
		})
	s.Register("storyboard_validate", "Validate storyboard.yml (schema + secret scan)", map[string]any{"type": "object", "properties": sbProp, "required": []string{"storyboard"}},
		func(args map[string]any) (any, error) {
			sb, err := storyboard.LoadFile(mcp.StrArg(args, "storyboard", "storyboard.yml"))
			if err != nil {
				return nil, err
			}
			if err := pipeline.ScanSecrets(sb); err != nil {
				return nil, err
			}
			return map[string]any{"valid": true, "scenes": len(sb.Scenes), "hash": sb.SourceHash()}, nil
		})
	s.Register("tts_synthesize", "Synthesize cached TTS segments for a storyboard", map[string]any{"type": "object", "properties": sbProp, "required": []string{"storyboard"}},
		func(args map[string]any) (any, error) {
			cwd, _ := os.Getwd()
			sbPath := mcp.StrArg(args, "storyboard", "storyboard.yml")
			if !filepath.IsAbs(sbPath) {
				sbPath = filepath.Join(cwd, sbPath)
			}
			root, cfg, resolved, err := resolveFromCwd(sbPath)
			_ = root
			if err != nil {
				return nil, err
			}
			run, err := pipeline.NewRun(filepath.Dir(resolved), resolved, cfg)
			if err != nil {
				return nil, err
			}
			if err := run.Compile(); err != nil {
				return nil, err
			}
			prov, err := ttsProviderFromConfig(cfg)
			if err != nil {
				return nil, err
			}
			rep, err := run.SynthesizeTTS(context.Background(), prov, nil)
			if err != nil {
				return nil, err
			}
			return map[string]any{"segments": rep.Segments, "hits": rep.Hits, "misses": rep.Misses}, nil
		})
	s.Register("timeline_build", "Build planned timeline from compiled recipe", map[string]any{"type": "object", "properties": sbProp, "required": []string{"storyboard"}},
		func(args map[string]any) (any, error) {
			r, err := recipe.LoadJSON(mcp.StrArg(args, "storyboard", ""))
			_ = r
			_ = err
			return map[string]any{"note": "use tts_synthesize to build timeline deterministically"}, nil
		})
	s.Register("record", "Deterministic scene record (optional scene_id retake)", map[string]any{"type": "object", "properties": map[string]any{"storyboard": map[string]any{"type": "string"}, "scene_id": map[string]any{"type": "string"}}},
		func(args map[string]any) (any, error) {
			return map[string]any{"note": "run `autodoc record --storyboard <sb> [--retake <scene>]` in a terminal for full capture logs"}, nil
		})
	s.Register("render", "Render final MP4 from timeline", map[string]any{"type": "object", "properties": sbProp},
		func(args map[string]any) (any, error) {
			return map[string]any{"note": "run `autodoc render --storyboard <sb>` in a terminal"}, nil
		})
	s.Register("artifact_export", "Export publishable bundle", map[string]any{"type": "object", "properties": sbProp},
		func(args map[string]any) (any, error) {
			return map[string]any{"note": "run `autodoc export --storyboard <sb>` in a terminal"}, nil
		})
	s.Register("doctor", "Run diagnostics (media/browser/tts/harness/skill)", map[string]any{"type": "object", "properties": map[string]any{}},
		func(args map[string]any) (any, error) {
			rep := doctor.Run()
			return rep, nil
		})
	registerInspectMCPTools(s)
}

func resolveFromCwd(sbPath string) (string, *config.Config, string, error) {
	cwd, _ := os.Getwd()
	if !filepath.IsAbs(sbPath) {
		sbPath = filepath.Join(cwd, sbPath)
	}
	root := filepath.Dir(sbPath)
	cfg := config.Default()
	if lc, err := config.FindConfig(filepath.Dir(sbPath)); err == nil && lc.Source != "default" {
		root = lc.Root
		cfg = lc.Config
	}
	return root, cfg, sbPath, nil
}

func ttsProviderFromConfig(cfg *config.Config) (tts.Provider, error) {
	switch cfg.TTS.Provider {
	case "disabled":
		return tts.Disabled{}, nil
	case "openai-compatible", "":
		key := ""
		if cfg.TTS.APIKeyEnv != "" {
			key = os.Getenv(cfg.TTS.APIKeyEnv)
		}
		return &tts.OpenAICompatible{BaseURL: cfg.TTS.BaseURL, APIKey: key}, nil
	default:
		return nil, fmt.Errorf("unknown tts provider %q", cfg.TTS.Provider)
	}
}

func canonicalSkillPath() string {
	if v := os.Getenv("AUTODOC_SKILL_SOURCE"); v != "" {
		return v
	}
	exe, _ := os.Executable()
	for _, c := range []string{
		filepath.Join(filepath.Dir(exe), "..", "share", "autodoc", "skill", "SKILL.md"),
		"src/skill/autodoc/SKILL.md",
	} {
		if fileExists(c) {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	if d, err := install.DataDir(); err == nil {
		if p := filepath.Join(d, "skill", "SKILL.md"); fileExists(p) {
			return p
		}
	}
	return "src/skill/autodoc/SKILL.md"
}

func writeMetadata(outDir string, run *pipeline.Run) {
	meta := map[string]any{
		"title": run.SB.Meta.Title, "description": run.SB.Meta.Description,
		"language": run.SB.Meta.Language, "storyboard_hash": run.Recipe.StoryboardHash,
		"duration_s": run.Timeline.TotalS, "scenes": run.Recipe.SceneIDs(),
		"autodoc_version": version.Version,
	}
	if run.Final != nil {
		meta["final_duration_s"] = run.Final.TotalS
		if run.Final.Sync != nil {
			meta["sync"] = map[string]any{"pass": run.Final.Sync.Pass, "max_drift_ms": run.Final.Sync.MaxDriftMs}
		}
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "metadata.json"), append(b, '\n'), 0o644)
}

func writeTutorialMD(outDir string, run *pipeline.Run) {
	var sb strings.Builder
	sb.WriteString("# " + run.SB.Meta.Title + "\n\n")
	if run.SB.Meta.Description != "" {
		sb.WriteString(run.SB.Meta.Description + "\n\n")
	}
	sb.WriteString("Video: ./tutorial.mp4\n\n## Transcript\n\n")
	for _, seg := range run.Timeline.Segments {
		fmt.Fprintf(&sb, "- (%s) %s\n", formatTS(seg.StartS), seg.Text)
	}
	sb.WriteString("\n## Scenes\n\n")
	for _, sc := range run.SB.Scenes {
		fmt.Fprintf(&sb, "### %s — %s\n\n", sc.ID, sc.Title)
	}
	_ = os.WriteFile(filepath.Join(outDir, "tutorial.md"), []byte(sb.String()), 0o644)
}

func formatTS(s float64) string {
	m := int(s) / 60
	sec := int(s) % 60
	return fmt.Sprintf("%02d:%02d", m, sec)
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		out = "tutorial"
	}
	return out
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func snapshotFiles(home string, h *harness.Harness) map[string]string {
	out := map[string]string{}
	for _, p := range append(append([]string{}, h.DetectPaths...), h.SkillPaths...) {
		ep := filepath.Join(home, strings.TrimPrefix(p, "~/"))
		if strings.HasPrefix(p, "~/") {
			if data, err := os.ReadFile(ep); err == nil {
				out[ep] = string(data)
			}
		}
	}
	return out
}

func isTTY() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

func askProvider(cfg *config.Config, out interface{ Write([]byte) (int, error) }) {
	fmt.Fprintln(out, "TTS")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "> OpenAI-compatible")
	fmt.Fprintln(out, "  Disabled")
	fmt.Fprintln(out, "")
	fmt.Fprintf(out, "Endpoint [%s]: ", cfg.TTS.BaseURL)
	sc := bufio.NewScanner(os.Stdin)
	endpoint := ""
	if sc.Scan() {
		endpoint = strings.TrimSpace(sc.Text())
	}
	if endpoint != "" {
		cfg.TTS.BaseURL = endpoint
	}
	fmt.Fprintf(out, "Model [%s]: ", cfg.TTS.Model)
	model := ""
	if sc.Scan() {
		model = strings.TrimSpace(sc.Text())
	}
	if model != "" {
		cfg.TTS.Model = model
	}
	fmt.Fprintf(out, "Voice [%s]: ", cfg.TTS.Voice)
	if sc.Scan() {
		if v := strings.TrimSpace(sc.Text()); v != "" {
			cfg.TTS.Voice = v
		}
	}
}

func checkTTS(cfg *config.Config, out interface{ Write([]byte) (int, error) }) {
	if cfg.TTS.Provider == "disabled" {
		fmt.Fprintln(out, "TTS: disabled (silent pacing) ✓")
		return
	}
	fmt.Fprintf(out, "TTS: %s %s model=%s ... ", cfg.TTS.Provider, cfg.TTS.BaseURL, cfg.TTS.Model)
	prov, err := ttsProviderFromConfig(cfg)
	if err != nil {
		fmt.Fprintln(out, "✗ "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15000)
	defer cancel()
	wav, err := prov.Synthesize(ctx, tts.Request{Text: "Teste do AutoDoc.", Voice: cfg.TTS.Voice, Model: cfg.TTS.Model, Language: cfg.TTS.Language, Speed: cfg.TTS.Speed, Format: "wav"})
	if err != nil {
		fmt.Fprintln(out, "✗ "+err.Error())
		return
	}
	dur, _ := tts.WavDurationSeconds(wav)
	fmt.Fprintf(out, "✓ (%.2fs wav)\n", dur)
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func displayHarness(name string) string {
	switch name {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude Code"
	case "antigravity":
		return "Antigravity"
	case "gemini":
		return "Gemini CLI"
	case "opencode":
		return "OpenCode"
	case "cursor":
		return "Cursor (best-effort)"
	default:
		return name
	}
}

func checkFFmpeg(out interface{ Write([]byte) (int, error) }) {
	if v, err := media.CheckFFmpeg(); err != nil {
		fmt.Fprintln(out, "FFmpeg: ✗ "+err.Error())
	} else {
		fmt.Fprintln(out, "FFmpeg: ✓ "+v)
	}
}

func checkBrowser(out interface{ Write([]byte) (int, error) }) {
	rep := doctor.BrowserSection()
	var sb strings.Builder
	rep.Print(&sb)
	for _, line := range strings.Split(strings.TrimSpace(sb.String()), "\n") {
		fmt.Fprintln(out, line)
	}
}
