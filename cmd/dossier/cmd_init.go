package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

// cloneVault fetches an existing vault. gh handles GitHub auth and the
// owner/repo shorthand; plain git covers every other remote.
func cloneVault(src, dir string) error {
	if _, err := exec.LookPath("gh"); err == nil {
		if _, err := exec.Command("gh", "repo", "clone", src, dir).CombinedOutput(); err == nil {
			return nil
		}
		_ = os.RemoveAll(dir) // partial clone; dir was empty before (checked)
	}
	if out, err := exec.Command("git", "clone", src, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("clone failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gitGlobalConfig reads a key from git's global config only — deliberately
// not the local config of whatever directory we happen to run in.
func gitGlobalConfig(key string) string {
	out, err := gitx.Run(".", "config", "--global", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "vault directory (default $DOSSIER_HOME or ~/.dossier)")
	github := fs.Bool("github", false, "create a private GitHub repo as the cloud copy (requires gh)")
	repo := fs.String("repo", "my-dossier", "repo name used with --github")
	remote := fs.String("remote", "", "attach an existing git remote URL as the cloud copy")
	noConnect := fs.Bool("no-connect", false, "skip wiring agent global configs (dossier connect)")
	gitName := fs.String("git-name", "", "author name for vault commits (confirm with the owner)")
	gitEmail := fs.String("git-email", "", "author email for vault commits (confirm with the owner)")
	from := fs.String("from", "", "attach this device to an existing vault: GitHub owner/repo or any git URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	d := *dir
	if d == "" {
		d = vault.Dir()
	}
	if vault.Exists(d) {
		return fmt.Errorf("vault already exists at %s", d)
	}

	// The vault is the owner's personal history — commits must carry an
	// identity the owner confirmed, not whatever git guesses from the host.
	name := strings.TrimSpace(*gitName)
	email := strings.TrimSpace(*gitEmail)
	confirmed := name != "" && email != ""
	if name == "" {
		name = gitGlobalConfig("user.name")
	}
	if email == "" {
		email = gitGlobalConfig("user.email")
	}
	if name == "" || email == "" {
		return fmt.Errorf(`vault commits need a confirmed identity, and git has none configured.
ask the owner what identity their memory vault should commit as, then rerun:
  dossier init --git-name "Owner Name" --git-email owner@example.com`)
	}
	if confirmed {
		fmt.Printf("vault commits authored as: %s <%s>\n", name, email)
	} else {
		fmt.Printf("vault commits authored as: %s <%s> (from git config — confirm this with the owner;\n  change anytime: git -C %s config user.name / user.email)\n", name, email, d)
	}

	if *from != "" {
		if *github || *remote != "" {
			return fmt.Errorf("--from attaches to an existing vault; don't combine it with --github or --remote")
		}
		if entries, err := os.ReadDir(d); err == nil && len(entries) > 0 {
			return fmt.Errorf("%s already exists and is not empty", d)
		}
		if err := cloneVault(*from, d); err != nil {
			return err
		}
		if !vault.Exists(d) {
			return fmt.Errorf("cloned %s, but it doesn't look like a dossier vault (no self/ + policy.yaml)", *from)
		}
		if _, err := gitx.Run(d, "config", "user.name", name); err != nil {
			return err
		}
		if _, err := gitx.Run(d, "config", "user.email", email); err != nil {
			return err
		}
		abs, _ := filepath.Abs(d)
		fmt.Printf("✓ vault attached: %s\n  cloud copy: %s\n  this device now shares the same memory — `dossier sync` keeps them aligned\n  have your agent read %s\n", abs, *from, filepath.Join(abs, "SKILL.md"))
		if !*noConnect {
			fmt.Println("\nwiring agents (dossier connect):")
			if err := cmdConnect(nil); err != nil {
				return fmt.Errorf("vault is ready, but wiring agents failed: %w (rerun with `dossier connect`)", err)
			}
		}
		return nil
	}

	if err := vault.Scaffold(d); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "init", "-b", "main"); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "config", "user.name", name); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "config", "user.email", email); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "add", "-A"); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "commit", "-m", "dossier: init vault"); err != nil {
		return err
	}

	cloud := "local only"
	switch {
	case *github:
		if _, err := exec.LookPath("gh"); err != nil {
			return fmt.Errorf("--github needs the GitHub CLI (gh). install: https://cli.github.com — or use --remote <url>")
		}
		out, err := exec.Command("gh", "repo", "create", *repo,
			"--private", "--source", d, "--remote", "origin", "--push",
			"--description", "My private Dossier memory vault").CombinedOutput()
		if err != nil {
			return fmt.Errorf("gh repo create failed: %s", string(out))
		}
		cloud = "private GitHub repo " + *repo
	case *remote != "":
		if _, err := gitx.Run(d, "remote", "add", "origin", *remote); err != nil {
			return err
		}
		if out, err := gitx.Run(d, "push", "-u", "origin", "main"); err != nil {
			return fmt.Errorf("push to %s failed: %s", *remote, out)
		}
		cloud = *remote
	}

	abs, _ := filepath.Abs(d)
	fmt.Printf(`✓ vault ready: %s
  self/         facts about the owner (path = topic)
  peers/        what others shared with you
  notes/        scratch — never leaves this machine
  policy.yaml   disclosure rules (default: nothing leaves)

cloud sync: %s

next steps:
  1. have your agent read %s
  2. edit memory freely; run "dossier check --changed" after edits, "dossier sync" when done
`, abs, cloud, filepath.Join(abs, "SKILL.md"))

	if !*github && *remote == "" {
		fmt.Println(`  3. add cloud sync anytime:
       git -C ` + abs + ` remote add origin <url>   (or: gh repo create my-dossier --private --source ` + abs + ` --remote origin --push)`)
	}

	if !*noConnect {
		fmt.Println("\nwiring agents (dossier connect):")
		if err := cmdConnect(nil); err != nil {
			return fmt.Errorf("vault is ready, but wiring agents failed: %w (rerun with `dossier connect`)", err)
		}
	}
	_ = os.Stdout.Sync()
	return nil
}
