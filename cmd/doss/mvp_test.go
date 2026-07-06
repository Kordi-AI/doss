package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestConnectWiresUpdatesAndRemovesPresetAgentFile(t *testing.T) {
	home := t.TempDir()
	dir := initTestVault(t)
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)

	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(codex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codex, []byte("# existing rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdConnect(nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, beginMark) || !strings.Contains(content, "Long-term preferences") {
		t.Fatalf("connect should add the managed Doss section, got:\n%s", content)
	}
	if strings.Contains(content, "\n\tLong-term preferences") {
		t.Fatalf("managed section body should be plain Markdown prose, got:\n%s", content)
	}

	if err := os.WriteFile(codex, []byte("# existing rules\n\n"+beginMark+"\nstale\n"+endMark+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdConnect(nil); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	content = string(raw)
	if strings.Contains(content, "stale") || strings.Count(content, beginMark) != 1 {
		t.Fatalf("connect should update the existing managed section in place, got:\n%s", content)
	}

	if err := cmdConnect([]string{"--remove"}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), beginMark) || strings.Contains(string(raw), endMark) {
		t.Fatalf("connect --remove should strip the managed section, got:\n%s", raw)
	}
}

func TestConnectCustomFilePersistsRefreshesAndRemoves(t *testing.T) {
	home := t.TempDir()
	dir := initTestVault(t)
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)

	custom := filepath.Join(home, "agent", "GLOBAL.md")
	if err := cmdConnect([]string{"--file", custom}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(custom)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), beginMark) {
		t.Fatalf("connect --file should write the custom target, got:\n%s", raw)
	}

	if err := os.WriteFile(custom, []byte(beginMark+"\nstale\n"+endMark+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdConnect(nil); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(custom)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "stale") || !strings.Contains(string(raw), "INSTRUCTION.md") {
		t.Fatalf("future connect runs should refresh saved custom targets, got:\n%s", raw)
	}

	if err := cmdConnect([]string{"--remove"}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(custom)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), beginMark) {
		t.Fatalf("connect --remove should strip saved custom targets, got:\n%s", raw)
	}
	if targets := loadCustomTargets(); len(targets) != 0 {
		t.Fatalf("connect --remove should clear saved custom targets, got %v", targets)
	}
}

func TestConnectRejectsCustomTargetInsideVault(t *testing.T) {
	home := t.TempDir()
	dir := initTestVault(t)
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)

	err := cmdConnect([]string{"--file", filepath.Join(dir, "INSTRUCTION.md")})
	if err == nil {
		t.Fatal("connect --file should reject vault-internal files")
	}
	if !strings.Contains(err.Error(), "not a file inside the vault") {
		t.Fatalf("unexpected error for vault-internal custom target: %v", err)
	}
	if targets := loadCustomTargets(); len(targets) != 0 {
		t.Fatalf("rejected custom target should not be persisted, got %v", targets)
	}
}

