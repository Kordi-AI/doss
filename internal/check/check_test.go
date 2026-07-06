package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hasCode(issues []Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}

func issueByCode(issues []Issue, code string) (Issue, bool) {
	for _, i := range issues {
		if i.Code == code {
			return i, true
		}
	}
	return Issue{}, false
}

func hasIssue(issues []Issue, file, code string) bool {
	file = filepath.ToSlash(file)
	for _, i := range issues {
		if filepath.ToSlash(i.File) == file && i.Code == code {
			return true
		}
	}
	return false
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFrontmatter(t *testing.T) {
	cases := []struct {
		name string
		fm   string
		code string // expected issue code; "" = expect none
	}{
		{"valid", "source: owner\nconfidence: high", ""},
		{"unknown key", "foo: bar", "E_FIELD"},
		{"bad source", "source: alien", "E_VALUE"},
		{"bad status", "status: bogus", "E_VALUE"},
		{"inferred needs suggested", "source: inferred\nstatus: active", "E_RULE"},
		{"inferred+suggested ok", "source: inferred\nstatus: suggested", ""},
		{"bad verify_by", "verify_by: soon", "E_VALUE"},
		{"good verify_by", "verify_by: 2027-01-02", ""},
		{"x- extension allowed", "x-custom: anything", ""},
		{"rough ok", `rough: "Toronto"`, ""},
		{"rough must be string", "rough: [a, b]", "E_VALUE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := checkFrontmatter("peers/x.md", []byte(c.fm), false)
			if c.code == "" {
				if len(issues) != 0 {
					t.Fatalf("expected clean, got %v", issues)
				}
				return
			}
			if !hasCode(issues, c.code) {
				t.Fatalf("expected %s, got %v", c.code, issues)
			}
		})
	}
}

func TestVaultCleanAndProblems(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "self", "profile", "dietary.md"), "---\nrough: \"peanut allergy\"\n---\n- peanuts\n")
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile: rough\n")

	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("clean vault should pass, got %v", issues)
	}

	write(t, filepath.Join(dir, "self", "profile", "Bad.md"), "x\n") // E_NAME
	write(t, filepath.Join(dir, "self", "profile", "empty.md"), "")  // E_EMPTY
	write(t, filepath.Join(dir, "stray.txt"), "junk\n")              // E_LAYOUT

	issues, _ = Vault(dir)
	for _, want := range []string{"E_NAME", "E_EMPTY", "E_LAYOUT"} {
		if !hasCode(issues, want) {
			t.Errorf("expected %s, got %v", want, issues)
		}
	}
}

func TestSelfMarkdownRequiresRoughOnlyWhenPolicySharesRough(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile/address: rough\n")

	write(t, filepath.Join(dir, "self", "profile", "address.md"), "123 King St W\n")
	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(issues, "self/profile/address.md", "E_ROUGH") {
		t.Fatalf("rough-shared self markdown without frontmatter rough should fail, got %v", issues)
	}
	if is, ok := issueByCode(issues, "E_ROUGH"); !ok || !strings.Contains(is.Msg, "rough-shared") {
		t.Fatalf("E_ROUGH should explain the rough-sharing requirement, got %v", issues)
	}

	write(t, filepath.Join(dir, "self", "profile", "address.md"), "---\nsource: owner\n---\n123 King St W\n")
	issues, err = Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(issues, "self/profile/address.md", "E_ROUGH") {
		t.Fatalf("rough-shared self markdown frontmatter without rough should fail, got %v", issues)
	}

	write(t, filepath.Join(dir, "self", "profile", "address.md"), "---\nrough: \"Toronto\"\n---\n123 King St W\n")
	issues, err = Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasCode(issues, "E_ROUGH") {
		t.Fatalf("self markdown with rough should pass rough check, got %v", issues)
	}
}

func TestSelfMarkdownDoesNotRequireRoughForFullNoOrUnlisted(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile/address: full\n    profile/dietary: no\n")
	write(t, filepath.Join(dir, "self", "profile", "address.md"), "123 King St W\n")
	write(t, filepath.Join(dir, "self", "profile", "dietary.md"), "Peanut allergy.\n")
	write(t, filepath.Join(dir, "self", "work", "style.md"), "Prefers concise updates.\n")

	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hasCode(issues, "E_ROUGH") {
		t.Fatalf("full/no/unlisted self facts should not require rough, got %v", issues)
	}
}

