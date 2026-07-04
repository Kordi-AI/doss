// Package vault knows where the doss lives and how to scaffold one.
package vault

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed templates
var tmpl embed.FS

// Dir returns the vault directory: $DOSS_HOME or ~/.doss.
func Dir() string {
	if d := os.Getenv("DOSS_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".doss"
	}
	return filepath.Join(home, ".doss")
}

// Exists reports whether dir already contains an initialized vault.
func Exists(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "policy.yaml")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "self")); err != nil {
		return false
	}
	return true
}

// MustExist returns the vault directory or an actionable error.
func MustExist() (string, error) {
	d := Dir()
	if !Exists(d) {
		return "", fmt.Errorf("no vault at %s — run `doss init` first", d)
	}
	return d, nil
}

// Scaffold creates the vault layout. It never overwrites existing files.
func Scaffold(dir string) error {
	for _, sub := range []string{"self", "peers", "notes"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
		keep := filepath.Join(dir, sub, ".gitkeep")
		if _, err := os.Stat(keep); os.IsNotExist(err) {
			if err := os.WriteFile(keep, nil, 0o644); err != nil {
				return err
			}
		}
	}
	files := map[string]string{
		"SKILL.md":    "templates/SKILL.md",
		"policy.yaml": "templates/policy.yaml",
		"README.md":   "templates/vault-readme.md",
		".gitignore":  "templates/vault-gitignore",
	}
	for dst, src := range files {
		path := filepath.Join(dir, dst)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		b, err := fs.ReadFile(tmpl, src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}
