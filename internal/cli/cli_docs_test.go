package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCliDocs keeps docs/cli.md in sync with the Cobra tree.
// Regenerate: AUTODOC_UPDATE_CLI_DOCS=1 go test ./internal/cli/ -run TestCliDocs
func TestCliDocs(t *testing.T) {
	combined := "# CLI reference\n\n"
	combined += "_Generated from the Cobra command tree. Do not edit by hand; run the test with AUTODOC_UPDATE_CLI_DOCS=1 to regenerate._\n"
	var walk func(cmd *cobra.Command, parents []string)
	walk = func(cmd *cobra.Command, parents []string) {
		path := append(append([]string{}, parents...), cmd.Name())
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			// Still descend: completion/help are cobra built-ins users never need here.
		} else if len(parents) >= 0 && cmd.Runnable() {
			full := "autodoc " + strings.Join(path[1:], " ")
			full = strings.TrimSpace(full)
			combined += "\n## `" + full + "`\n\n"
			if cmd.Short != "" {
				combined += cmd.Short + "\n\n"
			}
			if cmd.Long != "" && cmd.Long != cmd.Short {
				combined += cmd.Long + "\n\n"
			}
			if cmd.Example != "" {
				combined += "```sh\n" + strings.TrimSpace(cmd.Example) + "\n```\n\n"
			}
			if cmd.HasAvailableFlags() {
				combined += "```\n" + strings.TrimSpace(cmd.Flags().FlagUsages()) + "\n```\n\n"
			}
			if cmd.HasAvailableSubCommands() {
				names := []string{}
				for _, sub := range cmd.Commands() {
					if !sub.Hidden && sub.Name() != "help" {
						names = append(names, "`"+strings.TrimSpace(full+" "+sub.Name())+"` — "+sub.Short)
					}
				}
				sort.Strings(names)
				combined += "Subcommands:\n\n"
				for _, n := range names {
					combined += "- " + n + "\n"
				}
				combined += "\n"
			}
		}
		for _, sub := range cmd.Commands() {
			if sub.Name() == "help" {
				continue
			}
			walk(sub, path)
		}
	}
	walk(NewRoot(), nil)
	target := filepath.Join("..", "..", "docs", "cli.md")
	if os.Getenv("AUTODOC_UPDATE_CLI_DOCS") == "1" {
		if err := os.WriteFile(target, []byte(combined), 0o644); err != nil {
			t.Fatalf("write docs/cli.md: %v", err)
		}
		return
	}
	want, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read docs/cli.md: %v (regenerate with AUTODOC_UPDATE_CLI_DOCS=1)", err)
	}
	if string(want) != combined {
		t.Fatalf("docs/cli.md is stale: regenerate with AUTODOC_UPDATE_CLI_DOCS=1 go test ./internal/cli/ -run TestCliDocs")
	}
}