func TestPolicySpecificityControlsRoughRequirement(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile: rough\n    profile/address: full\n    work: rough\n    work/private: no\n")
	write(t, filepath.Join(dir, "self", "profile", "name.md"), "Shenzhe.\n")
	write(t, filepath.Join(dir, "self", "profile", "address.md"), "123 King St W\n")
	write(t, filepath.Join(dir, "self", "work", "style.md"), "Concise updates.\n")
	write(t, filepath.Join(dir, "self", "work", "private", "note.md"), "Private work note.\n")

	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"self/profile/name.md", "self/work/style.md"} {
		if !hasIssue(issues, want, "E_ROUGH") {
			t.Fatalf("%s should need rough due to inherited rough policy, got %v", want, issues)
		}
	}
	for _, want := range []string{"self/profile/address.md", "self/work/private/note.md"} {
		if hasIssue(issues, want, "E_ROUGH") {
			t.Fatalf("%s should not need rough because a more specific rule overrides it, got %v", want, issues)
		}
	}
}

func TestChangedPolicyChecksExistingRoughRequirements(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile/address: rough\n")
	write(t, filepath.Join(dir, "self", "profile", "address.md"), "123 King St W\n")

	issues, err := Files(dir, []string{"policy.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(issues, "self/profile/address.md", "E_ROUGH") {
		t.Fatalf("changing policy to rough should surface existing facts missing rough, got %v", issues)
	}
}

func TestChangedSelfFileUsesPolicyRoughRequirement(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile/address: rough\n")
	write(t, filepath.Join(dir, "self", "profile", "address.md"), "123 King St W\n")

	issues, err := Files(dir, []string{"self/profile/address.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !hasIssue(issues, "self/profile/address.md", "E_ROUGH") {
		t.Fatalf("changed rough-shared self fact should require rough, got %v", issues)
	}
}

func TestPeersMarkdownDoesNotRequireRough(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile/address: rough\n")
	write(t, filepath.Join(dir, "peers", "kordi-pedro", "team.md"), "Pedro likes async updates.\n")
	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range issues {
		if is.File == filepath.Join("peers", "kordi-pedro", "team.md") && is.Code == "E_ROUGH" {
			t.Fatalf("peers markdown should not require rough, got %v", issues)
		}
	}
}

func TestContentFactsMustBeMarkdown(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"), "groups: {}\ncan-see: {}\n")
	write(t, filepath.Join(dir, "self", "profile", "raw.yaml"), "city: Toronto\n")
	write(t, filepath.Join(dir, "peers", "kordi-pedro", "team.yaml"), "style: async\n")
	write(t, filepath.Join(dir, "notes", "scratch.yaml"), "todo: later\n")

	issues, err := Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(issues, "E_EXT") {
		t.Fatalf("self/peers/notes YAML content should fail with E_EXT, got %v", issues)
	}
}

func TestCheckPolicy(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  freinds:\n    profile: rough\n") // typo'd group
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("expected E_POLICY for can-see group undefined in groups, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile: rough\n    profile/dietary: full\n    work: no\n")
	if issues := checkPolicy(dir); len(issues) != 0 {
		t.Errorf("valid policy should pass, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [Pedro]\ncan-see: {}\n")
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("display-name policy members should fail with E_POLICY, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  Friends: [kordi:pedro]\ncan-see: {}\n")
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("non-normalized group names should fail with E_POLICY, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends: [profile]\n")
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("legacy folder list should fail with E_POLICY, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    profile: read\n")
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("invalid disclosure level should fail with E_POLICY, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends:\n    self/profile: rough\n    ../secrets: full\n")
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("invalid policy topic should fail with E_POLICY, got %v", issues)
	}
}

func TestCheckAccess(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"), "groups:\n  friends: [kordi:pedro]\n")

	// bad level (write) and undefined group (ghost)
	write(t, filepath.Join(dir, "local", "access.yaml"),
		"grants:\n  friends:\n    ~/p: write\n  ghost:\n    ~/q: read\n")
	issues := checkAccess(dir)
	if !hasCode(issues, "E_ACCESS") {
		t.Fatalf("expected E_ACCESS, got %v", issues)
	}
	if len(issues) != 2 {
		t.Errorf("expected 2 access issues (bad level + undefined group), got %d: %v", len(issues), issues)
	}

	// valid
	write(t, filepath.Join(dir, "local", "access.yaml"),
		"grants:\n  friends:\n    ~/p: full\n")
	if issues := checkAccess(dir); len(issues) != 0 {
		t.Errorf("valid access should pass, got %v", issues)
	}

	// absent access.yaml is fine
	os.Remove(filepath.Join(dir, "local", "access.yaml"))
	if issues := checkAccess(dir); len(issues) != 0 {
		t.Errorf("absent access.yaml should be fine, got %v", issues)
	}
}

func TestFilesChecksLocalAccessConsistently(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"), "groups:\n  friends: [kordi:pedro]\n")
	write(t, filepath.Join(dir, "local", "access.yaml"),
		"grants:\n  ghost:\n    ~/p: read\n")

	issues, err := Files(dir, []string{filepath.ToSlash(filepath.Join("local", "access.yaml"))})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(issues, "E_ACCESS") {
		t.Fatalf("expected local/access.yaml E_ACCESS from Files, got %v", issues)
	}

	issues, err = Vault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(issues, "E_ACCESS") {
		t.Fatalf("expected local/access.yaml E_ACCESS from Vault, got %v", issues)
	}
}

func TestCheckLedger(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"), "groups: {}\ncan-see: {}\n")
	valid := `{"ts":"2026-07-05T12:00:00Z","to":"kordi:pedro","shared":"profile/address","level":"rough","note":"test"}` + "\n"
	write(t, filepath.Join(dir, "ledger", "device-1.log"), valid)
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if hasCode(issues, "E_LEDGER") {
		t.Fatalf("valid ledger should pass, got %v", issues)
	}

	write(t, filepath.Join(dir, "ledger", "device-1.log"),
		`{"ts":"2026-07-05T12:00:00Z","to":"kordi:pedro","shared":"profile/address"}`+"\n")
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if !hasCode(issues, "E_LEDGER") {
		t.Fatalf("ledger entry missing level should fail, got %v", issues)
	}

	write(t, filepath.Join(dir, "ledger", "device-1.log"),
		`{"ts":"2026-07-05T12:00:00Z","to":"Pedro","shared":"self/profile/address","level":"rough"}`+"\n")
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if !hasCode(issues, "E_LEDGER") {
		t.Fatalf("ledger entry with display-name recipient and self/ topic should fail, got %v", issues)
	}

	write(t, filepath.Join(dir, "ledger", "device-1.log"), "null\n")
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if !hasCode(issues, "E_LEDGER") {
		t.Fatalf("ledger entry must be a JSON object, got %v", issues)
	}

	write(t, filepath.Join(dir, "ledger", "device-1.log"), "{bad json}\n")
	issues, err := Files(dir, []string{filepath.ToSlash(filepath.Join("ledger", "device-1.log"))})
	if err != nil {
		t.Fatal(err)
	}
	if !hasCode(issues, "E_LEDGER") {
		t.Fatalf("changed ledger file should be checked, got %v", issues)
	}
}

func TestCheckDevices(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"), "groups: {}\ncan-see: {}\n")
	valid := "id: macbook-1234\nlabel: MacBook\nstatus: active\nregistered_at: \"2026-07-05T12:00:00Z\"\nunregistered_at: \"\"\n"
	write(t, filepath.Join(dir, "devices", "macbook-1234.yaml"), valid)
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if hasCode(issues, "E_DEVICE") {
		t.Fatalf("valid device should pass, got %v", issues)
	}

	write(t, filepath.Join(dir, "devices", "macbook-1234.yaml"),
		valid+"github_repo: owner/repo\ndeploy_key_id: 42\ndeploy_key_title: doss macbook-1234\ndeploy_key_fingerprint: SHA256:abc\n")
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else if hasCode(issues, "E_DEVICE") {
		t.Fatalf("valid deploy key metadata should pass, got %v", issues)
	}

	write(t, filepath.Join(dir, "devices", "macbook-1234.yaml"),
		valid+"github_repo: owner/repo\n")
	if issues, err := Files(dir, []string{filepath.ToSlash(filepath.Join("devices", "macbook-1234.yaml"))}); err != nil {
		t.Fatal(err)
	} else if !hasCode(issues, "E_DEVICE") {
		t.Fatalf("github_repo without deploy_key_id should fail, got %v", issues)
	}

	write(t, filepath.Join(dir, "devices", "macbook-1234.yaml"),
		"id: other\nlabel: MacBook\nstatus: disabled\nregistered_at: soon\n")
	if issues, err := Files(dir, []string{filepath.ToSlash(filepath.Join("devices", "macbook-1234.yaml"))}); err != nil {
		t.Fatal(err)
	} else if !hasCode(issues, "E_DEVICE") {
		t.Fatalf("invalid device should fail, got %v", issues)
	}

	write(t, filepath.Join(dir, "devices", "old.yaml"),
		"id: old\nlabel: Old Device\nstatus: unregistered\nregistered_at: \"2026-07-05T12:00:00Z\"\nunregistered_at: \"2026-07-06T12:00:00Z\"\n")
	if issues, err := Vault(dir); err != nil {
		t.Fatal(err)
	} else {
		for _, is := range issues {
			if is.File == filepath.ToSlash(filepath.Join("devices", "old.yaml")) && is.Code == "E_DEVICE" {
				t.Fatalf("unregistered device should pass, got %v", issues)
			}
		}
	}
}

func TestFrontmatterSplit(t *testing.T) {
	meta, body := Frontmatter([]byte("---\nsource: owner\n---\nhello\n"))
	if meta["source"] != "owner" {
		t.Errorf("frontmatter not parsed: %v", meta)
	}
	if body != "hello\n" {
		t.Errorf("body = %q", body)
	}
	// no frontmatter → whole thing is body
	_, body = Frontmatter([]byte("just text\n"))
	if body != "just text\n" {
		t.Errorf("body = %q", body)
	}
}
