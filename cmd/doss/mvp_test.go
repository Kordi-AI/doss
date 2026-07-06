package main

import (
	"bytes"
	"io"
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

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatal(copyErr)
	}
	return buf.String(), err
}

func TestInitRegistersDeviceAndPrintsDevices(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(t.TempDir(), "vault")
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)

	out, err := captureStdout(t, func() error {
		return cmdInit([]string{"--no-connect", "--git-name", "Test Owner", "--git-email", "owner@example.com"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "devices: 1 active / 1 total") {
		t.Fatalf("init should print device registry summary, got:\n%s", out)
	}
	if !strings.Contains(out, "* ") {
		t.Fatalf("init should mark the current device, got:\n%s", out)
	}
	listed := exec.Command("git", "-C", dir, "ls-files", "devices/*.yaml")
	raw, err := listed.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files devices failed: %v\n%s", err, raw)
	}
	if strings.TrimSpace(string(raw)) == "" {
		t.Fatalf("device registration should be git-tracked, got no devices/*.yaml")
	}
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
	cmd = exec.Command("git", "--git-dir", remote, "ls-tree", "-r", "--name-only", "refs/heads/trunk")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reading remote tree failed:\n%s", out)
	} else if !strings.Contains(string(out), "devices/") {
		t.Fatalf("sync should register and push the current device, remote tree:\n%s", out)
	}

	runGit(t, dir, "commit", "--allow-empty", "-m", "ahead")
	if got := unpushedCount(dir); got != 1 {
		t.Fatalf("unpushedCount = %d, want 1 for current upstream branch", got)
	}
}

func TestSyncRechecksCurrentDeviceAfterPull(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)
	id := "test-device"
	runGit(t, dir, "config", "--local", "doss.device", id)

	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	runGit(t, dir, "remote", "add", "origin", remote)
	if err := cmdSync([]string{"--quiet"}); err != nil {
		t.Fatal(err)
	}

	admin := filepath.Join(t.TempDir(), "admin")
	runGit(t, t.TempDir(), "clone", "--branch", "main", remote, admin)
	runGit(t, admin, "config", "user.name", "Admin")
	runGit(t, admin, "config", "user.email", "admin@example.com")
	devFile := filepath.Join(admin, "devices", id+".yaml")
	raw, err := os.ReadFile(devFile)
	if err != nil {
		t.Fatal(err)
	}
	deactivated := strings.ReplaceAll(string(raw), "status: active", "status: deactivated")
	deactivated = strings.ReplaceAll(deactivated, "deactivated_at: \"\"", "deactivated_at: \"2026-07-06T12:00:00Z\"")
	if err := os.WriteFile(devFile, []byte(deactivated), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, admin, "add", "-A")
	runGit(t, admin, "commit", "-m", "deactivate device")
	runGit(t, admin, "push")

	profile := filepath.Join(dir, "self", "profile", "style.md")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("Prefers short updates.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdSync([]string{"--quiet"}); err == nil || !strings.Contains(err.Error(), "is deactivated") {
		t.Fatalf("sync should stop after pulling current-device deactivation, got: %v", err)
	}

	cmd := exec.Command("git", "--git-dir", remote, "ls-tree", "-r", "--name-only", "main")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reading remote tree failed:\n%s", out)
	}
	if strings.Contains(string(out), "self/profile/style.md") {
		t.Fatalf("deactivated device should not push new owner facts, remote tree:\n%s", out)
	}
}

func TestSyncRefusesDeactivatedCurrentDevice(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)
	dev, err := vault.RegisterDevice(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.DeactivateDevice(dir, dev.ID); err != nil {
		t.Fatal(err)
	}

	err = cmdSync([]string{"--quiet"})
	if err == nil {
		t.Fatal("sync should refuse a deactivated current device")
	}
	if !strings.Contains(err.Error(), "is deactivated") {
		t.Fatalf("sync should explain deactivated device, got: %v", err)
	}
}

func TestUninstallPushesDeviceUnregistration(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(t.TempDir(), "vault")
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "init", "--bare", remote)
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)

	if err := cmdInit([]string{"--no-connect", "--remote", remote, "--git-name", "Test Owner", "--git-email", "owner@example.com"}); err != nil {
		t.Fatal(err)
	}
	id := vault.DeviceID(dir)
	if err := cmdUninstall([]string{"--yes", "--keep-agents"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("vault should be deleted, stat err: %v", err)
	}
	cmd := exec.Command("git", "--git-dir", remote, "show", "refs/heads/main:devices/"+id+".yaml")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remote device registration missing:\n%s", out)
	}
	if !strings.Contains(string(out), "status: deactivated") {
		t.Fatalf("uninstall should push deactivated status, got:\n%s", out)
	}
}

