// Package gitx shells out to git; doss keeps git as the sync substrate.
package gitx

import (
	"fmt"
	"os/exec"
	"strings"
)

// Run executes git -C dir args... and returns combined output.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// Dirty reports whether the working tree has uncommitted changes.
func Dirty(dir string) (bool, error) {
	out, err := Run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// HasRemote reports whether an origin remote is configured.
func HasRemote(dir string) bool {
	_, err := Run(dir, "remote", "get-url", "origin")
	return err == nil
}

// CurrentBranch returns the checked-out branch. Doss-created vaults use main,
// but attached vaults may use another default branch.
func CurrentBranch(dir string) string {
	out, err := Run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(out)
	if branch == "" || branch == "HEAD" {
		return "main"
	}
	return branch
}

// Upstream returns the configured upstream ref for the current branch, if any.
func Upstream(dir string) string {
	out, err := Run(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// LastCommitUnix returns the Unix time of the most recent commit touching
// relpath, or 0 if the file has no commit yet (brand new / uncommitted).
// Commit time survives clones; filesystem mtime does not.
func LastCommitUnix(dir, relpath string) int64 {
	out, err := Run(dir, "log", "-1", "--format=%ct", "--", relpath)
	if err != nil {
		return 0
	}
	var t int64
	fmt.Sscanf(strings.TrimSpace(out), "%d", &t)
	return t
}

// ChangedFiles lists files modified since HEAD plus untracked files,
// relative to the repo root.
func ChangedFiles(dir string) ([]string, error) {
	var files []string
	seen := map[string]bool{}
	for _, args := range [][]string{
		{"diff", "--name-only", "HEAD"},
		{"ls-files", "--others", "--exclude-standard"},
	} {
		out, err := Run(dir, args...)
		if err != nil {
			return nil, err
		}
		for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
			if f != "" && !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files, nil
}