func TestDoctorFixRepairsAgentWiring(t *testing.T) {
	home := t.TempDir()
	dir := initTestVault(t)
	t.Setenv("HOME", home)
	t.Setenv("DOSS_HOME", dir)
	if _, err := vault.RegisterDevice(dir); err != nil {
		t.Fatal(err)
	}

	codex := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(codex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(codex, []byte("# existing rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := captureStdout(t, func() error { return cmdDoctor([]string{"--fix"}) }); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(codex)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), beginMark) || !strings.Contains(string(raw), "INSTRUCTION.md") {
		t.Fatalf("doctor --fix should repair missing agent wiring, got:\n%s", raw)
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

func TestChangedCheckReportsMalformedPolicyEvenWhenOnlySelfChanged(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	if err := os.WriteFile(filepath.Join(dir, "policy.yaml"), []byte("groups:\n  friends: [kordi:pedro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "policy.yaml")
	runGit(t, dir, "commit", "-m", "malformed policy baseline")

	profile := filepath.Join(dir, "self", "profile", "address.md")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("123 King St W\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return cmdCheck([]string{"--changed", "--quiet"})
	})
	if err == nil {
		t.Fatal("check --changed should fail when policy.yaml is malformed even if only a self fact changed")
	}
	if !strings.Contains(out, "policy.yaml:1 [E_YAML]") {
		t.Fatalf("check --changed should report the malformed baseline policy, got:\n%s", out)
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
	if err := cmdDevices([]string{"deactivate", old}); err == nil || !strings.Contains(err.Error(), "usage: doss devices") {
		t.Fatalf("devices should be read-only; use doss deactivate for mutation, got: %v", err)
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

func TestViewProjectsRequesterScopedFactsAndAccess(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	writeTestFile(t, filepath.Join(dir, "policy.yaml"), []byte(`groups:
  friends: [kordi:pedro]
  coworkers: [kordi:pedro]
  strangers: []
can-see:
  friends:
    profile/address: rough
    profile/dietary: full
    profile/missing: rough
    work: no
  coworkers:
    profile/address: no
`))
	writeTestFile(t, filepath.Join(dir, "local", "access.yaml"), []byte(`grants:
  friends:
    /tmp/project: read
    /tmp/write: full
    /tmp/nope: no
  strangers:
    /tmp/private: full
`))
	writeTestFile(t, filepath.Join(dir, "self", "profile", "address.md"), []byte("---\nrough: \"Toronto\"\n---\n123 Private Street, Toronto\n"))
	writeTestFile(t, filepath.Join(dir, "self", "profile", "dietary.md"), []byte("---\nsource: owner\n---\nSevere peanut allergy.\n"))
	writeTestFile(t, filepath.Join(dir, "self", "profile", "missing.md"), []byte("Private missing rough body.\n"))
	writeTestFile(t, filepath.Join(dir, "self", "profile", "suggested.md"), []byte("---\nstatus: suggested\nrough: \"draft\"\n---\nDraft fact.\n"))
	writeTestFile(t, filepath.Join(dir, "self", "work", "company.md"), []byte("---\nrough: \"software\"\n---\nPrivate employer.\n"))
	writeTestFile(t, filepath.Join(dir, "peers", "pedro.md"), []byte("Peer note.\n"))
	writeTestFile(t, filepath.Join(dir, "notes", "scratch.md"), []byte("Scratch note.\n"))

	out := filepath.Join(t.TempDir(), "pedro-view")
	stdout, err := captureStdout(t, func() error {
		return cmdView([]string{"--for", "kordi:pedro", "--out", out, "--ttl", "5m"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "view ready:") || !strings.Contains(stdout, "warning: 1 fact(s) omitted") {
		t.Fatalf("view output should include ready path and missing rough warning, got:\n%s", stdout)
	}

	assertFileEquals(t, filepath.Join(out, "self", "profile", "address.md"), "Toronto\n")
	assertFileContains(t, filepath.Join(out, "self", "profile", "dietary.md"), "Severe peanut allergy.")
	assertMissing(t, filepath.Join(out, "self", "profile", "missing.md"))
	assertMissing(t, filepath.Join(out, "self", "profile", "suggested.md"))
	assertMissing(t, filepath.Join(out, "self", "work", "company.md"))
	assertMissing(t, filepath.Join(out, "peers"))
	assertMissing(t, filepath.Join(out, "notes"))

	raw, err := os.ReadFile(filepath.Join(out, "access.json"))
	if err != nil {
		t.Fatal(err)
	}
	var access viewAccessOut
	if err := json.Unmarshal(raw, &access); err != nil {
		t.Fatal(err)
	}
	if access.Requester != "kordi:pedro" || len(access.Folders) != 2 {
		t.Fatalf("unexpected access projection: %+v", access)
	}
	if access.Folders[0] != (viewAccessFolder{Path: "/tmp/project", Level: "read"}) {
		t.Fatalf("unexpected first folder grant: %+v", access.Folders[0])
	}
	if access.Folders[1] != (viewAccessFolder{Path: "/tmp/write", Level: "full"}) {
		t.Fatalf("unexpected second folder grant: %+v", access.Folders[1])
	}

	raw, err = os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest viewManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if !manifest.DossView || manifest.Requester != "kordi:pedro" {
		t.Fatalf("manifest should identify the Doss view and requester: %+v", manifest)
	}
	if manifest.PolicyHash == "" || manifest.LocalAccessHash == "" || manifest.SelfTreeHash == "" || manifest.SourceVaultCommit == "" {
		t.Fatalf("manifest should include source hashes and commit: %+v", manifest)
	}
	if len(manifest.Blocked) != 1 || manifest.Blocked[0].Topic != "profile/missing" || manifest.Blocked[0].Reason != "missing rough" {
		t.Fatalf("manifest should record missing rough block: %+v", manifest.Blocked)
	}
	if _, err := time.Parse(time.RFC3339, manifest.ExpiresAt); err != nil {
		t.Fatalf("manifest should have RFC3339 expiry: %v", err)
	}
	assertFileContains(t, filepath.Join(out, "README.md"), "Do not read the raw vault")
}

func TestViewRefusesInvalidPolicyOrAccessBeforeExport(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	writeTestFile(t, filepath.Join(dir, "self", "profile", "address.md"), []byte("---\nrough: \"Toronto\"\n---\n123 Private Street, Toronto\n"))
	writeTestFile(t, filepath.Join(dir, "policy.yaml"), []byte(`groups:
  friends: [kordi:pedro]
can-see:
  friends:
    profile/address: read
`))
	out := filepath.Join(t.TempDir(), "bad-policy")
	err := cmdView([]string{"--for", "kordi:pedro", "--out", out})
	if err == nil || !strings.Contains(err.Error(), "refusing to export requester view") || !strings.Contains(err.Error(), "E_POLICY") {
		t.Fatalf("view should fail on invalid disclosure policy, got: %v", err)
	}
	assertMissing(t, out)

	writeTestFile(t, filepath.Join(dir, "policy.yaml"), []byte(`groups:
  friends: [kordi:pedro]
can-see:
  friends:
    profile/address: full
`))
	writeTestFile(t, filepath.Join(dir, "local", "access.yaml"), []byte(`grants:
  friends:
    /tmp/project: rough
`))
	out = filepath.Join(t.TempDir(), "bad-access")
	err = cmdView([]string{"--for", "kordi:pedro", "--out", out})
	if err == nil || !strings.Contains(err.Error(), "refusing to export requester view") || !strings.Contains(err.Error(), "E_ACCESS") {
		t.Fatalf("view should fail on invalid local access policy, got: %v", err)
	}
	assertMissing(t, out)
}

func TestViewRejectsUnsafeOutputAndCleansExpiredViews(t *testing.T) {
	dir := initTestVault(t)
	t.Setenv("DOSS_HOME", dir)

	if err := cmdView([]string{"--for", "Pedro", "--out", filepath.Join(t.TempDir(), "bad")}); err == nil {
		t.Fatal("view should reject unverified requester ids")
	}
	if err := cmdView([]string{"--for", "kordi:pedro", "--out", filepath.Join(dir, "tmp-view")}); err == nil {
		t.Fatal("view should reject output paths inside the raw vault")
	}
	existing := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmdView([]string{"--for", "kordi:pedro", "--out", existing}); err == nil {
		t.Fatal("view should not overwrite directories it did not create")
	}

	parent := t.TempDir()
	expired := filepath.Join(parent, "expired")
	live := filepath.Join(parent, "live")
	plain := filepath.Join(parent, "plain")
	if err := os.MkdirAll(expired, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(expired, "manifest.json"), viewManifest{
		DossView:  true,
		ExpiresAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	})
	writeTestJSON(t, filepath.Join(live, "manifest.json"), viewManifest{
		DossView:  true,
		ExpiresAt: time.Now().UTC().Add(time.Minute).Format(time.RFC3339),
	})
	if err := cmdView([]string{"cleanup", "--dir", parent}); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, expired)
	assertExists(t, live)
	assertExists(t, plain)
}

func writeTestFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, b)
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), want) {
		t.Fatalf("%s should contain %q, got:\n%s", path, want, raw)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s should exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should be missing, stat err: %v", path, err)
	}
}
