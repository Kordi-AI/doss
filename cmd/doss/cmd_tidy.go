package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

// dirt is everything machines can flag but only judgment can resolve.
type dirt struct {
	checkIssues  int
	missingRough []string // rough-shared self facts missing a usable rough value
	stale        []string // possibly out of date (git-time based); informational, not an alarm
	suggested    []string // unconfirmed guesses waiting for the owner
	notes        int
}

// due decides whether to nudge. Staleness is deliberately NOT here: a fact
// going old over time isn't a mess to clean, just something to re-confirm
// when it's actually used — so it never triggers the alarm on its own.
func (d dirt) due() bool {
	return d.checkIssues > 0 || len(d.missingRough) > 0 || len(d.suggested) >= 5 || d.notes >= 50
}

// nudge is the plain-language, specific one-liner shown after a passing check.
// Empty when nothing needs attention.
func (d dirt) nudge() string {
	var parts []string
	if d.checkIssues > 0 {
		parts = append(parts, fmt.Sprintf("%d check problem(s) to fix", d.checkIssues))
	}
	if len(d.missingRough) > 0 {
		parts = append(parts, fmt.Sprintf("%d rough-shared fact(s) need rough values (%s) — add rough frontmatter; keep full facts in the body",
			len(d.missingRough), topicList(d.missingRough, 3)))
	}
	if len(d.suggested) >= 5 {
		parts = append(parts, fmt.Sprintf("%d guesses still unconfirmed (%s) — confirm or drop them",
			len(d.suggested), topicList(d.suggested, 3)))
	}
	if d.notes >= 50 {
		parts = append(parts, fmt.Sprintf("%d scratch notes piling up — promote the keepers, delete the rest", d.notes))
	}
	if len(parts) == 0 {
		return ""
	}
	return "tidy: " + strings.Join(parts, "; ") + "  (doss tidy)"
}

func gatherDirt(dir string, issues []check.Issue) dirt {
	d := dirt{checkIssues: len(issues), missingRough: missingRoughFiles(issues)}
	now := time.Now()
	for _, area := range []string{"self", "peers"} {
		root := filepath.Join(dir, area)
		_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			// Freshness from git commit time (survives clones), with mtime as a
			// fallback for not-yet-committed files.
			ref := gitx.LastCommitUnix(dir, filepath.ToSlash(rel))
			var last time.Time
			if ref > 0 {
				last = time.Unix(ref, 0)
			} else if info, err := e.Info(); err == nil {
				last = info.ModTime()
			}
			if !last.IsZero() && now.Sub(last) > staleAfter {
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

func missingRoughFiles(issues []check.Issue) []string {
	seen := map[string]bool{}
	var out []string
	for _, is := range issues {
		if is.Code != "E_ROUGH" || !strings.HasPrefix(filepath.ToSlash(is.File), "self/") {
			continue
		}
		rel := filepath.ToSlash(is.File)
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	return out
}

func nonRoughIssues(issues []check.Issue) []check.Issue {
	var out []check.Issue
	for _, is := range issues {
		if is.Code == "E_ROUGH" && strings.HasPrefix(filepath.ToSlash(is.File), "self/") {
			continue
		}
		out = append(out, is)
	}
	return out
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
	d := gatherDirt(dir, issues)

	shown := false
	if other := nonRoughIssues(issues); len(other) > 0 {
		shown = true
		fmt.Println("Fix first (these block disclosure):")
		for _, is := range other {
			fmt.Println("  " + is.String())
		}
		fmt.Println()
	}
	if len(d.missingRough) > 0 {
		shown = true
		fmt.Println("Missing rough values (these block rough-level disclosure):")
		for _, f := range capList(topicsOf(d.missingRough), 10) {
			fmt.Println("  " + f)
		}
		fmt.Println("  add rough frontmatter to each listed rough-shared fact; keep the full fact in the Markdown body")
		fmt.Println()
	}
	if len(d.suggested) > 0 {
		shown = true
		fmt.Printf("Unconfirmed guesses — confirm (drop `status: suggested`) or delete:\n")
		for _, f := range capList(topicsOf(d.suggested), 10) {
			fmt.Println("  " + f)
		}
		fmt.Println()
	}
	if len(d.stale) > 0 {
		shown = true
		fmt.Printf("Possibly out of date (untouched %d+ days) — re-confirm when you next use them, no rush:\n", int(staleAfter.Hours()/24))
		for _, f := range capList(topicsOf(d.stale), 10) {
			fmt.Println("  " + f)
		}
		fmt.Println()
	}
	if d.notes >= 50 {
		shown = true
		fmt.Printf("notes/ holds %d files — promote the durable ones into self/, delete the rest.\n\n", d.notes)
	}

	if !shown {
		fmt.Println("✓ nothing needs attention")
		return nil
	}
	if d.due() {
		fmt.Println("Handle a small batch now (a few is enough); rerun `doss tidy` to see progress.")
	} else {
		fmt.Println("Nothing urgent — the above is just for when you have a moment.")
	}
	return nil
}

// topicOf turns a vault path into a friendly dotted topic:
// self/profile/dietary.md -> profile.dietary
func topicOf(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = strings.TrimPrefix(rel, "self/")
	return strings.ReplaceAll(rel, "/", ".")
}

func topicsOf(rels []string) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = topicOf(r)
	}
	return out
}

// topicList renders up to n topics as "a, b, c, …" for one-line nudges.
func topicList(rels []string, n int) string {
	ts := topicsOf(rels)
	if len(ts) > n {
		return strings.Join(ts[:n], ", ") + ", …"
	}
	return strings.Join(ts, ", ")
}

func capList(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return append(items[:n:n], fmt.Sprintf("… and %d more", len(items)-n))
}
