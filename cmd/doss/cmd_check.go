package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	changed := fs.Bool("changed", false, "only check files changed since the last commit")
	quiet := fs.Bool("quiet", false, "print nothing when everything passes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dv, err := vault.MustExist()
	if err != nil {
		return err
	}

	var issues []check.Issue
	var scope string
	if *changed {
		files, err := gitx.ChangedFiles(dv)
		if err != nil {
			return err
		}
		files = includeLocalAccess(dv, files)
		issues, err = check.Files(dv, files)
		if err != nil {
			return err
		}
		scope = fmt.Sprintf("%d changed file(s)", len(files))
	} else {
		issues, err = check.Vault(dv)
		if err != nil {
			return err
		}
		scope = "vault"
	}

	if len(issues) == 0 {
		if !*quiet {
			fmt.Printf("✓ check passed (%s)\n", scope)
		}
		// The dirt-threshold nudge: maintenance piggybacks on a moment
		// the agent is already awake, like allocation-triggered GC.
		if d := gatherDirt(dv, nil); d.due() {
			fmt.Println(d.nudge())
		}
		return nil
	}
	for _, is := range issues {
		fmt.Println(is)
	}
	return fmt.Errorf("%d problem(s) — fix and rerun `doss check --changed`", len(issues))
}

func includeLocalAccess(dir string, files []string) []string {
	rel := filepath.ToSlash(filepath.Join("local", "access.yaml"))
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		return files
	}
	for _, f := range files {
		if filepath.ToSlash(f) == rel {
			return files
		}
	}
	return append(files, rel)
}
