package main

import (
	"strings"
	"testing"

	"github.com/Kordi-AI/doss/internal/check"
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
