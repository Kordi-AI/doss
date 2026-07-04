package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Kordi-AI/dossier/internal/check"
	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fix := fs.Bool("fix", false, "repair wiring problems by running dossier connect")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var problems []string

	exe, _ := os.Executable()
	fmt.Printf("  binary   dossier %s (%s)\n", version, exe)

	vd := vault.Dir()
	if vault.Exists(vd) {
		issues, err := check.Vault(vd)
		if err != nil {
			return err
		}
		state := "check clean"
		if len(issues) > 0 {
			state = fmt.Sprintf("%d check problem(s) — run `dossier check`", len(issues))
			problems = append(problems, "vault has check problems")
		}
		sync := "local only — no cloud copy"
		if gitx.HasRemote(vd) {
			sync = "cloud sync on"
		}
		fmt.Printf("  vault    %s — %s, %s\n", vd, state, sync)
	} else {
		fmt.Printf("  vault    %s — missing\n", vd)
		problems = append(problems, "no vault — run `dossier init`")
	}

	fmt.Println("  wiring   (does each agent load the rules in every project?)")
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
				fmt.Println("  hooks    ✓ Claude Code write/stop hooks wired (bounce-and-retry live)")
			} else {
				fmt.Println("  hooks    ✗ Claude Code hooks missing")
				broken = true
			}
		}
	}

	if broken {
		if *fix {
			fmt.Println("\nrepairing wiring:")
			if err := cmdConnect(nil); err != nil {
				return err
			}
			broken = false
			for _, w := range wiringStates() {
				if w.broken() {
					broken = true
				}
			}
		}
		if broken {
			problems = append(problems, "agent wiring incomplete — run `dossier connect` (or `dossier doctor --fix`)")
		}
	}

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
