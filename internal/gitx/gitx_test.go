package gitx

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestCurrentBranchAndUpstream(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "trunk")
	runGit(t, dir, "config", "user.name", "Test Owner")
	runGit(t, dir, "config", "user.email", "owner@example.com")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")
	if got := CurrentBranch(dir); got != "trunk" {
		t.Fatalf("CurrentBranch = %q, want trunk", got)
	}
	if got := Upstream(dir); got != "" {
		t.Fatalf("Upstream without remote = %q, want empty", got)
	}

	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-u", "origin", "trunk")

	if got := Upstream(dir); got != "origin/trunk" {
		t.Fatalf("Upstream = %q, want origin/trunk", got)
	}
}
