// Package check validates vault files. Deterministic, milliseconds, precise
// errors. This is the structural guarantee behind "the library is always
// clean": nothing unvalidated syncs or discloses.
package check

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Issue is one problem found in one file.
type Issue struct {
	File string
	Line int
	Code string
	Msg  string
	Hint string
}

func (i Issue) String() string {
	loc := i.File
	if i.Line > 0 {
		loc = fmt.Sprintf("%s:%d", i.File, i.Line)
	}
	s := fmt.Sprintf("%s [%s] %s", loc, i.Code, i.Msg)
	if i.Hint != "" {
		s += " — " + i.Hint
	}
	return s
}

var (
	nameRe = regexp.MustCompile(`^[a-z0-9._-]+$`)

	rootAllowed = map[string]bool{
		"self": true, "peers": true, "notes": true,
		"policy.yaml": true, "SKILL.md": true, "README.md": true,
		"ledger.log": true, ".git": true, ".gitignore": true, ".index": true,
	}
	allowedKeys = map[string]bool{
		"source": true, "status": true, "confidence": true,
		"tags": true, "verify_by": true, "evidence": true,
	}
	sourceVals = map[string]bool{"owner": true, "imported": true, "inferred": true, "peer": true}
	statusVals = map[string]bool{"active": true, "suggested": true}
	giveVals   = map[string]bool{"full": true, "rough": true, "yes-no": true, "nothing": true}
)

const maxFileSize = 128 * 1024

// Vault checks the whole vault.
func Vault(dir string) ([]Issue, error) {
	var issues []Issue

	// Root layout: no strays.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !rootAllowed[e.Name()] && e.Name() != ".DS_Store" {
			issues = append(issues, Issue{
				File: e.Name(), Code: "E_LAYOUT",
				Msg:  "unexpected item at vault root",
				Hint: "memory goes under self/, peers/, or notes/",
			})
		}
	}

	for _, area := range []string{"self", "peers"} {
		root := filepath.Join(dir, area)
		lower := map[string]string{} // case-collision detection, per vault area
		_ = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			base := filepath.Base(p)
			if base == ".gitkeep" || base == ".DS_Store" {
				return nil
			}
			lc := strings.ToLower(rel)
			if prev, dup := lower[lc]; dup {
				issues = append(issues, Issue{File: rel, Code: "E_CASE",
					Msg: "path differs only by letter case from " + prev, Hint: "rename one of them"})
			}
			lower[lc] = rel
			if e.IsDir() {
				if !nameRe.MatchString(base) {
					issues = append(issues, Issue{File: rel, Code: "E_NAME",
						Msg: "directory name must be lowercase [a-z0-9._-]", Hint: "e.g. work-history"})
				}
				return nil
			}
			issues = append(issues, checkFile(dir, rel)...)
			return nil
		})
	}

	issues = append(issues, checkPolicy(dir)...)
	return issues, nil
}

// Files checks a specific set of vault-relative paths (plus policy.yaml if listed).
func Files(dir string, files []string) ([]Issue, error) {
	var issues []Issue
	for _, f := range files {
		f = filepath.ToSlash(f)
		switch {
		case f == "policy.yaml":
			issues = append(issues, checkPolicy(dir)...)
		case strings.HasPrefix(f, "self/") || strings.HasPrefix(f, "peers/"):
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				issues = append(issues, checkFile(dir, f)...)
			}
		case strings.HasPrefix(f, "notes/"), f == "SKILL.md", f == "README.md",
			f == ".gitignore", f == "ledger.log":
			// exempt
		default:
			issues = append(issues, Issue{File: f, Code: "E_LAYOUT",
				Msg:  "unexpected location",
				Hint: "memory goes under self/, peers/, or notes/"})
		}
	}
	return issues, nil
}

func checkFile(dir, rel string) []Issue {
	var issues []Issue
	base := filepath.Base(rel)

	if !nameRe.MatchString(base) {
		issues = append(issues, Issue{File: rel, Code: "E_NAME",
			Msg: "file name must be lowercase [a-z0-9._-]", Hint: "e.g. dietary.md"})
	}
	ext := filepath.Ext(base)
	if ext != ".md" && ext != ".yaml" && ext != ".yml" {
		issues = append(issues, Issue{File: rel, Code: "E_EXT",
			Msg: "unsupported file type " + ext, Hint: "use .md (or .yaml for pure data)"})
		return issues
	}

	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return append(issues, Issue{File: rel, Code: "E_READ", Msg: err.Error()})
	}
	if len(b) > maxFileSize {
		issues = append(issues, Issue{File: rel, Code: "E_SIZE",
			Msg: fmt.Sprintf("file is %dKB (limit 128KB)", len(b)/1024),
			Hint: "split by topic — one topic per file"})
	}

	if ext == ".yaml" || ext == ".yml" {
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			issues = append(issues, Issue{File: rel, Line: yamlLine(err), Code: "E_YAML",
				Msg: "invalid YAML: " + yamlMsg(err)})
		}
		return issues
	}

	fm, body, ok := splitFrontmatter(b)
	if ok {
		issues = append(issues, checkFrontmatter(rel, fm)...)
	} else {
		body = b
	}
	if strings.TrimSpace(string(body)) == "" {
		issues = append(issues, Issue{File: rel, Code: "E_EMPTY",
			Msg: "no content", Hint: "write the fact, or delete the file"})
	}
	return issues
}

