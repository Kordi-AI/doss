package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	quiet := fs.Bool("quiet", false, "print nothing when there is nothing to do")
	if err := fs.Parse(args); err != nil {
		return err
	}

	d, err := vault.MustExist()
	if err != nil {
		return err
	}
	if err := ensureCurrentDeviceCanSync(d); err != nil {
		return err
	}
	if _, err := vault.RegisterDevice(d); err != nil {
		return err
	}
	if err := ensureGitHubDeviceKey(d, ""); err != nil {
		return err
	}

	// Only validated state ever syncs.
	issues, err := check.Vault(d)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		for _, is := range issues {
			fmt.Println(is)
		}
		return fmt.Errorf("refusing to sync: %d problem(s) — nothing was committed", len(issues))
	}

	return syncGit(d, "doss sync: "+time.Now().Format("2006-01-02 15:04"), *quiet)
}

func ensureCurrentDeviceCanSync(d string) error {
	id := vault.DeviceID(d)
	dev, err := vault.DeviceRecord(d, id)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if dev.Status == "deactivated" {
		return fmt.Errorf("this device (%s) is deactivated; refusing to sync. Reattach it with `doss init --from <repo>` after the owner approves a new device key", id)
	}
	return nil
}

func syncGit(d, msg string, quiet bool) error {
	dirty, err := gitx.Dirty(d)
	if err != nil {
		return err
	}
	committed := false
	if dirty {
		if _, err := gitx.Run(d, "add", "-A"); err != nil {
			return err
		}
		if _, err := gitx.Run(d, "commit", "-m", msg); err != nil {
			return err
		}
		committed = true
	}

	pushed := false
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
				return fmt.Errorf("pull hit a conflict; sync aborted safely, nothing lost.\nresolve by hand in %s (both versions are in git), then rerun `doss sync`.\ngit said: %s", d, out)
			}
		}
		if out, err := gitx.Run(d, pushArgs...); err != nil {
			_, _ = gitx.Run(d, "rebase", "--abort")
			return fmt.Errorf("push failed: %s", out)
		}
		pushed = true
	}

	switch {
	case committed && pushed:
		fmt.Println("✓ synced (committed + pushed)")
	case committed:
		fmt.Println("✓ committed (local only — no remote configured)")
	case pushed:
		fmt.Println("✓ up to date with remote")
	default:
		if !quiet {
			fmt.Println("✓ nothing to sync")
		}
	}
	return nil
}
