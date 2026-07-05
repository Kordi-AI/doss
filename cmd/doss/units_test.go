package main

import "testing"

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
