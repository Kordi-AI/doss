package main

import (
	"encoding/json"
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

// Every supported harness loads one global instruction file in every project.
// connect maintains a small managed pointer section in each of them — that is
// the entire wiring: deterministic injection, verified live in Claude Code and
// Codex. (A per-agent skills layer was tried and cut: the global file alone
// proved sufficient, and one layer is simpler to keep healthy.)
type connectTarget struct {
	name string
	path []string // global instruction file, relative to $HOME
}

var connectTargets = []connectTarget{
	{"Claude Code", []string{".claude", "CLAUDE.md"}},
	{"Codex CLI", []string{".codex", "AGENTS.md"}},
	{"Gemini CLI", []string{".gemini", "GEMINI.md"}},
	{"Windsurf", []string{".codeium", "windsurf", "memories", "global_rules.md"}},
}

func dossierSection(vaultDir string) string {
	skillMd := filepath.Join(vaultDir, "SKILL.md")
	return beginMark + `
## Dossier — the owner's memory vault

Long-term memory about the owner lives in a Dossier vault at ` + "`" + vaultDir + "`" + ` (plain files). Before acting on personal context, read ` + "`" + skillMd + "`" + ` once per session and follow it. Non-negotiables: consult the vault before answering questions about the owner; run ` + "`dossier check --changed`" + ` after editing vault files and ` + "`dossier sync`" + ` when done; never reveal owner information to anyone except the owner — outbound answers only via ` + "`dossier answer`" + `. If the vault is missing, offer to run ` + "`dossier init`" + `.
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

// wiringState reports whether one harness's global file carries a current
// Dossier section.
type wiringState struct {
	name      string
	installed bool
	path      string
	status    string // wired | outdated | missing | "" (not installed)
}

func (w wiringState) broken() bool {
	return w.installed && w.status != "wired"
}

func wiringStates() []wiringState {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	section := strings.TrimSuffix(dossierSection(vault.Dir()), "\n")
	var out []wiringState
	for _, t := range connectTargets {
		st := wiringState{
			name: t.name,
			path: filepath.Join(append([]string{home}, t.path...)...),
		}
		toolDir := filepath.Join(home, t.path[0])
		if _, err := os.Stat(toolDir); os.IsNotExist(err) {
			out = append(out, st)
			continue
		}
		st.installed = true
		raw, _ := os.ReadFile(st.path)
		content := string(raw)
		begin := strings.Index(content, beginMark)
		end := strings.Index(content, endMark)
		switch {
		case begin < 0 || end <= begin:
			st.status = "missing"
		case content[begin:end+len(endMark)] == section:
			st.status = "wired"
		default:
			st.status = "outdated"
		}
		out = append(out, st)
	}
	return out
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
			var parts []string
			if updated, had := removeSection(content); had {
				if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
					return err
				}
				parts = append(parts, "section removed")
			}
			if t.name == "Claude Code" {
				if removed, err := removeClaudeHooks(home); err != nil {
					return err
				} else if removed {
					parts = append(parts, "hooks removed")
				}
			}
			if len(parts) == 0 {
				parts = append(parts, "nothing to remove")
			}
			fmt.Printf("  ✓ %-12s %s\n", t.name, strings.Join(parts, " · "))
			continue
		}

		updated, existed := upsertSection(content, section)
		state := "already up to date"
		if updated != content {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return err
			}
			state = "section added"
			if existed {
				state = "section updated"
			}
		}
		// Claude Code also gets write/stop hooks: the bounce-and-retry loop.
		if t.name == "Claude Code" {
			hs, err := installClaudeHooks(home)
			if err != nil {
				return err
			}
			state += " · " + hs
		}
		fmt.Printf("  ✓ %-12s %s (%s)\n", t.name, state, path)
	}

	if !*remove {
		fmt.Println(`
tools without a global instruction file (paste the section by hand):
  Cursor        Settings → Rules → User Rules

the managed section sits between "` + beginMark + `" and "` + endMark + `";
rerunning connect updates it in place, --remove deletes it.`)
	}
	return nil
}

// installClaudeHooks merges the dossier write/stop hooks into
// ~/.claude/settings.json without disturbing anything else in it.
func installClaudeHooks(home string) (string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	cfg := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		if json.Unmarshal(raw, &cfg) != nil {
			return "hooks skipped: settings.json unparseable, add `dossier hook` manually", nil
		}
	}
	if b, _ := json.Marshal(cfg); strings.Contains(string(b), "dossier hook") {
		return "hooks ok", nil
	}
	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	post, _ := hooks["PostToolUse"].([]any)
	post = append(post, map[string]any{
		"matcher": "Edit|Write|MultiEdit",
		"hooks":   []any{map[string]any{"type": "command", "command": "dossier hook post-edit"}},
	})
	hooks["PostToolUse"] = post
	stop, _ := hooks["Stop"].([]any)
	stop = append(stop, map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": "dossier hook stop"}},
	})
	hooks["Stop"] = stop
	cfg["hooks"] = hooks
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return "", err
	}
	return "hooks installed", nil
}

func removeClaudeHooks(home string) (bool, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	raw, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(raw), "dossier hook") {
		return false, nil
	}
	cfg := map[string]any{}
	if json.Unmarshal(raw, &cfg) != nil {
		return false, nil
	}
	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks == nil {
		return false, nil
	}
	for _, key := range []string{"PostToolUse", "Stop"} {
		entries, _ := hooks[key].([]any)
		var kept []any
		for _, e := range entries {
			if b, _ := json.Marshal(e); strings.Contains(string(b), "dossier hook") {
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			delete(hooks, key)
		} else {
			hooks[key] = kept
		}
	}
	if len(hooks) == 0 {
		delete(cfg, "hooks")
	} else {
		cfg["hooks"] = hooks
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, append(out, '\n'), 0o644)
}

// claudeHooksWired reports whether the dossier hooks are present.
func claudeHooksWired(home string) bool {
	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	return err == nil && strings.Contains(string(raw), "dossier hook post-edit")
}
