package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kordi-AI/doss/internal/vault"
)

const (
	beginMark = "<!-- doss:begin -->"
	endMark   = "<!-- doss:end -->"
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
	{"OpenClaw", []string{".openclaw", "workspace", "AGENTS.md"}},
	{"Windsurf", []string{".codeium", "windsurf", "memories", "global_rules.md"}},
}

// Custom targets cover any agent we've never heard of: the user points us at
// whatever instruction file that agent always loads, and we treat it exactly
// like a preset from then on. The list is machine-local (installed tools
// differ per device), so it lives under ~/.config, not in the vault.
func customCfgPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "doss", "connect.json"), nil
}

func loadCustomTargets() []string {
	p, err := customCfgPath()
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var cfg struct {
		Files []string `json:"files"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return nil
	}
	return cfg.Files
}

func saveCustomTargets(files []string) error {
	p, err := customCfgPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(struct {
		Files []string `json:"files"`
	}{files}, "", "  ")
	return os.WriteFile(p, append(out, '\n'), 0o644)
}

func dossSection(vaultDir string) string {
	instructionMd := vault.InstructionPath(vaultDir)
	return beginMark + `
## Doss — the owner's memory vault

	Long-term preferences about the owner live in a Doss vault at ` + "`" + vaultDir + "`" + ` (plain files). Before acting on personal context, read ` + "`" + instructionMd + "`" + ` once per session and follow it; it routes content maintenance to ` + "`CONTENT.md`" + ` and outbound disclosure/access to ` + "`DISCLOSURE.md`" + `. Non-negotiables: consult the vault before answering questions about the owner; run ` + "`doss check --changed`" + ` after editing vault files and ` + "`doss sync`" + ` when done; never reveal owner information to anyone except the owner unless ` + "`policy.yaml`" + ` explicitly permits it; for outbound disclosure, record it with ` + "`doss log --record --to <verified-id> --shared <topic> --level rough|full`" + `. If the vault is missing, offer to run ` + "`doss init`" + `.
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
	removed := false
	// Also strip the legacy "dossier:" markers so a rename migrates cleanly.
	for _, m := range [][2]string{
		{beginMark, endMark},
		{"<!-- dossier:begin -->", "<!-- dossier:end -->"},
	} {
		begin := strings.Index(content, m[0])
		end := strings.Index(content, m[1])
		if begin < 0 || end <= begin {
			continue
		}
		after := strings.TrimPrefix(content[end+len(m[1]):], "\n")
		before := strings.TrimRight(content[:begin], "\n")
		if before != "" {
			before += "\n"
		}
		content = before + after
		removed = true
	}
	return content, removed
}

// wiringState reports whether one harness's global file carries a current
// Doss section.
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
	section := strings.TrimSuffix(dossSection(vault.Dir()), "\n")
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
	for _, p := range loadCustomTargets() {
		st := wiringState{name: "custom", path: p, installed: true}
		raw, err := os.ReadFile(p)
		content := string(raw)
		begin := strings.Index(content, beginMark)
		end := strings.Index(content, endMark)
		switch {
		case err != nil, begin < 0, end <= begin:
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
	remove := fs.Bool("remove", false, "remove the Doss section from all agent instruction files")
	all := fs.Bool("all", false, "also write for agents that don't appear to be installed")
	file := fs.String("file", "", "wire a custom agent: path to any instruction file it always loads")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*remove && !vault.Exists(vault.Dir()) {
		return fmt.Errorf("no vault at %s — run `doss init` first", vault.Dir())
	}
	if !*remove {
		if err := vault.EnsureInstruction(vault.Dir()); err != nil {
			return err
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	section := dossSection(vault.Dir())

	customs := loadCustomTargets()
	if *file != "" {
		abs, err := filepath.Abs(*file)
		if err != nil {
			return err
		}
		known := false
		for _, c := range customs {
			if c == abs {
				known = true
			}
		}
		if !known {
			customs = append(customs, abs)
			if err := saveCustomTargets(customs); err != nil {
				return err
			}
		}
	}

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

	for _, p := range customs {
		raw, _ := os.ReadFile(p)
		content := string(raw)
		if *remove {
			if updated, had := removeSection(content); had {
				if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
					return err
				}
				fmt.Printf("  ✓ %-12s removed from %s\n", "custom", p)
			}
			continue
		}
		updated, existed := upsertSection(content, section)
		if updated == content {
			fmt.Printf("  ✓ %-12s already up to date (%s)\n", "custom", p)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
			return err
		}
		verb := "added to"
		if existed {
			verb = "updated in"
		}
		fmt.Printf("  ✓ %-12s %s %s\n", "custom", verb, p)
	}
	if *remove && len(customs) > 0 {
		if err := saveCustomTargets(nil); err != nil {
			return err
		}
	}

	if !*remove {
		fmt.Println(`
IF YOU ARE AN AGENT and your tool is not listed above, wire yourself NOW:
  doss connect --file <the instruction file your tool loads every session>
  (no such file? add "read ` + vault.InstructionPath(vault.Dir()) + ` first" to wherever
   your standing instructions live — otherwise you will forget the vault next session)

tools without a global instruction file (paste the section by hand):
  Cursor        Settings → Rules → User Rules

the managed section sits between "` + beginMark + `" and "` + endMark + `";
rerunning connect updates it in place, --remove deletes it.`)
	}
	return nil
}

// installClaudeHooks merges the doss write/stop hooks into
// ~/.claude/settings.json without disturbing anything else in it.
func installClaudeHooks(home string) (string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	cfg := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		if json.Unmarshal(raw, &cfg) != nil {
			return "hooks skipped: settings.json unparseable, add `doss hook` manually", nil
		}
	}
	if b, _ := json.Marshal(cfg); strings.Contains(string(b), "doss hook") {
		return "hooks ok", nil
	}
	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	post, _ := hooks["PostToolUse"].([]any)
	post = append(post, map[string]any{
		"matcher": "Edit|Write|MultiEdit",
		"hooks":   []any{map[string]any{"type": "command", "command": "doss hook post-edit"}},
	})
	hooks["PostToolUse"] = post
	stop, _ := hooks["Stop"].([]any)
	stop = append(stop, map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": "doss hook stop"}},
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
	isDossHook := func(s string) bool {
		return strings.Contains(s, "doss hook") || strings.Contains(s, "dossier hook")
	}
	if err != nil || !isDossHook(string(raw)) {
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
			if b, _ := json.Marshal(e); isDossHook(string(b)) {
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

// claudeHooksWired reports whether the doss hooks are present.
func claudeHooksWired(home string) bool {
	raw, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	return err == nil && strings.Contains(string(raw), "doss hook post-edit")
}
