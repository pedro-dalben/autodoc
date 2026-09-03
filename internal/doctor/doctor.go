package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mxschmitt/playwright-go"
	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/harness"
	"github.com/pedro-dalben/autodoc/internal/install"
	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/version"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Section struct {
	Name   string  `json:"name"`
	Checks []Check `json:"checks"`
}

func (s Section) Failed() bool {
	for _, c := range s.Checks {
		if c.Status == "fail" {
			return true
		}
	}
	return false
}

func (s Section) Print(w io.Writer) {
	fmt.Fprintf(w, "## %s\n", s.Name)
	for _, c := range s.Checks {
		mark := "✓"
		if c.Status == "fail" {
			mark = "✗"
		} else if c.Status == "warn" {
			mark = "!"
		}
		if c.Detail != "" {
			fmt.Fprintf(w, "%s %s — %s\n", mark, c.Name, c.Detail)
		} else {
			fmt.Fprintf(w, "%s %s\n", mark, c.Name)
		}
	}
}

type Report struct {
	Sections []Section `json:"sections"`
}

func (r *Report) Failed() bool {
	for _, s := range r.Sections {
		if s.Failed() {
			return true
		}
	}
	return false
}

func (r *Report) Print(w io.Writer) {
	for _, s := range r.Sections {
		s.Print(w)
		fmt.Fprintln(w)
	}
}

func ok(name, detail string) Check   { return Check{Name: name, Status: "ok", Detail: detail} }
func warn(name, detail string) Check { return Check{Name: name, Status: "warn", Detail: detail} }
func fail(name, detail string) Check { return Check{Name: name, Status: "fail", Detail: detail} }

func Run() *Report {
	return &Report{Sections: []Section{
		AutodocSection(),
		MediaSection(),
		BrowserSection(),
		TTSSection(),
		HarnessSection(),
		SkillSection(),
		FilesystemSection(),
	}}
}

func AutodocSection() Section {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	s := Section{Name: "AutoDoc"}
	s.Checks = append(s.Checks, ok("version", fmt.Sprintf("%s (commit %s)", version.Version, version.Commit)))
	s.Checks = append(s.Checks, ok("binary", exe))
	s.Checks = append(s.Checks, ok("platform", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)))
	s.Checks = append(s.Checks, ok("cwd", cwd))
	cwd, _ = filepath.Abs(cwd)
	if lc, err := config.FindConfig(cwd); err != nil {
		s.Checks = append(s.Checks, warn("config", err.Error()))
	} else {
		switch lc.Source {
		case "project":
			s.Checks = append(s.Checks, ok("config", redactHome(lc.Path)+" (project)"))
		case "global":
			s.Checks = append(s.Checks, ok("config", redactHome(lc.Path)+" (global)"))
		default:
			s.Checks = append(s.Checks, warn("config", "autodoc.toml not found (run autodoc init)"))
		}
	}
	return s
}

func MediaSection() Section {
	s := Section{Name: "Media"}
	if v, err := media.CheckFFmpeg(); err != nil {
		s.Checks = append(s.Checks, fail("ffmpeg", err.Error()))
	} else {
		s.Checks = append(s.Checks, ok("ffmpeg", v))
	}
	if out, err := exec.Command(media.FFprobePath(), "-version").Output(); err != nil {
		s.Checks = append(s.Checks, fail("ffprobe", err.Error()))
	} else {
		s.Checks = append(s.Checks, ok("ffprobe", strings.SplitN(string(out), "\n", 2)[0]))
	}
	for _, enc := range []string{"libx264", "aac"} {
		out, err := exec.Command(media.FFmpegPath(), "-hide_banner", "-encoders").Output()
		if err != nil || !strings.Contains(string(out), enc) {
			s.Checks = append(s.Checks, fail("codec "+enc, "encoder not available"))
		} else {
			s.Checks = append(s.Checks, ok("codec "+enc, "available"))
		}
	}
	return s
}

