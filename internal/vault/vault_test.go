package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirHonorsEnv(t *testing.T) {
	t.Setenv("DOSS_HOME", "/tmp/doss-test-home")
	if Dir() != "/tmp/doss-test-home" {
		t.Errorf("Dir() ignored DOSS_HOME: %q", Dir())
	}
}

func TestScaffoldAndExists(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Error("empty dir should not look like a vault")
	}
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"self", "peers", "notes",
		"policy.yaml", "SKILL.md", "README.md", ".gitignore",
		filepath.Join("local", "access.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("scaffold missing %s", f)
		}
	}
	if !Exists(dir) {
		t.Error("Exists false after scaffold")
	}

	// local/ must be gitignored by the vault's .gitignore
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ln := range strings.Split(string(gi), "\n") {
		if strings.TrimSpace(ln) == "local/" {
			found = true
		}
	}
	if !found {
		t.Error("vault .gitignore does not exclude local/")
	}
}
