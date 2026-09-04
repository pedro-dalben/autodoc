package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAuthStateDoesNotTreatProfileNamedDirectoryAsStateFile(t *testing.T) {
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	if got := resolveAuthState("docs"); got == "docs" {
		t.Fatalf("directory must be resolved as a profile, got %q", got)
	}
}

func TestRootSubcommandsExist(t *testing.T) {
	root := NewRoot()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	// test explain --help
	root.SetArgs([]string{"explain", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("explain --help failed: %v", err)
	}

	// test diagnose --help
	root.SetArgs([]string{"diagnose", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("diagnose --help failed: %v", err)
	}

	// test agent state --help
	root.SetArgs([]string{"agent", "state", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("agent state --help failed: %v", err)
	}
}
