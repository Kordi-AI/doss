package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/vault"
)

func TestTopicOf(t *testing.T) {
	cases := map[string]string{
		"self/profile/dietary.md": "profile.dietary",
		"self/work/skills.md":     "work.skills",
		"peers/kordi-pedro/x.md":  "peers.kordi-pedro.x",
	}
	for in, want := range cases {
		if got := topicOf(in); got != want {
			t.Errorf("topicOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDirtDue(t *testing.T) {
	if (dirt{}).due() {
		t.Error("empty dirt should not be due")
	}
	if !(dirt{checkIssues: 1}).due() {
		t.Error("a check problem should be due")
	}
	if !(dirt{suggested: make([]string, 5)}).due() {
		t.Error("5 unconfirmed should be due")
	}
	if (dirt{suggested: make([]string, 4)}).due() {
		t.Error("4 unconfirmed should not be due")
	}
	if !(dirt{notes: 50}).due() {
		t.Error("50 notes should be due")
	}
	// staleness alone must NOT trigger the alarm (design decision)
	if (dirt{stale: make([]string, 100)}).due() {
		t.Error("staleness alone must not be due")
	}
}

func TestMissingRoughNudge(t *testing.T) {
	issues := []check.Issue{
		{File: "self/profile/address.md", Code: "E_ROUGH"},
		{File: "self/profile/address.md", Code: "E_ROUGH"},
		{File: "self/work/style.md", Code: "E_ROUGH"},
		{File: "self/profile/bad.md", Code: "E_EMPTY"},
	}
	missing := missingRoughFiles(issues)
	if len(missing) != 2 {
		t.Fatalf("missingRoughFiles should dedupe rough issues, got %v", missing)
	}
	nudge := (dirt{checkIssues: len(issues), missingRough: missing}).nudge()
	if !strings.Contains(nudge, "rough-shared fact(s) need rough values") {
		t.Fatalf("nudge should call out missing rough values, got %q", nudge)
	}
	if others := nonRoughIssues(issues); len(others) != 1 || others[0].Code != "E_EMPTY" {
		t.Fatalf("nonRoughIssues should keep only non-rough issues, got %v", others)
	}
}

func TestTokenCloneURL(t *testing.T) {
	if got := tokenCloneURL("owner/repo", "TKN"); got != "https://TKN@github.com/owner/repo.git" {
		t.Errorf("shorthand: got %q", got)
	}
	if got := tokenCloneURL("https://github.com/o/r.git", "T"); got != "https://T@github.com/o/r.git" {
		t.Errorf("full url: got %q", got)
	}
}

func TestSanitizeToken(t *testing.T) {
	if got := sanitizeToken("boom SECRET boom", "SECRET"); got != "boom *** boom" {
		t.Errorf("token not redacted: %q", got)
	}
	if got := sanitizeToken("no token here", ""); got != "no token here" {
		t.Errorf("empty token changed string: %q", got)
	}
}

func TestGitHubRepoFromRef(t *testing.T) {
	cases := map[string]string{
		"owner/repo":                                  "owner/repo",
		"https://github.com/owner/repo.git":           "owner/repo",
		"https://TOKEN@github.com/owner/repo.git":     "owner/repo",
		"git@github.com:owner/repo.git":               "owner/repo",
		"ssh://git@github.com/owner/repo.git":         "owner/repo",
		"https://example.com/owner/repo.git":          "",
		"/tmp/local.git":                              "",
		"https://github.com/owner/repo/extra/path":    "owner/repo",
		"https://github.com/Owner-1/repo.name.git":    "Owner-1/repo.name",
		"https://github.com/owner/repo.with-dash.git": "owner/repo.with-dash",
	}
	for in, want := range cases {
		got, ok := githubRepoFromRef(in)
		if want == "" {
			if ok {
				t.Fatalf("githubRepoFromRef(%q) = %q, want no match", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Fatalf("githubRepoFromRef(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestGitHubDeployKeyAPI(t *testing.T) {
	var sawCreate, sawDelete bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer TOKEN" {
			t.Fatalf("Authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/keys":
			sawCreate = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["title"] != "doss device" || payload["key"] != "ssh-ed25519 AAA" || payload["read_only"] != false {
				t.Fatalf("bad create payload: %#v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":42}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/owner/repo/keys/42":
			sawDelete = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	old := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = old }()

	id, err := githubAddDeployKey("TOKEN", "owner/repo", "doss device", "ssh-ed25519 AAA")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 || !sawCreate {
		t.Fatalf("githubAddDeployKey id=%d sawCreate=%v", id, sawCreate)
	}
	if err := githubDeleteDeployKey("TOKEN", "owner/repo", 42); err != nil {
		t.Fatal(err)
	}
	if !sawDelete {
		t.Fatal("delete endpoint was not called")
	}
}

func TestEnsureGitHubDeviceKeyReusesExistingRecordWithoutAuth(t *testing.T) {
	dir := initTestVault(t)
	dev, err := vault.RegisterDevice(dir)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "config", "--local", "doss.githubRepo", "owner/repo")

	keyPath := devicePrivateKeyPath(dir, dev.ID)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.SetDeviceDeployKey(dir, dev.ID, "owner/repo", "doss "+dev.ID, "SHA256:abc", 42); err != nil {
		t.Fatal(err)
	}

	calledAuth := false
	oldAuth := githubAuthToken
	githubAuthToken = func() (string, error) {
		calledAuth = true
		return "", errors.New("unexpected auth lookup")
	}
	defer func() { githubAuthToken = oldAuth }()

	if err := ensureGitHubDeviceKey(dir, ""); err != nil {
		t.Fatal(err)
	}
	if calledAuth {
		t.Fatal("existing deploy key record should not require GitHub API auth")
	}
	cmd := exec.Command("git", "-C", dir, "config", "--local", "--get", "core.sshCommand")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("core.sshCommand missing: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), keyPath) {
		t.Fatalf("core.sshCommand should use device key %s, got %s", keyPath, out)
	}
}

func TestEnsureGitHubDeviceKeyMigratesLegacyTokenOrigin(t *testing.T) {
	dir := initTestVault(t)
	dev, err := vault.RegisterDevice(dir)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "remote", "add", "origin", "https://LEGACY@github.com/owner/repo.git")

	var sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer LEGACY" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/repos/owner/repo/keys" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		sawCreate = true
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(payload["key"].(string), "ssh-ed25519 ") || payload["read_only"] != false {
			t.Fatalf("bad create payload: %#v", payload)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":101}`))
	}))
	defer srv.Close()
	oldAPI := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = oldAPI }()

	calledAuth := false
	oldAuth := githubAuthToken
	githubAuthToken = func() (string, error) {
		calledAuth = true
		return "", errors.New("unexpected auth lookup")
	}
	defer func() { githubAuthToken = oldAuth }()

	if err := ensureGitHubDeviceKey(dir, ""); err != nil {
		t.Fatal(err)
	}
	if calledAuth {
		t.Fatal("legacy token remote should be used before gh auth")
	}
	if !sawCreate {
		t.Fatal("deploy key create endpoint was not called")
	}
	record, err := vault.DeviceRecord(dir, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.GitHubRepo != "owner/repo" || record.DeployKeyID != 101 {
		t.Fatalf("deploy key metadata not recorded: %+v", record)
	}
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("reading origin failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "git@github.com:owner/repo.git" {
		t.Fatalf("origin should migrate to deploy-key SSH URL, got %s", out)
	}
}