func BrowserSection() Section {
	s := Section{Name: "Browser"}
	s.Checks = append(s.Checks, ok("playwright-go pin", fmt.Sprintf("%s (min %s)", version.PlaywrightGoPin, version.PlaywrightGoMinimum)))
	s.Checks = append(s.Checks, BrowserDriverCheck())
	if p, err := exec.LookPath("google-chrome"); err == nil {
		s.Checks = append(s.Checks, ok("system chrome", p))
	} else if p, err := exec.LookPath("chromium"); err == nil {
		s.Checks = append(s.Checks, ok("system chromium", p))
	} else {
		s.Checks = append(s.Checks, warn("system browser", "no system chrome/chromium; playwright bundled chromium is used"))
	}
	for _, p := range []string{filepath.Join(os.Getenv("HOME"), ".cache", "ms-playwright"), filepath.Join(os.Getenv("HOME"), ".cache", "ms-playwright-go")} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			entries, _ := os.ReadDir(p)
			names := []string{}
			for _, e := range entries {
				names = append(names, e.Name())
			}
			s.Checks = append(s.Checks, ok("browser cache "+p, strings.Join(names, ", ")))
		}
	}
	return s
}

func BrowserDriverCheck() Check {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".cache", "ms-playwright-go"),
		filepath.Join(home, ".cache", "ms-playwright"),
	}
	found := []string{}
	for _, c := range candidates {
		if entries, err := os.ReadDir(c); err == nil {
			for _, e := range entries {
				found = append(found, e.Name())
			}
		}
	}
	if len(found) == 0 {
		return warn("playwright driver", "no cached browsers yet (run autodoc browser install)")
	}
	return ok("playwright driver", "cached: "+strings.Join(found, ", "))
}

func InstallBrowsers(w io.Writer) error {
	cmd := exec.Command("go", "run", "github.com/mxschmitt/playwright-go/cmd/playwright", "install", "--with-deps", "chromium")
	_ = cmd
	if err := playwright.Install(); err != nil {
		return fmt.Errorf("playwright install: %w", err)
	}
	fmt.Fprintln(w, "playwright browsers installed")
	return nil
}

func ProfileDir(profile string) string {
	d, err := install.DataDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		d = filepath.Join(home, ".local", "share", "autodoc")
	}
	return filepath.Join(d, "profiles", profile)
}

func BrowserLogin(profile, url string) error {
	dir := ProfileDir(profile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := playwright.Install(); err != nil {
		return err
	}
	pw, err := playwright.Run()
	if err != nil {
		return err
	}
	defer pw.Stop()
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	})
	if err != nil {
		return fmt.Errorf("launch headed chromium for login: %w (headless environments: run with xvfb or provision storage_state instead)", err)
	}
	defer browser.Close()
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{})
	if err != nil {
		return err
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		return err
	}
	target := url
	if target == "" {
		target = "about:blank"
	}
	if _, err := page.Goto(target); err != nil {
		return err
	}
	fmt.Printf("Authenticate in the opened window, then press Enter here to save session to %s\n", dir)
	fmt.Scanln()
	statePath := filepath.Join(dir, "storage-state.json")
	_, err = ctx.StorageState(playwright.BrowserContextStorageStateOptions{Path: playwright.String(statePath)})
	if err != nil {
		return err
	}
	fmt.Printf("session saved to %s\n", statePath)
	return nil
}

