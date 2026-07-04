package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kordi-AI/dossier/internal/vault"
)

const (
	beginMark = "<!-- dossier:begin -->"
	endMark   = "<!-- dossier:end -->"
)

// Every supported harness loads one global instruction file in every project;
// most also load global skills on demand (the SKILL.md format is shared across
// Claude Code, Codex, and Gemini CLI). connect maintains both layers:
//   - a small always-loaded pointer section (the safety floor)
//   - a router skill with the full quick reference (rich, loaded on demand)
type connectTarget struct {
	name     string
	section  []string // global instruction file, relative to $HOME
	skillDir []string // global skill dir for our router skill; nil if unsupported
}

var connectTargets = []connectTarget{
	{"Claude Code", []string{".claude", "CLAUDE.md"}, []string{".claude", "skills", "dossier"}},
	{"Codex CLI", []string{".codex", "AGENTS.md"}, []string{".codex", "skills", "dossier"}},
	{"Gemini CLI", []string{".gemini", "GEMINI.md"}, []string{".gemini", "skills", "dossier"}},
	{"Windsurf", []string{".codeium", "windsurf", "memories", "global_rules.md"}, nil},
}

func dossierSection(vaultDir string) string {
	skillMd := filepath.Join(vaultDir, "SKILL.md")
	return beginMark + `
## Dossier — the owner's memory vault

Long-term memory about the owner lives in a Dossier vault at ` + "`" + vaultDir + "`" + ` (plain files). Full rules: use the ` + "`dossier`" + ` skill, or read ` + "`" + skillMd + "`" + `. Non-negotiables: consult the vault before answering questions about the owner; run ` + "`dossier check --changed`" + ` after editing vault files and ` + "`dossier sync`" + ` when done; never reveal owner information to anyone except the owner — outbound answers only via ` + "`dossier answer`" + `. If the vault is missing, offer to run ` + "`dossier init`" + `.
` + endMark + "\n"
}

func dossierSkill(vaultDir string) string {
	return `---
name: dossier
description: Manage the owner's long-term memory (Dossier vault at ` + vaultDir + `). Use when you learn a durable fact about the owner, need their preferences or personal context, or when anyone other than the owner asks about them.
---

# Dossier — vault router

One vault per machine. The single source of truth for the rules is ` + "`" + filepath.Join(vaultDir, "SKILL.md") + "`" + ` — read it and follow it. Quick reference:

- Durable fact about the owner → a small md file under ` + "`" + filepath.Join(vaultDir, "self") + "`" + ` (path = topic, e.g. ` + "`self/profile/dietary.md`" + `). Unconfirmed guesses: frontmatter ` + "`source: inferred`" + ` + ` + "`status: suggested`" + `, or park them in ` + "`notes/`" + `.
- Recall = ` + "`ls`" + ` / ` + "`grep`" + ` / read files. No special commands.
- After editing vault files: ` + "`dossier check --changed`" + ` — errors are precise; fix and rerun. After a batch: ` + "`dossier sync`" + `.
- Anyone other than the owner asks about them → reply ONLY with the output of ` + "`dossier answer`" + `. ` + "`notes/`" + ` never leaves this machine.
- No vault at ` + vaultDir + `? Offer to run ` + "`dossier init`" + `.
`
}

