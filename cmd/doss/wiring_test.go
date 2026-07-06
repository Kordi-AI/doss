package main

import (
	"os"
	"strings"
	"testing"
)

func TestUpsertSection(t *testing.T) {
	sec := "<!-- doss:begin -->\nHELLO\n<!-- doss:end -->\n"

	out, existed := upsertSection("", sec)
	if existed {
		t.Error("empty content should not report existed")
	}
	if !strings.Contains(out, "HELLO") {
		t.Error("section not added")
	}

	out2, existed2 := upsertSection(out, "<!-- doss:begin -->\nWORLD\n<!-- doss:end -->\n")
	if !existed2 {
		t.Error("should report existed on update")
	}
	if strings.Contains(out2, "HELLO") || !strings.Contains(out2, "WORLD") {
		t.Errorf("did not replace in place: %q", out2)
	}

	out3, _ := upsertSection("# my notes\n", sec)
	if !strings.HasPrefix(out3, "# my notes") {
		t.Errorf("clobbered user content: %q", out3)
	}
}

func TestUpsertSectionMigratesLegacyAndDedupes(t *testing.T) {
	sec := "<!-- doss:begin -->\nCURRENT\n<!-- doss:end -->\n"
	content := strings.Join([]string{
		"# mine",
		"",
		"<!-- dossier:begin -->",
		"legacy",
		"<!-- dossier:end -->",
		"",
		"middle",
		"",
		"<!-- doss:begin -->",
		"duplicate",
		"<!-- doss:end -->",
		"",
		"tail",
	}, "\n")

	out, existed := upsertSection(content, sec)
	if !existed {
		t.Fatal("legacy managed section should count as existing")
	}
	for _, stale := range []string{"dossier:begin", "legacy", "duplicate"} {
		if strings.Contains(out, stale) {
			t.Fatalf("stale managed content remains: %q\n%s", stale, out)
		}
	}
	if strings.Count(out, beginMark) != 1 || !strings.Contains(out, "CURRENT") {
		t.Fatalf("managed section should be current and unique:\n%s", out)
	}
	if !strings.Contains(out, "# mine") || !strings.Contains(out, "middle") || !strings.Contains(out, "tail") {
		t.Fatalf("user content lost while upserting:\n%s", out)
	}
}

func TestRemoveSectionLegacy(t *testing.T) {
	// legacy dossier: markers must migrate cleanly too
	content := "# mine\n<!-- dossier:begin -->\nold\n<!-- dossier:end -->\n"
	out, removed := removeSection(content)
	if !removed {
		t.Fatal("legacy markers not removed")
	}
	if strings.Contains(out, "dossier:begin") {
		t.Errorf("legacy section remains: %q", out)
	}
	if !strings.Contains(out, "# mine") {
		t.Errorf("user content lost: %q", out)
	}

	if _, removed := removeSection("nothing here\n"); removed {
		t.Error("removeSection reported removal when there was none")
	}
}

func TestClaudeHooksRequireAndRepairBothHooks(t *testing.T) {
	home := t.TempDir()
	settings := home + "/.claude/settings.json"
	if err := os.MkdirAll(home+"/.claude", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [{"type": "command", "command": "doss hook post-edit"}]
      }
    ]
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if claudeHooksWired(home) {
		t.Fatal("Claude hooks should not be wired when the stop hook is missing")
	}
	if _, err := installClaudeHooks(home); err != nil {
		t.Fatal(err)
	}
	if !claudeHooksWired(home) {
		raw, _ := os.ReadFile(settings)
		t.Fatalf("installClaudeHooks should repair the missing stop hook:\n%s", raw)
	}
	raw, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if strings.Count(content, "doss hook post-edit") != 1 || strings.Count(content, "doss hook stop") != 1 {
		t.Fatalf("hooks should be present once each:\n%s", content)
	}
}
