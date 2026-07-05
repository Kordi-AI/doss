package main

import (
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