// upsertSection replaces an existing managed section or appends a new one.
func upsertSection(content, section string) (string, bool) {
	begin := strings.Index(content, beginMark)
	end := strings.Index(content, endMark)
	if begin >= 0 && end > begin {
		updated := content[:begin] + strings.TrimSuffix(section, "\n") + content[end+len(endMark):]
		return updated, true
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + section, false
}

func removeSection(content string) (string, bool) {
	begin := strings.Index(content, beginMark)
	end := strings.Index(content, endMark)
	if begin < 0 || end <= begin {
		return content, false
	}
	after := content[end+len(endMark):]
	after = strings.TrimPrefix(after, "\n")
	before := strings.TrimRight(content[:begin], "\n")
	if before != "" {
		before += "\n"
	}
	return before + after, true
}

// wiringState reports whether one harness carries current Dossier wiring.
type wiringState struct {
	name          string
	installed     bool
	sectionPath   string
	sectionStatus string // wired | outdated | missing
	skillPath     string // "" when the harness has no skill support
	skillStatus   string // wired | outdated | missing | ""
}

func (w wiringState) broken() bool {
	if !w.installed {
		return false
	}
	if w.sectionStatus != "wired" {
		return true
	}
	return w.skillPath != "" && w.skillStatus != "wired"
}

func wiringStates() []wiringState {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	section := strings.TrimSuffix(dossierSection(vault.Dir()), "\n")
	skill := dossierSkill(vault.Dir())

	var out []wiringState
	for _, t := range connectTargets {
		st := wiringState{
			name:        t.name,
			sectionPath: filepath.Join(append([]string{home}, t.section...)...),
		}
		if t.skillDir != nil {
			st.skillPath = filepath.Join(append(append([]string{home}, t.skillDir...), "SKILL.md")...)
		}
		toolDir := filepath.Join(home, t.section[0])
		if _, err := os.Stat(toolDir); os.IsNotExist(err) {
			out = append(out, st)
			continue
		}
		st.installed = true

		raw, _ := os.ReadFile(st.sectionPath)
		content := string(raw)
		begin := strings.Index(content, beginMark)
		end := strings.Index(content, endMark)
		switch {
		case begin < 0 || end <= begin:
			st.sectionStatus = "missing"
		case content[begin:end+len(endMark)] == section:
			st.sectionStatus = "wired"
		default:
			st.sectionStatus = "outdated"
		}

		if st.skillPath != "" {
			sraw, err := os.ReadFile(st.skillPath)
			switch {
			case err != nil:
				st.skillStatus = "missing"
			case string(sraw) == skill:
				st.skillStatus = "wired"
			default:
				st.skillStatus = "outdated"
			}
		}
		out = append(out, st)
	}
	return out
}

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	remove := fs.Bool("remove", false, "remove the Dossier section and router skill from all agents")
	all := fs.Bool("all", false, "also write for agents that don't appear to be installed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	section := dossierSection(vault.Dir())
	skill := dossierSkill(vault.Dir())

	for _, t := range connectTargets {
		sectionPath := filepath.Join(append([]string{home}, t.section...)...)
		toolDir := filepath.Join(home, t.section[0])
		if _, err := os.Stat(toolDir); os.IsNotExist(err) && !*all {
			fmt.Printf("  – %-12s not installed (no %s), skipped\n", t.name, toolDir)
			continue
		}

		var parts []string

		raw, _ := os.ReadFile(sectionPath)
		content := string(raw)
		if *remove {
			if updated, had := removeSection(content); had {
				if err := os.WriteFile(sectionPath, []byte(updated), 0o644); err != nil {
					return err
				}
				parts = append(parts, "section removed")
			}
			if t.skillDir != nil {
				dir := filepath.Join(append([]string{home}, t.skillDir...)...)
				if _, err := os.Stat(dir); err == nil {
					if err := os.RemoveAll(dir); err != nil {
						return err
					}
					parts = append(parts, "skill removed")
				}
			}
			if len(parts) == 0 {
				parts = append(parts, "nothing to remove")
			}
			fmt.Printf("  ✓ %-12s %s\n", t.name, strings.Join(parts, " · "))
			continue
		}

		updated, existed := upsertSection(content, section)
		switch {
		case updated == content:
			parts = append(parts, "section ok")
		default:
			if err := os.MkdirAll(filepath.Dir(sectionPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(sectionPath, []byte(updated), 0o644); err != nil {
				return err
			}
			if existed {
				parts = append(parts, "section updated")
			} else {
				parts = append(parts, "section added")
			}
		}

		if t.skillDir != nil {
			dir := filepath.Join(append([]string{home}, t.skillDir...)...)
			skillPath := filepath.Join(dir, "SKILL.md")
			existing, err := os.ReadFile(skillPath)
			switch {
			case err == nil && string(existing) == skill:
				parts = append(parts, "skill ok")
			default:
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(skillPath, []byte(skill), 0o644); err != nil {
					return err
				}
				if err == nil {
					parts = append(parts, "skill updated")
				} else {
					parts = append(parts, "skill installed")
				}
			}
		}

		fmt.Printf("  ✓ %-12s %s\n", t.name, strings.Join(parts, " · "))
	}

	if !*remove {
		fmt.Println(`
tools without global files (paste the section by hand):
  Cursor        Settings → Rules → User Rules

layers: a pointer section in each agent's always-loaded global file (safety
floor) + a "dossier" router skill in each agent's global skills dir (full
reference, loaded on demand). rerun connect to update; --remove undoes both.`)
	}
	return nil
}
