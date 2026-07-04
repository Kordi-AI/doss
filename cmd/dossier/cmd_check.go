package main

import (
	"flag"
	"fmt"

	"github.com/Kordi-AI/dossier/internal/check"
	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	changed := fs.Bool("changed", false, "only check files changed since the last commit")
	quiet := fs.Bool("quiet", false, "print nothing when everything passes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	d, err := vault.MustExist()
	if err != nil {
		return err
	}

	var issues []check.Issue
	var scope string
	if *changed {
		files, err := gitx.ChangedFiles(d)
		if err != nil {
			return err
		}
		issues, err = check.Files(d, files)
		if err != nil {
			return err
		}
		scope = fmt.Sprintf("%d changed file(s)", len(files))
	} else {
		issues, err = check.Vault(d)
		if err != nil {
			return err
		}
		scope = "vault"
	}

	if len(issues) == 0 {
		if !*quiet {
			fmt.Printf("✓ check passed (%s)\n", scope)
		}
		return nil
	}
	for _, is := range issues {
		fmt.Println(is)
	}
	return fmt.Errorf("%d problem(s) — fix and rerun `dossier check --changed`", len(issues))
}
