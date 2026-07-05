package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kordi-AI/doss/internal/vault"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func initTestVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := vault.Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.name", "Test Owner")
	runGit(t, dir, "config", "user.email", "owner@example.com")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func TestConnectRequiresExistingVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", filepath.Join(t.TempDir(), "missing"))

	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(codex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codex, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdConnect(nil); err == nil {
		t.Fatal("cmdConnect should fail when the vault is missing")
	}
	raw, err := os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), beginMark) {
		t.Fatalf("connect wrote a doss section for a missing vault:\n%s", raw)
	}
}

func TestDoctorFixDoesNotWireMissingVault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", filepath.Join(t.TempDir(), "missing"))

	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(codex), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("# mine\n")
	if err := os.WriteFile(codex, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdDoctor([]string{"--fix"}); err == nil {
		t.Fatal("doctor should report the missing vault")
	}
	raw, err := os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(original) {
		t.Fatalf("doctor --fix modified wiring for a missing vault:\n%s", raw)
	}
}

func TestChangedCheckIncludesGitignoredLocalAccess(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	access := filepath.Join(dir, "local", "access.yaml")
	if err := os.WriteFile(access, []byte("grants:\n  ghost:\n    ~/p: read\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdCheck([]string{"--changed", "--quiet"}); err == nil {
		t.Fatal("check --changed should validate local/access.yaml even though it is gitignored")
	}
}

func TestSyncUsesCurrentBranchWhenNoUpstream(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)
	runGit(t, dir, "checkout", "-b", "trunk")

	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	runGit(t, dir, "remote", "add", "origin", remote)

	profile := filepath.Join(dir, "self", "profile.md")
	if err := os.WriteFile(profile, []byte("---\nrough: \"concise updates\"\n---\nPrefers concise updates.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync([]string{"--quiet"}); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "--git-dir", remote, "rev-parse", "--verify", "refs/heads/trunk")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sync did not push current branch trunk:\n%s", out)
	}

	runGit(t, dir, "commit", "--allow-empty", "-m", "ahead")
	if got := unpushedCount(dir); got != 1 {
		t.Fatalf("unpushedCount = %d, want 1 for current upstream branch", got)
	}
}

func TestDossSectionDoesNotMentionAnswerCommand(t *testing.T) {
	section := dossSection("/tmp/doss-test-vault")
	if strings.Contains(section, "doss answer") {
		t.Fatalf("managed section still points agents at a removed command:\n%s", section)
	}
	if !strings.Contains(section, "doss log --record") {
		t.Fatalf("managed section should tell agents to record disclosures:\n%s", section)
	}
}