func checkFrontmatter(rel string, fm []byte) []Issue {
	var issues []Issue
	var m map[string]any
	if err := yaml.Unmarshal(fm, &m); err != nil {
		return []Issue{{File: rel, Line: yamlLine(err) + 1, Code: "E_YAML",
			Msg: "invalid frontmatter: " + yamlMsg(err)}}
	}
	for k, v := range m {
		if strings.HasPrefix(k, "x-") {
			continue
		}
		if !allowedKeys[k] {
			issues = append(issues, Issue{File: rel, Code: "E_FIELD",
				Msg:  fmt.Sprintf("unknown frontmatter key %q", k),
				Hint: "allowed: source, status, confidence, tags, verify_by, evidence (x-* for extensions)"})
			continue
		}
		switch k {
		case "source":
			if s, _ := v.(string); !sourceVals[s] {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: fmt.Sprintf("source: %v", v), Hint: "one of: owner, imported, inferred, peer"})
			}
		case "status":
			if s, _ := v.(string); !statusVals[s] {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: fmt.Sprintf("status: %v", v), Hint: "one of: active, suggested"})
			}
		case "confidence":
			ok := false
			switch c := v.(type) {
			case string:
				ok = c == "high" || c == "medium" || c == "low"
			case int:
				ok = c >= 0 && c <= 1
			case float64:
				ok = c >= 0 && c <= 1
			}
			if !ok {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: fmt.Sprintf("confidence: %v", v), Hint: "high|medium|low or a number 0–1"})
			}
		case "tags":
			list, ok := v.([]any)
			if !ok {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: "tags must be a list", Hint: "tags: [health, food]"})
				break
			}
			for _, t := range list {
				if _, ok := t.(string); !ok {
					issues = append(issues, Issue{File: rel, Code: "E_VALUE",
						Msg: fmt.Sprintf("tag %v is not a string", t)})
				}
			}
		case "verify_by":
			s := fmt.Sprintf("%v", v)
			if _, err := time.Parse("2006-01-02", s); err != nil {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: "verify_by: " + s, Hint: "date as YYYY-MM-DD"})
			}
		case "evidence":
			if _, ok := v.(string); !ok {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: "evidence must be a string"})
			}
		}
	}
	// Inferred facts must stay suggestions until confirmed.
	if src, _ := m["source"].(string); src == "inferred" {
		if st, _ := m["status"].(string); st != "suggested" {
			issues = append(issues, Issue{File: rel, Code: "E_RULE",
				Msg:  "source: inferred requires status: suggested",
				Hint: "guesses become facts only after the owner confirms"})
		}
	}
	return issues
}

func checkPolicy(dir string) []Issue {
	rel := "policy.yaml"
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return []Issue{{File: rel, Code: "E_READ", Msg: "policy.yaml missing",
			Hint: "run `dossier init` or restore it — default is deny-all"}}
	}
	var p struct {
		Groups   map[string][]string `yaml:"groups"`
		Defaults map[string]any      `yaml:"defaults"`
		Rules    []struct {
			About string `yaml:"about"`
			To    string `yaml:"to"`
			Give  string `yaml:"give"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(b, &p); err != nil {
		return []Issue{{File: rel, Line: yamlLine(err), Code: "E_YAML",
			Msg: "invalid YAML: " + yamlMsg(err)}}
	}
	var issues []Issue
	for i, r := range p.Rules {
		if r.About == "" || r.To == "" {
			issues = append(issues, Issue{File: rel, Code: "E_POLICY",
				Msg: fmt.Sprintf("rule %d needs both `about` and `to`", i+1)})
		}
		if !giveVals[r.Give] {
			issues = append(issues, Issue{File: rel, Code: "E_POLICY",
				Msg:  fmt.Sprintf("rule %d: give: %q", i+1, r.Give),
				Hint: "one of: full, rough, yes-no, nothing"})
		}
	}
	return issues
}

func splitFrontmatter(b []byte) (fm, body []byte, ok bool) {
	if !bytes.HasPrefix(b, []byte("---\n")) && !bytes.HasPrefix(b, []byte("---\r\n")) {
		return nil, b, false
	}
	rest := b[bytes.IndexByte(b, '\n')+1:]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, b, false
	}
	after := rest[end+4:]
	if len(after) > 0 && after[0] == '\r' {
		after = after[1:]
	}
	if len(after) > 0 && after[0] != '\n' {
		return nil, b, false
	}
	if len(after) > 0 {
		after = after[1:]
	}
	return rest[:end], after, true
}

var lineRe = regexp.MustCompile(`line (\d+)`)

func yamlLine(err error) int {
	m := lineRe.FindStringSubmatch(err.Error())
	if len(m) == 2 {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		return n
	}
	return 0
}

func yamlMsg(err error) string {
	s := err.Error()
	s = strings.TrimPrefix(s, "yaml: ")
	return s
}
