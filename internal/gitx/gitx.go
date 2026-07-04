// Package gitx shells out to git; dossier keeps git as the sync substrate.
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
