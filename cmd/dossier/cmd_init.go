package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "vault directory (default $DOSSIER_HOME or ~/.dossier)")
	github := fs.Bool("github", false, "create a private GitHub repo as the cloud copy (requires gh)")
	repo := fs.String("repo", "my-dossier", "repo name used with --github")
	remote := fs.String("remote", "", "attach an existing git remote URL as the cloud copy")
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

	if err := vault.Scaffold(d); err != nil {
		return err
	}
	if _, err := gitx.Run(d, "init", "-b", "main"); err != nil {
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
  1. run "dossier connect" — every agent on this machine will load the rules in all projects
  2. have your agent read %s
  3. edit memory freely; run "dossier check --changed" after edits, "dossier sync" when done
`, abs, cloud, filepath.Join(abs, "SKILL.md"))

	if !*github && *remote == "" {
		fmt.Println(`  3. add cloud sync anytime: dossier init is done, so use
       git -C ` + abs + ` remote add origin <url>   (or: gh repo create my-dossier --private --source ` + abs + ` --remote origin --push)`)
	}
	_ = os.Stdout.Sync()
	return nil
}
