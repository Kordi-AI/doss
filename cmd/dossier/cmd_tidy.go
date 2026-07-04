package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kordi-AI/dossier/internal/check"
	"github.com/Kordi-AI/dossier/internal/vault"
)

// dirt is everything machines can flag but only judgment can resolve.
type dirt struct {
	checkIssues int
	stale       []string // untouched past the freshness horizon
	suggested   []string // unconfirmed facts waiting for the owner
	notes       int
}

func (d dirt) due() bool {
	return d.checkIssues > 0 || len(d.stale) >= 10 || len(d.suggested) >= 5 || d.notes >= 50
}

func (d dirt) summary() string {
	var parts []string
	if d.checkIssues > 0 {
		parts = append(parts, fmt.Sprintf("%d check problem(s)", d.checkIssues))
	}
	if len(d.stale) > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", len(d.stale)))
	}
	if len(d.suggested) > 0 {
		parts = append(parts, fmt.Sprintf("%d unconfirmed", len(d.suggested)))
	}
	if d.notes > 0 {
		parts = append(parts, fmt.Sprintf("%d notes", d.notes))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " · ")
}

func gatherDirt(dir string, knownIssues int) dirt {
	d := dirt{checkIssues: knownIssues}
	now := time.Now()
	for _, area := range []string{"self", "peers"} {
		root := filepath.Join(dir, area)
		_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > staleAfter {
				d.stale = append(d.stale, rel)
			}
			if head, err := os.ReadFile(p); err == nil {
				if len(head) > 512 {
					head = head[:512]
				}
				if strings.Contains(string(head), "status: suggested") {
					d.suggested = append(d.suggested, rel)
				}
			}
			return nil
		})
	}
	_ = filepath.WalkDir(filepath.Join(dir, "notes"), func(p string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			d.notes++
		}
		return nil
	})
	return d
}

func cmdTidy(args []string) error {
	dir, err := vault.MustExist()
	if err != nil {
		return err
	}
	issues, err := check.Vault(dir)
	if err != nil {
		return err
	}
	d := gatherDirt(dir, len(issues))

	fmt.Printf("dirt: %s\n", d.summary())
	if d.checkIssues > 0 {
		fmt.Printf("\ncheck problems (fix first — these block disclosure):\n")
		for _, is := range issues {
			fmt.Println("  " + is.String())
		}
	}
	if len(d.suggested) > 0 {
		fmt.Println("\nunconfirmed facts (confirm with the owner → drop the suggested status, or delete):")
		for _, f := range capList(d.suggested, 10) {
			fmt.Println("  " + f)
		}
	}
	if len(d.stale) > 0 {
		fmt.Println("\nstale files, untouched 180+ days (reconfirm, update, or delete):")
		for _, f := range capList(d.stale, 10) {
			fmt.Println("  " + f)
		}
	}
	if d.notes >= 50 {
		fmt.Printf("\nnotes/ holds %d files — promote the durable ones into self/, delete the rest\n", d.notes)
	}

	if !d.due() {
		fmt.Println("\n✓ nothing needs attention")
		return nil
	}
	fmt.Println("\nhandle a small batch now (a few items is enough); rerun `dossier tidy` to see progress")
	return nil
}

func capList(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(items[:n:n], fmt.Sprintf("… and %d more", len(items)-n))
}
