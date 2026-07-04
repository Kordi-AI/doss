package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

// cmdUninstall tears down the local vault and unwires the agents. Like git,
// it refuses to quietly destroy work that isn't backed up: without a cloud
// copy, or with changes not yet pushed, it stops unless you insist.
func cmdUninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt (required for non-interactive use)")
	force := fs.Bool("force", false, "delete even when the vault has no cloud copy or has unpushed changes")
	keepAgents := fs.Bool("keep-agents", false, "leave the agent instruction files wired")
	if err := fs.Parse(args); err != nil {
		return err
	}

	d := vault.Dir()
	if !vault.Exists(d) {
		// No vault, but agents may still be wired at a dangling path.
		fmt.Printf("no vault at %s\n", d)
		if !*keepAgents {
			fmt.Println("removing any leftover agent wiring:")
			return cmdConnect([]string{"--remove"})
		}
		return nil
	}
	abs, _ := filepath.Abs(d)

	// Assess how safe deletion is.
	hasRemote := gitx.HasRemote(d)
	dirty, _ := gitx.Dirty(d)
	unpushed := unpushedCount(d)
	facts := countFacts(d)

	fmt.Printf("This will delete your local vault:\n  %s  (%d fact file(s))\n\n", abs, facts)

	safe := true
	switch {
	case !hasRemote:
		fmt.Println("⚠️  This vault has NO cloud copy. Deleting it erases all of it permanently.")
		safe = false
	case dirty || unpushed > 0:
		var parts []string
		if dirty {
			parts = append(parts, "uncommitted edits")
		}
		if unpushed > 0 {
			parts = append(parts, fmt.Sprintf("%d commit(s) not pushed since last sync", unpushed))
		}
		fmt.Printf("⚠️  %s are NOT in the cloud yet — run `dossier sync` first or they're lost.\n", strings.Join(parts, " and "))
		safe = false
	default:
		remote, _ := gitx.Run(d, "remote", "get-url", "origin")
		fmt.Printf("✓ Your memory stays safe in the cloud: %s\n  This only removes the local copy — re-attach anytime with `dossier init --from <repo>`.\n", strings.TrimSpace(remote))
	}
	if !*keepAgents {
		fmt.Println("\nIt also unwires Dossier from your agents' global config.")
	}
	fmt.Println()

	if !safe && !*force {
		if !*yes && stdinIsTTY() {
			// fall through to the typed-name confirmation below
		} else {
			return fmt.Errorf("refusing to delete an un-backed-up vault; rerun with --force if you really mean it")
		}
	}

	// Confirm.
	if !*yes {
		if !stdinIsTTY() {
			return fmt.Errorf("this is destructive — rerun with --yes to confirm (or --force if there's no cloud copy)")
		}
		p := newPrompter()
		want := filepath.Base(abs)
		got := p.ask(fmt.Sprintf("type the vault folder name (%q) to confirm deletion", want), "")
		if got != want {
			return fmt.Errorf("names don't match — nothing was deleted")
		}
	}

	// Do it: unwire first (needs the vault path), then remove the directory.
	if !*keepAgents {
		if err := cmdConnect([]string{"--remove"}); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not fully unwire agents:", err)
		}
	}
	if err := os.RemoveAll(d); err != nil {
		return fmt.Errorf("removing %s: %w", d, err)
	}

	fmt.Printf("\n✓ vault deleted: %s\n", abs)
	if hasRemote {
		fmt.Println("  your cloud copy is untouched — `dossier init --from <repo>` brings it back on any device")
	}
	fmt.Println("  the dossier binary is still installed; remove it with:  rm", binPathHint())
	return nil
}

func countFacts(d string) int {
	n := 0
	for _, area := range []string{"self", "peers"} {
		_ = filepath.Walk(filepath.Join(d, area), func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
				n++
			}
			return nil
		})
	}
	return n
}

// unpushedCount counts local commits ahead of the last-known remote tip.
// No network: it reflects state as of the last sync, which is enough to warn.
func unpushedCount(d string) int {
	out, err := gitx.Run(d, "rev-list", "--count", "origin/main..main")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func binPathHint() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "$(which dossier)"
}