func TTSSection() Section {
	s := Section{Name: "TTS"}
	cwd, _ := os.Getwd()
	cwd, _ = filepath.Abs(cwd)
	lc, err := config.FindConfig(cwd)
	if err != nil || lc.Source == "default" {
		s.Checks = append(s.Checks, warn("config", "autodoc.toml not found"))
		return s
	}
	cfgPath := lc.Path
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		s.Checks = append(s.Checks, warn("config", "autodoc.toml not found"))
		return s
	}
	text := string(data)
	provider := "openai-compatible"
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "provider") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				provider = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
		}
		if strings.Contains(line, "sk-live") || strings.Contains(line, "sk-test") || strings.Contains(line, "ghp_") {
			s.Checks = append(s.Checks, fail("secrets", "possible secret literal in autodoc.toml (use env vars)"))
		}
	}
	s.Checks = append(s.Checks, ok("provider", redactValue(provider)+" ("+lc.Source+")"))
	if provider == "disabled" {
		s.Checks = append(s.Checks, ok("mode", "silent pacing, no network needed"))
		return s
	}
	baseURL := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "base_url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				baseURL = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			}
		}
	}
	if baseURL == "" {
		s.Checks = append(s.Checks, fail("endpoint", "tts.base_url missing"))
	} else {
		s.Checks = append(s.Checks, ok("endpoint", baseURL))
	}
	return s
}

func HarnessSection() Section {
	s := Section{Name: "Harnesses"}
	home := harness.HomeDir()
	for _, h := range harness.All() {
		pr := h.Probe(home)
		switch {
		case pr.Configured:
			s.Checks = append(s.Checks, ok(h.Name, "skill+mcp configured"))
		case pr.Partial:
			s.Checks = append(s.Checks, warn(h.Name, "partially configured"))
		case pr.Found:
			s.Checks = append(s.Checks, warn(h.Name, "found, autodoc not configured"))
		default:
			if h.Name == "cursor" {
				s.Checks = append(s.Checks, warn(h.Name, "not found (best-effort)"))
			} else {
				s.Checks = append(s.Checks, Check{Name: h.Name, Status: "warn", Detail: "not found"})
			}
		}
	}
	return s
}

func SkillSection() Section {
	s := Section{Name: "Skill"}
	home := harness.HomeDir()
	canonical := ""
	for _, c := range []string{"src/skill/autodoc/SKILL.md"} {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			canonical = abs
		}
	}
	if canonical == "" {
		if exe, err := os.Executable(); err == nil {
			c := filepath.Join(filepath.Dir(exe), "..", "share", "autodoc", "skill", "SKILL.md")
			if _, err := os.Stat(c); err == nil {
				canonical = c
			}
		}
	}
	if canonical == "" {
		if d, err := install.DataDir(); err == nil {
			c := filepath.Join(d, "skill", "SKILL.md")
			if _, err := os.Stat(c); err == nil {
				canonical = c
			}
		}
	}
	if canonical == "" {
		s.Checks = append(s.Checks, warn("canonical", "SKILL.md not found next to binary (dev checkout?)"))
	} else {
		s.Checks = append(s.Checks, ok("canonical", canonical+" (v"+version.CanonicalSkillVersion+")"))
	}
	for _, h := range harness.All() {
		for _, sp := range h.SkillPaths {
			p := sp
			if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
			}
			if _, err := os.Stat(p); err == nil {
				s.Checks = append(s.Checks, ok("installed "+h.Name, p))
			}
		}
	}
	return s
}

func FilesystemSection() Section {
	s := Section{Name: "Filesystem"}
	for name, fn := range map[string]func() (string, error){
		"config":   install.ConfigDir,
		"data":     install.DataDir,
		"cache":    install.CacheDir,
		"profiles": install.ProfilesDir,
	} {
		d, err := fn()
		if err != nil {
			s.Checks = append(s.Checks, fail(name, err.Error()))
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			s.Checks = append(s.Checks, fail(name, d+": "+err.Error()))
			continue
		}
		if f, err := os.CreateTemp(d, ".writetest"); err != nil {
			s.Checks = append(s.Checks, fail(name, d+" not writable"))
		} else {
			f.Close()
			os.Remove(f.Name())
			s.Checks = append(s.Checks, ok(name, redactHome(d)))
		}
	}
	return s
}

func redactValue(v string) string {
	l := strings.ToLower(v)
	if strings.Contains(l, "key") || strings.Contains(l, "secret") || strings.Contains(l, "token") {
		return "[redacted]"
	}
	return v
}

func redactHome(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
