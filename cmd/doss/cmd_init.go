package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/Kordi-AI/dossier/internal/gitx"
	"github.com/Kordi-AI/dossier/internal/vault"
)

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "vault directory (default $DOSS_HOME or ~/.doss)")
	github := fs.Bool("github", false, "create a private GitHub repo as the cloud copy (requires gh)")
	repo := fs.String("repo", "my-dossier", "repo name used with --github")
	remote := fs.String("remote", "", "attach an existing git remote URL as the cloud copy")
	from := fs.String("from", "", "attach this device to an existing vault: GitHub owner/repo or any git URL")
	noConnect := fs.Bool("no-connect", false, "skip wiring agent global configs (doss connect)")
	gitName := fs.String("git-name", "", "author name for vault commits (confirm with the owner)")
	gitEmail := fs.String("git-email", "", "author email for vault commits (confirm with the owner)")
	interactive := fs.Bool("interactive", false, "force the guided setup even when stdin is not a terminal")
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

	name := strings.TrimSpace(*gitName)
	email := strings.TrimSpace(*gitEmail)
	mode := "new"
	attachRef := ""
	attachToken := ""
	cloudToken := ""
	wantGitHub := *github

	if *from != "" {
		mode = "attach"
		attachRef = *from
	}

	// Humans get a guided setup: one question at a time, nothing assumed.
	// Agents pass flags and never see it.
	guided := mode == "new" && !wantGitHub && *remote == "" &&
		(*interactive || stdinIsTTY())
	if guided {
		p := newPrompter()
		fmt.Println("Doss setup — your agent's memory, your rules.")
		fmt.Println()
		if p.choose("Is this your first vault, or do you already have one in the cloud?",
			"Create a new vault on this machine",
			"Connect this device to my existing cloud vault") == 1 {
			mode = "attach"
			_ = p.choose("Where does it live? (more clouds later)", "GitHub")
			for attachRef == "" {
				attachRef = p.ask("GitHub repo (owner/name, e.g. you/my-dossier)", "")
			}
			if !ghLoggedIn() {
				fmt.Println("gh CLI isn't logged in — a GitHub personal access token (repo scope) works too.")
				attachToken = p.secret("GitHub token")
			}
		}

		fmt.Println()
		fmt.Println("Vault commits are signed as you — not as a machine guess.")
		for name == "" {
			name = p.ask("  your name", gitGlobalConfig("user.name"))
		}
		for email == "" {
			email = p.ask("  your email", gitGlobalConfig("user.email"))
		}

		if mode == "new" {
			fmt.Println()
			if p.choose("Back the vault up to a private cloud copy? (enables sync and other devices)",
				"Yes — private GitHub repo",
				"Not now (add anytime later)") == 0 {
				wantGitHub = true
				*repo = p.ask("  repo name", *repo)
				if !ghLoggedIn() {
					fmt.Println("  gh CLI isn't logged in — a GitHub personal access token (repo scope) works too.")
					cloudToken = p.secret("  GitHub token")
				}
			}
		}
		fmt.Println()
	}

	// Non-interactive fallback: flags, then git's global config, then refuse.
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
  doss init --git-name "Owner Name" --git-email owner@example.com`)
	}
	if confirmed || guided {
		fmt.Printf("vault commits authored as: %s <%s>\n", name, email)
	} else {
		fmt.Printf("vault commits authored as: %s <%s> (from git config — confirm this with the owner;\n  change anytime: git -C %s config user.name / user.email)\n", name, email, d)
	}

	if mode == "attach" {
		if *github || *remote != "" {
			return fmt.Errorf("--from attaches to an existing vault; don't combine it with --github or --remote")
		}
		if entries, err := os.ReadDir(d); err == nil && len(entries) > 0 {
			return fmt.Errorf("%s already exists and is not empty", d)
		}
		if attachToken != "" {
			if out, err := exec.Command("git", "clone", tokenCloneURL(attachRef, attachToken), d).CombinedOutput(); err != nil {
				return fmt.Errorf("clone failed: %s", sanitizeToken(strings.TrimSpace(string(out)), attachToken))
			}
			fmt.Println("note: the token is stored in this vault's git remote URL (a local file); prefer `gh auth login` when you can")
		} else if err := cloneVault(attachRef, d); err != nil {
			return err
		}
		if !vault.Exists(d) {
			return fmt.Errorf("cloned %s, but it doesn't look like a doss vault (no self/ + policy.yaml)", attachRef)
		}
		if _, err := gitx.Run(d, "config", "user.name", name); err != nil {
			return err
		}
		if _, err := gitx.Run(d, "config", "user.email", email); err != nil {
			return err
		}
		abs, _ := filepath.Abs(d)
		fmt.Printf("✓ vault attached: %s\n  cloud copy: %s\n  this device now shares the same memory — `doss sync` keeps them aligned\n  have your agent read %s\n", abs, attachRef, filepath.Join(abs, "SKILL.md"))
		return maybeConnect(*noConnect)
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
	if _, err := gitx.Run(d, "commit", "-m", "doss: init vault"); err != nil {
		return err
	}

	cloud := "local only"
	switch {
	case wantGitHub && cloudToken != "":
		url, fullName, err := githubCreateRepoWithToken(cloudToken, *repo)
		if err != nil {
			return err
		}
		if _, err := gitx.Run(d, "remote", "add", "origin", url); err != nil {
			return err
		}
		if out, err := gitx.Run(d, "push", "-u", "origin", "main"); err != nil {
			return fmt.Errorf("push failed: %s", sanitizeToken(out, cloudToken))
		}
		cloud = "private GitHub repo " + fullName
		fmt.Println("note: the token is stored in this vault's git remote URL (a local file); prefer `gh auth login` when you can")
	case wantGitHub:
		if _, err := exec.LookPath("gh"); err != nil {
			return fmt.Errorf("--github needs the GitHub CLI (gh). install: https://cli.github.com — or use --remote <url>")
		}
		out, err := exec.Command("gh", "repo", "create", *repo,
			"--private", "--source", d, "--remote", "origin", "--push",
			"--description", "My private Doss memory vault").CombinedOutput()
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
  2. edit memory freely; run "doss check --changed" after edits, "doss sync" when done
`, abs, cloud, filepath.Join(abs, "SKILL.md"))

	if cloud == "local only" {
		fmt.Println(`  3. add cloud sync anytime:
       git -C ` + abs + ` remote add origin <url>   (or: gh repo create my-dossier --private --source ` + abs + ` --remote origin --push)`)
	}
	return maybeConnect(*noConnect)
}

