package smoke

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/cli"
	"github.com/spf13/cobra"
)

var linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
var fenceRe = regexp.MustCompile("(?m)^```(sh|bash|powershell|ps1)?[ \t]*$")

// mdFiles returns every user-facing Markdown file covered by the link check.
func mdFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var files []string
	roots := []string{root, filepath.Join(root, "docs")}
	for _, dir := range roots {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	// Recursive docs subdirectories (harnesses, shipped examples).
	filepath.Walk(filepath.Join(root, "docs"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".md") && !contains(files, p) {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestDocLinks(t *testing.T) {
	for _, file := range mdFiles(t) {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		// Strip fenced code blocks: links inside examples are illustrative.
		lines := strings.Split(string(data), "\n")
		var prose []string
		inFence := false
		for _, l := range lines {
			if fenceRe.MatchString(l) {
				inFence = !inFence
				continue
			}
			if !inFence {
				prose = append(prose, l)
			}
		}
		for _, m := range linkRe.FindAllStringSubmatch(strings.Join(prose, "\n"), -1) {
			target := m[1]
			if target == "" || strings.HasPrefix(target, "http") ||
				strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			target = strings.SplitN(target, "#", 2)[0]
			resolved := target
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(file), target)
			}
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s: broken link %q", file, m[1])
			}
		}
	}
}

func findCommand(root *cobra.Command, path []string) *cobra.Command {
	c, _, err := root.Find(path)
	if err != nil || c == nil {
		return nil
	}
	return c
}

// TestQuickstartCommands parses autodoc invocations out of the README and the
// getting-started guide and verifies every command path exists in the CLI.
func TestQuickstartCommands(t *testing.T) {
	root := repoRoot(t)
	callRe := regexp.MustCompile(`(?m)^\s*(?:\$ )?autodoc\s+([a-z][a-z0-9_-]*(?:\s+[a-z][a-z0-9_-]+)*)`)
	checked := map[string]bool{}
	for _, file := range []string{filepath.Join(root, "README.md"), filepath.Join(root, "docs", "getting-started.md")} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range callRe.FindAllStringSubmatch(string(data), -1) {
			fields := strings.Fields(m[1])
			// Trim trailing tokens that are flags, values, or scene ids.
			var path []string
			cmd := cli.NewRoot()
			for _, f := range fields {
				if strings.HasPrefix(f, "-") {
					break
				}
				candidate := append(append([]string{}, path...), f)
				if findCommand(cmd, candidate) == nil {
					break
				}
				// Only descend while the candidate names a subcommand that
				// itself has subcommands or is the final runnable target.
				path = candidate
				found := findCommand(cmd, path)
				if found.Runnable() && !found.HasSubCommands() {
					break
				}
			}
			if len(path) == 0 {
				t.Errorf("%s: unknown command in %q", file, m[0])
				continue
			}
			key := strings.Join(path, " ")
			if checked[key] {
				continue
			}
			checked[key] = true
			if c := findCommand(cli.NewRoot(), path); c == nil || !c.Runnable() {
				t.Errorf("%s: %q is not a runnable autodoc command", file, "autodoc "+key)
			}
		}
	}
	if len(checked) == 0 {
		t.Fatal("no autodoc invocations found in README/getting-started")
	}
}
