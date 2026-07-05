package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

// cmdHook is the endpoint harness hooks call. It must be fast and silent
// for anything that doesn't concern the vault: exit 0 costs nothing.
// Exit code 2 is the harness convention for "feed stderr back to the model".
func cmdHook(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: doss hook <post-edit|stop>")
	}
	switch args[0] {
	case "post-edit":
		return hookPostEdit()
	case "stop":
		return hookStop()
	default:
		return fmt.Errorf("unknown hook %q (want post-edit or stop)", args[0])
	}
}

func hookPostEdit() error {
	d := vault.Dir()
	if !vault.Exists(d) {
		return nil
	}
	var payload struct {
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		return nil
	}
	if json.Unmarshal(raw, &payload) != nil || payload.ToolInput.FilePath == "" {
		return nil
	}
	fp, err := filepath.Abs(payload.ToolInput.FilePath)
	if err != nil {
		return nil
	}
	da, _ := filepath.Abs(d)
	if !strings.HasPrefix(fp, da+string(filepath.Separator)) {
		return nil // not a vault file — none of our business
	}
	rel, err := filepath.Rel(da, fp)
	if err != nil {
		return nil
	}
	issues, err := check.Files(d, []string{filepath.ToSlash(rel)})
	if err != nil || len(issues) == 0 {
		return nil
	}
	for _, is := range issues {
		fmt.Fprintln(os.Stderr, is)
	}
	fmt.Fprintln(os.Stderr, "this write did not pass doss check — fix the file now (errors above are precise), then it counts")
	os.Exit(2)
	return nil
}

func hookStop() error {
	d := vault.Dir()
	if !vault.Exists(d) {
		return nil
	}
	// Session end is the least-noisy moment to surface maintenance, so the
	// tidy nudge piggybacks here (once per session) even when nothing changed.
	defer func() {
		if n := gatherDirt(d, nil); n.due() {
			fmt.Fprintln(os.Stderr, n.nudge())
		}
	}()

	dirty, err := gitx.Dirty(d)
	if err != nil || !dirty {
		return nil
	}
	issues, err := check.Vault(d)
	if err != nil {
		return nil
	}
	if len(issues) > 0 {
		for _, is := range issues {
			fmt.Fprintln(os.Stderr, is)
		}
		fmt.Fprintf(os.Stderr, "the vault has %d unresolved problem(s); fix them and run `doss sync` before finishing\n", len(issues))
		os.Exit(2)
	}
	// Validated and dirty: commit. Network problems must never trap the user.
	if _, err := gitx.Run(d, "add", "-A"); err != nil {
		return nil
	}
	if _, err := gitx.Run(d, "commit", "-m", "doss sync (session end)"); err != nil {
		return nil
	}
	if gitx.HasRemote(d) {
		pullArgs := []string{"pull", "--rebase"}
		pushArgs := []string{"push"}
		if gitx.Upstream(d) == "" {
			branch := gitx.CurrentBranch(d)
			pullArgs = append(pullArgs, "origin", branch)
			pushArgs = append(pushArgs, "-u", "origin", branch)
		}
		if out, err := gitx.Run(d, pullArgs...); err != nil {
			if !(gitx.Upstream(d) == "" && strings.Contains(out, "couldn't find remote ref")) {
				_, _ = gitx.Run(d, "rebase", "--abort")
				fmt.Fprintln(os.Stderr, "doss: committed locally; pull hit a conflict — run `doss sync` by hand later")
				return nil
			}
		}
		if out, err := gitx.Run(d, pushArgs...); err != nil {
			fmt.Fprintln(os.Stderr, "doss: committed locally; push failed — run `doss sync` later")
			if msg := strings.TrimSpace(out); msg != "" {
				fmt.Fprintln(os.Stderr, "git said: "+msg)
			}
		}
	}
	return nil
}
