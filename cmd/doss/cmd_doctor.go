package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

const staleAfter = 180 * 24 * time.Hour

// cmdDoctor is the one health command: it reports the vault (facts, freshness,
// sync, tidy) and the plumbing (install, agent wiring, hooks), and with --fix
// repairs the wiring. `doss status` is an alias for it.
func cmdDoctor(args []string) error {
	fset := flag.NewFlagSet("doctor", flag.ExitOnError)
	fix := fset.Bool("fix", false, "repair wiring problems by running doss connect")
	if err := fset.Parse(args); err != nil {
		return err
	}

	var problems []string

	exe, _ := os.Executable()
	fmt.Printf("binary:  doss %s (%s)\n", version, exe)

	vd := vault.Dir()
	if !vault.Exists(vd) {
		fmt.Printf("vault:   %s — missing\n", vd)
		problems = append(problems, "no vault — run `doss init`")
		printWiring(&problems, fix)
		return finish(problems)
	}

	// Vault stats.
	counts := map[string]int{}
	stale := 0
	now := time.Now()
	for _, area := range []string{"self", "peers", "notes"} {
		_ = filepath.WalkDir(filepath.Join(vd, area), func(_ string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				return nil
			}
			counts[area]++
			if area != "notes" {
				if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > staleAfter {
					stale++
				}
			}
			return nil
		})
	}

	issues, err := check.Vault(vd)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		problems = append(problems, fmt.Sprintf("%d check problem(s) — run `doss check`", len(issues)))
	}

	remote := "local only — no cloud copy"
	if gitx.HasRemote(vd) {
		if out, err := gitx.Run(vd, "remote", "get-url", "origin"); err == nil {
			remote = strings.TrimSpace(out)
		}
	}
	last := "never"
	if out, err := gitx.Run(vd, "log", "-1", "--format=%cr"); err == nil {
		last = strings.TrimSpace(out)
	}
	dirty, _ := gitx.Dirty(vd)
	dirtyNote := ""
	if dirty {
		dirtyNote = " · uncommitted changes"
	}

	fmt.Printf("vault:   %s\n", vd)
	fmt.Printf("facts:   %d in self/ · %d in peers/ · %d notes\n", counts["self"], counts["peers"], counts["notes"])
	fmt.Printf("check:   %d problem(s)\n", len(issues))
	fmt.Printf("sync:    %s · last commit %s%s\n", remote, last, dirtyNote)

	printWiring(&problems, fix)

	// Tidy hints.
	var due []string
	if len(issues) > 0 {
		due = append(due, fmt.Sprintf("fix %d check problem(s)", len(issues)))
	}
	if stale >= 10 {
		due = append(due, fmt.Sprintf("review %d stale file(s)", stale))
	}
	if counts["notes"] >= 50 {
		due = append(due, fmt.Sprintf("triage %d notes", counts["notes"]))
	}
	if len(due) > 0 {
		fmt.Printf("tidy:    due — %s\n", strings.Join(due, "; "))
	} else {
		fmt.Printf("tidy:    not needed (%d stale)\n", stale)
	}

	return finish(problems)
}

func printWiring(problems *[]string, fix *bool) {
	fmt.Println("wiring:  (does each agent load the rules in every project?)")
	broken := false
	for _, w := range wiringStates() {
		if !w.installed {
			fmt.Printf("    – %-12s not installed\n", w.name)
			continue
		}
		mark := "✓"
		if w.broken() {
			mark = "✗"
			broken = true
		}
		fmt.Printf("    %s %-12s %-9s %s\n", mark, w.name, w.status, w.path)
	}
	fmt.Println("    – Cursor       manual: paste the section into Settings → Rules → User Rules")

	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(home + "/.claude"); err == nil {
			if claudeHooksWired(home) {
				fmt.Println("hooks:   ✓ Claude Code write/stop hooks wired")
			} else {
				fmt.Println("hooks:   ✗ Claude Code hooks missing")
				broken = true
			}
		}
	}

	if broken && *fix {
		fmt.Println("\nrepairing wiring:")
		if err := cmdConnect(nil); err == nil {
			broken = false
			for _, w := range wiringStates() {
				if w.broken() {
					broken = true
				}
			}
		}
	}
	if broken {
		*problems = append(*problems, "agent wiring incomplete — run `doss connect` (or `doss doctor --fix`)")
	}
}

func finish(problems []string) error {
	if len(problems) == 0 {
		fmt.Println("\n✓ everything looks good")
		return nil
	}
	fmt.Println()
	for _, p := range problems {
		fmt.Println("  ! " + p)
	}
	return fmt.Errorf("%d problem(s)", len(problems))
}
