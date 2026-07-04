package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kordi-AI/dossier/internal/check"
	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

const staleAfter = 180 * 24 * time.Hour

func cmdStatus(args []string) error {
	d, err := vault.MustExist()
	if err != nil {
		return err
	}

	counts := map[string]int{}
	stale := 0
	now := time.Now()
	for _, area := range []string{"self", "peers", "notes"} {
		root := filepath.Join(d, area)
		_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
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

	issues, err := check.Vault(d)
	if err != nil {
		return err
	}

	remote := "local only"
	if gitx.HasRemote(d) {
		if out, err := gitx.Run(d, "remote", "get-url", "origin"); err == nil {
			remote = strings.TrimSpace(out)
		}
	}
	last := "never"
	if out, err := gitx.Run(d, "log", "-1", "--format=%cr"); err == nil {
		last = strings.TrimSpace(out)
	}
	dirty, _ := gitx.Dirty(d)

	fmt.Printf("vault:   %s\n", d)
	fmt.Printf("facts:   %d in self/ · %d in peers/ · %d notes\n", counts["self"], counts["peers"], counts["notes"])
	fmt.Printf("check:   %d problem(s)\n", len(issues))
	fmt.Printf("sync:    %s · last commit %s%s\n", remote, last, map[bool]string{true: " · uncommitted changes", false: ""}[dirty])
	wired, broken := 0, 0
	for _, w := range wiringStates() {
		if !w.installed {
			continue
		}
		if w.broken() {
			broken++
		} else {
			wired++
		}
	}
	wireLine := fmt.Sprintf("%d agent(s) load the rules everywhere", wired)
	if broken > 0 {
		wireLine += fmt.Sprintf(" · %d broken → run `dossier connect`", broken)
	}
	fmt.Printf("wiring:  %s\n", wireLine)
	fmt.Printf("stale:   %d file(s) untouched for 180+ days\n", stale)

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
		fmt.Printf("tidy:    not needed\n")
	}
	return nil
}
