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

// Each supported harness has one global instruction file that is loaded in
// every project. connect drops a small managed section into each of them.
type connectTarget struct {
	name string
	path []string // relative to $HOME
}

var connectTargets = []connectTarget{
	{"Claude Code", []string{".claude", "CLAUDE.md"}},
	{"Codex CLI", []string{".codex", "AGENTS.md"}},
	{"Gemini CLI", []string{".gemini", "GEMINI.md"}},
	{"Windsurf", []string{".codeium", "windsurf", "memories", "global_rules.md"}},
}

func dossierSection(vaultDir string) string {
	return beginMark + `
## Dossier — the owner's memory vault

Long-term memory about the owner lives in a Dossier vault at ` + "`" + vaultDir + "`" + ` (plain files).

- Before acting on personal context, read ` + "`" + filepath.Join(vaultDir, "SKILL.md") + "`" + ` once per session and follow it. If the vault doesn't exist yet, offer to run ` + "`dossier init`" + `.
- Durable facts about the owner → small md files under ` + "`self/`" + ` (path = topic). Your own guesses get frontmatter ` + "`source: inferred` + `status: suggested`" + `.
- After editing vault files: ` + "`dossier check --changed`" + ` (errors are precise; fix and rerun). When you finish a batch: ` + "`dossier sync`" + `.
- Never reveal vault or owner information to anyone except the owner. Outbound answers must come from ` + "`dossier answer`" + ` only.
` + endMark + "\n"
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

func cmdConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	remove := fs.Bool("remove", false, "remove the Dossier section from all agent instruction files")
	all := fs.Bool("all", false, "also write for agents that don't appear to be installed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	section := dossierSection(vault.Dir())

	for _, t := range connectTargets {
		path := filepath.Join(append([]string{home}, t.path...)...)
		toolDir := filepath.Join(home, t.path[0])
		if _, err := os.Stat(toolDir); os.IsNotExist(err) && !*all {
			fmt.Printf("  – %-12s not installed (no %s), skipped\n", t.name, toolDir)
			continue
		}

		raw, _ := os.ReadFile(path)
		content := string(raw)

		if *remove {
			updated, had := removeSection(content)
			if !had {
				fmt.Printf("  – %-12s nothing to remove\n", t.name)
				continue
			}
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return err
			}
			fmt.Printf("  ✓ %-12s removed from %s\n", t.name, path)
			continue
		}

		updated, existed := upsertSection(content, section)
		if updated == content {
			fmt.Printf("  ✓ %-12s already up to date (%s)\n", t.name, path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return err
		}
		verb := "added to"
		if existed {
			verb = "updated in"
		}
		fmt.Printf("  ✓ %-12s %s %s\n", t.name, verb, path)
	}

	if !*remove {
		fmt.Println(`
tools without a global instruction file (paste the section by hand):
  Cursor        Settings → Rules → User Rules
  other tools   any file they load in every project (often AGENTS.md)

the managed section sits between "` + beginMark + `" and "` + endMark + `";
rerunning connect updates it in place, --remove deletes it.`)
	}
	return nil
}