func TestDeactivateAnotherRegisteredDevice(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)
	if _, err := vault.RegisterDevice(dir); err != nil {
		t.Fatal(err)
	}
	old := "old-device"
	oldFile := filepath.Join(dir, "devices", old+".yaml")
	if err := os.WriteFile(oldFile, []byte("id: old-device\nlabel: Old Device\nstatus: active\nregistered_at: \"2026-07-05T12:00:00Z\"\ndeactivated_at: \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "devices")

	if err := cmdDeactivate([]string{vault.DeviceID(dir)}); err == nil {
		t.Fatal("deactivate should reject the current device")
	}
	if err := cmdDeactivate([]string{"missing-device"}); err == nil {
		t.Fatal("deactivate should reject unknown devices")
	}
	if err := cmdDeactivate(nil); err == nil || !strings.Contains(err.Error(), "choose a device") {
		t.Fatalf("deactivate without id should require an interactive terminal, got: %v", err)
	}
	if err := cmdDeactivate([]string{old}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(oldFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "status: deactivated") {
		t.Fatalf("device should be marked deactivated, got:\n%s", raw)
	}
	if err := cmdDeactivate([]string{old}); err == nil || !strings.Contains(err.Error(), "already deactivated") {
		t.Fatalf("deactivate should reject already deactivated devices, got: %v", err)
	}

	alias := "alias-device"
	aliasFile := filepath.Join(dir, "devices", alias+".yaml")
	if err := os.WriteFile(aliasFile, []byte("id: alias-device\nlabel: Alias Device\nstatus: active\nregistered_at: \"2026-07-05T12:00:00Z\"\ndeactivated_at: \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "alias device")
	if err := cmdDevices([]string{"deactivate", alias}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(aliasFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "status: deactivated") {
		t.Fatalf("devices deactivate alias should work, got:\n%s", raw)
	}
}

func TestDossSectionDoesNotMentionRemovedDisclosureCommand(t *testing.T) {
	section := dossSection("/tmp/doss-test-vault")
	removedCommand := "doss " + "answer"
	if strings.Contains(section, removedCommand) {
		t.Fatalf("managed section still points agents at a removed command:\n%s", section)
	}
	if !strings.Contains(section, "doss log --record") {
		t.Fatalf("managed section should tell agents to record disclosures:\n%s", section)
	}
	if !strings.Contains(section, "--level rough|full") {
		t.Fatalf("managed section should tell agents to record disclosure level:\n%s", section)
	}
	if !strings.Contains(section, "CONTENT.md") || !strings.Contains(section, "DISCLOSURE.md") {
		t.Fatalf("managed section should mention split instruction files:\n%s", section)
	}
	if !strings.Contains(section, "unless `policy.yaml` explicitly permits it") {
		t.Fatalf("managed section should allow only policy-permitted outbound disclosure:\n%s", section)
	}
	if !strings.Contains(section, "INSTRUCTION.md") {
		t.Fatalf("managed section should point agents at INSTRUCTION.md:\n%s", section)
	}
}

func TestLogRequiresAndRecordsDisclosureLevel(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	if err := cmdLog([]string{"--record", "--to", "kordi:pedro", "--shared", "profile/address"}); err == nil {
		t.Fatal("log --record should require --level")
	}
	if err := cmdLog([]string{"--record", "--to", "Pedro", "--shared", "profile/address", "--level", "rough"}); err == nil {
		t.Fatal("log --record should reject unverified display-name recipients")
	}
	if err := cmdLog([]string{"--record", "--to", "kordi:pedro", "--shared", "self/profile/address", "--level", "rough"}); err == nil {
		t.Fatal("log --record should reject shared topics with self/ prefix")
	}
	if err := cmdLog([]string{"--record", "--to", "kordi:pedro", "--shared", "profile/address", "--level", "rough"}); err != nil {
		t.Fatal(err)
	}
	entries, _, err := readLedger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}
	if entries[0].Level != "rough" || entries[0].Shared != "profile/address" || entries[0].To != "kordi:pedro" {
		t.Fatalf("ledger entry not recorded correctly: %+v", entries[0])
	}
}
