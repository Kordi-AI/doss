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
	beginMark             = "<!-- doss:begin -->"
	endMark               = "<!-- doss:end -->"
	claudePostEditCommand = "doss hook post-edit"
	claudeStopCommand     = "doss hook stop"
)

type markerPair struct {
	begin string
	end   string
}

var managedMarkerPairs = []markerPair{
	{beginMark, endMark},
	{"<!-- dossier:begin -->", "<!-- dossier:end -->"},
}

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

// upsertSection replaces existing managed sections or appends a new one. It
// also migrates the old dossier markers and collapses accidental duplicates.
func upsertSection(content, section string) (string, bool) {
	cleaned, insertAt, existed := stripManagedSections(content)
	if existed {
		return insertSectionAt(cleaned, section, insertAt), true
	}
	return insertSectionAt(content, section, len(content)), false
}

func removeSection(content string) (string, bool) {
	cleaned, _, removed := stripManagedSections(content)
	return cleaned, removed
}

func stripManagedSections(content string) (string, int, bool) {
	insertAt := -1
	removed := false
	for {
		begin, end, ok := nextManagedSection(content)
		if !ok {
			break
		}
		if insertAt < 0 {
			insertAt = begin
		}
		before := strings.TrimRight(content[:begin], "\n")
		after := strings.TrimPrefix(content[end:], "\n")
		if before != "" && after != "" {
			content = before + "\n" + after
		} else {
			content = before + after
		}
		if insertAt > len(content) {
			insertAt = len(content)
		}
		removed = true
	}
	return content, insertAt, removed
}

func nextManagedSection(content string) (int, int, bool) {
	bestBegin := -1
	bestEnd := -1
	for _, m := range managedMarkerPairs {
		begin := strings.Index(content, m.begin)
		if begin < 0 {
			continue
		}
		afterBegin := begin + len(m.begin)
		relEnd := strings.Index(content[afterBegin:], m.end)
		if relEnd < 0 {
			continue
		}
		end := afterBegin + relEnd + len(m.end)
		if bestBegin < 0 || begin < bestBegin {
			bestBegin = begin
			bestEnd = end
		}
	}
	if bestBegin < 0 {
		return 0, 0, false
	}
	return bestBegin, bestEnd, true
}

func insertSectionAt(content, section string, at int) string {
	section = strings.TrimSuffix(section, "\n")
	if at < 0 || at > len(content) {
		at = len(content)
	}
	before := strings.TrimRight(content[:at], "\n")
	after := strings.TrimPrefix(content[at:], "\n")
	switch {
	case before != "" && after != "":
		return before + "\n\n" + section + "\n\n" + after
	case before != "":
		return before + "\n\n" + section + "\n"
	case after != "":
		return section + "\n\n" + after
	default:
		return section + "\n"
	}
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
	if *remove && *file != "" {
		return fmt.Errorf("--file cannot be combined with --remove; saved custom targets are removed automatically")
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
		abs, err := validateCustomTarget(*file, vault.Dir())
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

func validateCustomTarget(p, vaultDir string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err == nil && st.IsDir() {
		return "", fmt.Errorf("--file must be an agent instruction file, got directory: %s", abs)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	vd, err := filepath.Abs(vaultDir)
	if err == nil && sameOrInside(abs, vd) {
		return "", fmt.Errorf("--file must point to an agent's always-loaded instruction file, not a file inside the vault")
	}
	return abs, nil
}

func sameOrInside(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
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
	hooks, _ := cfg["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	changed := false
	if ensureClaudeHook(hooks, "PostToolUse", claudePostEditCommand, map[string]any{
		"matcher": "Edit|Write|MultiEdit",
		"hooks":   []any{map[string]any{"type": "command", "command": claudePostEditCommand}},
	}) {
		changed = true
	}
	if ensureClaudeHook(hooks, "Stop", claudeStopCommand, map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": claudeStopCommand}},
	}) {
		changed = true
	}
	if !changed {
		return "hooks ok", nil
	}
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
	return "hooks installed/repaired", nil
}

func ensureClaudeHook(hooks map[string]any, key, command string, entry map[string]any) bool {
	items, _ := hooks[key].([]any)
	changed := false
	hasCommand := false
	var kept []any
	for _, item := range items {
		b, _ := json.Marshal(item)
		s := string(b)
		switch {
		case strings.Contains(s, command):
			hasCommand = true
			kept = append(kept, item)
		case isDossHookJSON(s):
			changed = true
		default:
			kept = append(kept, item)
		}
	}
	if !hasCommand {
		kept = append(kept, entry)
		changed = true
	}
	hooks[key] = kept
	return changed
}

func hookListHasCommand(v any, command string) bool {
	items, _ := v.([]any)
	for _, item := range items {
		b, _ := json.Marshal(item)
		if strings.Contains(string(b), command) {
			return true
		}
	}
	return false
}

func isDossHookJSON(s string) bool {
	return strings.Contains(s, "doss hook") || strings.Contains(s, "dossier hook")
}

func removeClaudeHooks(home string) (bool, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	raw, err := os.ReadFile(path)
	if err != nil || !isDossHookJSON(string(raw)) {
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
			if b, _ := json.Marshal(e); isDossHookJSON(string(b)) {
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
	if err != nil {
		return false
	}
	cfg := map[string]any{}
	if json.Unmarshal(raw, &cfg) != nil {
		return strings.Contains(string(raw), claudePostEditCommand) && strings.Contains(string(raw), claudeStopCommand)
	}
	hooks, _ := cfg["hooks"].(map[string]any)
	return hookListHasCommand(hooks["PostToolUse"], claudePostEditCommand) &&
		hookListHasCommand(hooks["Stop"], claudeStopCommand)
}
