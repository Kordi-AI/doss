package vault

import (
	"os"
	"os/exec"
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
		"self", "peers", "notes", "devices",
		"policy.yaml", "INSTRUCTION.md", "CONTENT.md", "DISCLOSURE.md", "README.md", ".gitignore",
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

func TestInstructionPathAndEnsureInstruction(t *testing.T) {
	dir := t.TempDir()
	if got := InstructionPath(dir); got != filepath.Join(dir, "INSTRUCTION.md") {
		t.Fatalf("empty vault instruction path = %q", got)
	}

	if err := EnsureInstruction(dir); err != nil {
		t.Fatal(err)
	}
	if got := InstructionPath(dir); got != filepath.Join(dir, "INSTRUCTION.md") {
		t.Fatalf("primary instruction path = %q", got)
	}
	for _, f := range []string{"INSTRUCTION.md", "CONTENT.md", "DISCLOSURE.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("EnsureInstruction should seed %s: %v", f, err)
		}
	}
}

func TestInstructionTemplatesExplainFactShapeAndDisclosure(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	mustRead := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	entry := mustRead("INSTRUCTION.md")
	content := mustRead("CONTENT.md")
	disclosure := mustRead("DISCLOSURE.md")
	if !strings.Contains(entry, "CONTENT.md") || !strings.Contains(entry, "DISCLOSURE.md") {
		t.Fatalf("INSTRUCTION.md should route to split instruction files:\n%s", entry)
	}
	for _, want := range []string{
		"Common `self/**/*.md` fact shape",
		"The `rough:` field is the ONLY rough value",
		"`rough` is required only for `self/` topics that `policy.yaml` shares at `rough` level",
		"Everything after the closing `---` is the full private fact body",
		"There is no `no:` field inside a fact",
		"Peer note at `peers/kordi-pedro/team.md`",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("CONTENT.md should contain %q", want)
		}
	}
	for _, want := range []string{
		"[Trusted current request metadata]",
		"Contact Onboarding",
		"If policy grants `rough` but the fact has no valid `rough:` value, disclose nothing",
		"doss log --record --to <verified-id> --shared <topic> --level <rough|full>",
	} {
		if !strings.Contains(disclosure, want) {
			t.Fatalf("DISCLOSURE.md should contain %q", want)
		}
	}
}

func TestRegisterAndDeactivateDevice(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-b", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	dev, err := RegisterDevice(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dev.ID == "" || dev.Status != "active" || dev.RegisteredAt == "" {
		t.Fatalf("bad registered device: %+v", dev)
	}
	if _, err := os.Stat(filepath.Join(dir, DeviceFile(dev.ID))); err != nil {
		t.Fatalf("registered device file missing: %v", err)
	}
	devices, err := Devices(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].ID != dev.ID {
		t.Fatalf("Devices() = %+v, want current device", devices)
	}
	withKey, err := SetDeviceDeployKey(dir, dev.ID, "owner/repo", "doss "+dev.ID, "SHA256:abc", 42)
	if err != nil {
		t.Fatal(err)
	}
	if withKey.GitHubRepo != "owner/repo" || withKey.DeployKeyID != 42 || withKey.DeployKeyFingerprint == "" {
		t.Fatalf("deploy key metadata not recorded: %+v", withKey)
	}

	deactivated, err := DeactivateDevice(dir, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deactivated.Status != "deactivated" || deactivated.DeactivatedAt == "" {
		t.Fatalf("bad deactivated device: %+v", deactivated)
	}
}
