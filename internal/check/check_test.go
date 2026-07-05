package check

import (
	"os"
	"path/filepath"
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
			issues := checkFrontmatter("self/x.md", []byte(c.fm))
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
	write(t, filepath.Join(dir, "self", "profile", "dietary.md"), "- peanuts\n")
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends: [profile]\n")

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

func TestCheckPolicy(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  freinds: [profile]\n") // typo'd group
	if issues := checkPolicy(dir); !hasCode(issues, "E_POLICY") {
		t.Errorf("expected E_POLICY for can-see group undefined in groups, got %v", issues)
	}

	write(t, filepath.Join(dir, "policy.yaml"),
		"groups:\n  friends: [kordi:pedro]\ncan-see:\n  friends: [profile]\n")
	if issues := checkPolicy(dir); len(issues) != 0 {
		t.Errorf("valid policy should pass, got %v", issues)
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
