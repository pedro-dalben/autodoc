package smoke

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot locates the repository root from the test working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
func runScript(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("sh", append([]string{"install.sh"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatalf("run install.sh: %v", err)
		}
	}
	return string(out), code
}

// fakeRelease builds a local file:// release tree with a real autodoc binary.
func fakeRelease(t *testing.T, version string, tamper bool) (base string, installDir string) {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()

	binDir := filepath.Join(tmp, "bin-src")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(binDir, "autodoc")
	build := exec.Command("go", "build", "-o", bin, "./cmd/autodoc")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build autodoc: %v\n%s", err, out)
	}

	// tar.gz containing the binary at archive root, like GoReleaser archives.
	assetDir := filepath.Join(tmp, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// GoReleaser strips the leading v from asset names
	// (autodoc_0.1.0_linux_amd64.tar.gz for tag v0.1.0).
	asset := "autodoc_" + strings.TrimPrefix(version, "v") + "_linux_amd64.tar.gz"
	tar := exec.Command("tar", "-czf", filepath.Join(assetDir, asset), "-C", binDir, "autodoc")
	if out, err := tar.CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	if tamper {
		f, err := os.OpenFile(filepath.Join(assetDir, asset), os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteAt([]byte("tampered"), 0)
		f.Close()
	}
	sumOut, err := exec.Command("sha256sum", filepath.Join(assetDir, asset)).Output()
	if err != nil {
		t.Fatal("sha256sum not available")
	}
	sum := strings.Fields(string(sumOut))[0]
	if tamper {
		sum = strings.Repeat("0", 64)
	}
	checksums := sum + "  " + asset + "\n"
	relDir := filepath.Join(tmp, "pedro-dalben", "autodoc", "releases", "download", version)
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(relDir, asset), mustRead(t, filepath.Join(assetDir, asset)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(relDir, "checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
	return "file://" + tmp, filepath.Join(tmp, "install-bin")
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInstallVersioned(t *testing.T) {
	base, dir := fakeRelease(t, "v9.9.9-smoke", false)
	env := []string{
		"AUTODOC_VERSION=v9.9.9-smoke",
		"AUTODOC_RELEASE_BASE=" + base,
		"AUTODOC_INSTALL_DIR=" + dir,
		"PATH=/usr/bin:/bin",
	}
	out, code := runScript(t, env)
	if code != 0 {
		t.Fatalf("install failed:\n%s", out)
	}
	ver := exec.Command(filepath.Join(dir, "autodoc"), "version")
	verOut, err := ver.CombinedOutput()
	if err != nil {
		t.Fatalf("installed binary failed to run: %v\n%s", err, verOut)
	}
	if !strings.Contains(string(verOut), "autodoc") {
		t.Fatalf("unexpected version output: %s", verOut)
	}
	// Re-install over the top must also succeed.
	if out, code := runScript(t, env); code != 0 {
		t.Fatalf("re-install failed:\n%s", out)
	}
}

func TestInstallUnknownVersion(t *testing.T) {
	base, dir := fakeRelease(t, "v9.9.9-smoke", false)
	env := []string{
		"AUTODOC_VERSION=v0.0.0-does-not-exist",
		"AUTODOC_RELEASE_BASE=" + base,
		"AUTODOC_INSTALL_DIR=" + dir,
		"PATH=/usr/bin:/bin",
	}
	out, code := runScript(t, env)
	if code == 0 {
		t.Fatalf("expected failure for unknown version, got success:\n%s", out)
	}
	if !strings.Contains(out, "not found") {
		t.Fatalf("error should name the missing asset, got:\n%s", out)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	base, dir := fakeRelease(t, "v9.9.9-smoke", true)
	env := []string{
		"AUTODOC_VERSION=v9.9.9-smoke",
		"AUTODOC_RELEASE_BASE=" + base,
		"AUTODOC_INSTALL_DIR=" + dir,
		"PATH=/usr/bin:/bin",
	}
	out, code := runScript(t, env)
	if code == 0 {
		t.Fatalf("expected failure for tampered asset, got success:\n%s", out)
	}
	if !strings.Contains(out, "checksum mismatch") {
		t.Fatalf("error should report checksum mismatch, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "autodoc")); !os.IsNotExist(err) {
		t.Fatalf("tampered binary must not be installed")
	}
}