func maybeConnect(skip bool) error {
	if skip {
		return nil
	}
	fmt.Println("\nwiring agents (doss connect):")
	if err := cmdConnect(nil); err != nil {
		return fmt.Errorf("vault is ready, but wiring agents failed: %w (rerun with `doss connect`)", err)
	}
	return nil
}

// ghLoggedIn reports whether the GitHub CLI is present and authenticated.
func ghLoggedIn() bool {
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	return exec.Command("gh", "auth", "status").Run() == nil
}

// cloneVault fetches an existing vault. A full URL always goes straight to
// git; the owner/repo shorthand uses gh (only if it's authenticated, so we
// never hang on an interactive auth prompt).
func cloneVault(src, dir string) error {
	if !strings.Contains(src, "://") && ghLoggedIn() {
		if out, err := exec.Command("gh", "repo", "clone", src, dir).CombinedOutput(); err == nil {
			return nil
		} else {
			_ = os.RemoveAll(dir) // partial clone; dir was empty before (checked)
			return fmt.Errorf("clone failed: %s", strings.TrimSpace(string(out)))
		}
	}
	if out, err := exec.Command("git", "clone", src, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("clone failed: %s\n(private repo? log in with `gh auth login`, or pass a token when prompted)", strings.TrimSpace(string(out)))
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

func tokenCloneURL(ownerRepo, token string) string {
	ref := strings.TrimSuffix(strings.TrimSpace(ownerRepo), ".git")
	ref = strings.TrimPrefix(ref, "https://github.com/")
	return "https://" + token + "@github.com/" + ref + ".git"
}

func sanitizeToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}

func githubCreateRepoWithToken(token, name string) (cloneURL, fullName string, err error) {
	req, err := http.NewRequest("POST", "https://api.github.com/user/repos",
		strings.NewReader(fmt.Sprintf(`{"name":%q,"private":true,"description":"My private Doss memory vault"}`, name)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 201 {
		return "", "", fmt.Errorf("GitHub said %s — check the token (needs repo scope) and that %q doesn't already exist", resp.Status, name)
	}
	var out struct {
		FullName string `json:"full_name"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.FullName == "" {
		return "", "", fmt.Errorf("unexpected GitHub response")
	}
	return "https://" + token + "@github.com/" + out.FullName + ".git", out.FullName, nil
}

// --- tiny prompt helpers (stdlib only) ---

func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type prompter struct{ r *bufio.Reader }

func newPrompter() *prompter { return &prompter{bufio.NewReader(os.Stdin)} }

func (p *prompter) ask(q, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", q, def)
	} else {
		fmt.Printf("%s: ", q)
	}
	line, _ := p.r.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

func (p *prompter) choose(q string, options ...string) int {
	fmt.Println(q)
	for i, o := range options {
		fmt.Printf("  %d) %s\n", i+1, o)
	}
	for {
		fmt.Print("> ")
		line, err := p.r.ReadString('\n')
		line = strings.TrimSpace(line)
		for i := range options {
			if line == fmt.Sprintf("%d", i+1) {
				return i
			}
		}
		if err != nil {
			return 0 // EOF: take the first (safest) option
		}
		fmt.Printf("please answer 1-%d\n", len(options))
	}
}

func (p *prompter) secret(q string) string {
	fmt.Printf("%s (input hidden): ", q)
	if stdinIsTTY() {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	line, _ := p.r.ReadString('\n')
	return strings.TrimSpace(line)
}
