package cli

import (
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
