// Package check validates vault files. Deterministic, milliseconds, precise
// errors. This is the structural guarantee behind "the library is always
// clean": nothing unvalidated syncs or discloses.
package check

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
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
		"ledger": true, "ledger.log": true, "local": true, ".git": true, ".gitignore": true, ".index": true,
	}
	allowedKeys = map[string]bool{
		"source": true, "status": true, "confidence": true,
		"tags": true, "verify_by": true, "evidence": true, "rough": true,
	}
	sourceVals       = map[string]bool{"owner": true, "imported": true, "inferred": true, "peer": true}
	statusVals       = map[string]bool{"active": true, "suggested": true}
	disclosureLevels = map[string]bool{"no": true, "rough": true, "full": true}
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

	for _, area := range []string{"self", "peers", "notes"} {
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
	issues = append(issues, checkAccess(dir)...)
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
		case strings.HasPrefix(f, "self/") || strings.HasPrefix(f, "peers/") || strings.HasPrefix(f, "notes/"):
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				issues = append(issues, checkFile(dir, f)...)
			}
		case f == filepath.ToSlash(filepath.Join("local", "access.yaml")):
			issues = append(issues, checkAccess(dir)...)
		case strings.HasPrefix(f, "local/"),
			f == "SKILL.md", f == "README.md",
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
	if ext != ".md" {
		issues = append(issues, Issue{File: rel, Code: "E_EXT",
			Msg:  "content files must be markdown",
			Hint: "use .md under self/, peers/, and notes/; YAML is only for policy.yaml and local/access.yaml"})
		return issues
	}

	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return append(issues, Issue{File: rel, Code: "E_READ", Msg: err.Error()})
	}
	if len(b) > maxFileSize {
		issues = append(issues, Issue{File: rel, Code: "E_SIZE",
			Msg:  fmt.Sprintf("file is %dKB (limit 128KB)", len(b)/1024),
			Hint: "split by topic — one topic per file"})
	}

	fm, body, ok := splitFrontmatter(b)
	if ok {
		issues = append(issues, checkFrontmatter(rel, fm)...)
	} else {
		body = b
		if strings.HasPrefix(rel, "self/") {
			issues = append(issues, Issue{File: rel, Code: "E_ROUGH",
				Msg:  "self facts need a rough value",
				Hint: `add frontmatter with a shareable coarse version, e.g. rough: "Toronto"`})
		}
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
				Hint: "allowed: source, status, confidence, tags, verify_by, evidence, rough (x-* for extensions)"})
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
			// YAML parses a bare 2027-01-02 into time.Time; a quoted string
			// stays a string. Accept either, as long as it's a real date.
			switch d := v.(type) {
			case time.Time:
				// already a valid date
			case string:
				if _, err := time.Parse("2006-01-02", d); err != nil {
					issues = append(issues, Issue{File: rel, Code: "E_VALUE",
						Msg: "verify_by: " + d, Hint: "date as YYYY-MM-DD"})
				}
			default:
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: fmt.Sprintf("verify_by: %v", v), Hint: "date as YYYY-MM-DD"})
			}
		case "evidence":
			if _, ok := v.(string); !ok {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg: "evidence must be a string"})
			}
		case "rough":
			if s, ok := v.(string); !ok {
				issues = append(issues, Issue{File: rel, Code: "E_VALUE",
					Msg:  "rough must be a string",
					Hint: `the blurred version of this fact, e.g. rough: "Shanghai" for a street address`})
			} else if strings.TrimSpace(s) == "" {
				issues = append(issues, Issue{File: rel, Code: "E_ROUGH",
					Msg:  "rough cannot be empty",
					Hint: `write the safest shareable coarse version, e.g. rough: "Toronto"`})
			}
		}
	}
	if strings.HasPrefix(rel, "self/") {
		if _, ok := m["rough"]; !ok {
			issues = append(issues, Issue{File: rel, Code: "E_ROUGH",
				Msg:  "self facts need a rough value",
				Hint: `add rough: "..." to frontmatter so rough-level disclosure never requires model guessing`})
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
			Hint: "run `doss init` or restore it — default is deny-all"}}
	}
	var p struct {
		Groups map[string][]string `yaml:"groups"`
		CanSee map[string]any      `yaml:"can-see"`
	}
	if err := yaml.Unmarshal(b, &p); err != nil {
		return []Issue{{File: rel, Line: yamlLine(err), Code: "E_YAML",
			Msg: "invalid YAML: " + yamlMsg(err)}}
	}
	var issues []Issue
	// Every group granted access in can-see must be defined in groups, and each
	// grant must say whether that group gets no, rough, or full disclosure.
	for g, raw := range p.CanSee {
		if _, ok := p.Groups[g]; !ok {
			issues = append(issues, Issue{File: rel, Code: "E_POLICY",
				Msg:  fmt.Sprintf("can-see names group %q, which isn't defined under groups", g),
				Hint: "define the group's members, or fix the name"})
		}
		topics, ok := raw.(map[string]any)
		if !ok {
			issues = append(issues, Issue{File: rel, Code: "E_POLICY",
				Msg:  fmt.Sprintf("can-see.%s must map self topics to disclosure levels", g),
				Hint: "use topic: full|rough|no, e.g. friends: {profile/address: rough}"})
			continue
		}
		for topic, rawLevel := range topics {
			if !validPolicyTopic(topic) {
				issues = append(issues, Issue{File: rel, Code: "E_POLICY",
					Msg:  fmt.Sprintf("can-see.%s has invalid topic %q", g, topic),
					Hint: "use a relative path under self/ without the self/ prefix, e.g. profile/address"})
			}
			level, ok := rawLevel.(string)
			if !ok {
				issues = append(issues, Issue{File: rel, Code: "E_POLICY",
					Msg:  fmt.Sprintf("can-see.%s.%s level must be a string", g, topic),
					Hint: "level must be: no, rough, or full"})
			} else if !disclosureLevels[level] {
				issues = append(issues, Issue{File: rel, Code: "E_POLICY",
					Msg:  fmt.Sprintf("can-see.%s.%s has invalid level %q", g, topic, level),
					Hint: "level must be: no, rough, or full"})
			}
		}
	}
	return issues
}

func validPolicyTopic(topic string) bool {
	if topic == "" || strings.HasPrefix(topic, "/") || strings.HasPrefix(topic, "self/") {
		return false
	}
	clean := path.Clean(topic)
	if clean != topic || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return false
	}
	for _, part := range strings.Split(topic, "/") {
		if part == "" || !nameRe.MatchString(part) {
			return false
		}
	}
	return true
}

// policyGroups returns the set of group names defined in policy.yaml.
func policyGroups(dir string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(dir, "policy.yaml"))
	if err != nil {
		return out
	}
	var p struct {
		Groups map[string][]string `yaml:"groups"`
	}
	if yaml.Unmarshal(b, &p) == nil {
		for g := range p.Groups {
			out[g] = true
		}
	}
	return out
}

// checkAccess validates local/access.yaml (device-local; absent is fine).
// Per group, per folder → level in {no, read, full}; groups must exist in
// policy.yaml.
func checkAccess(dir string) []Issue {
	rel := filepath.Join("local", "access.yaml")
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return nil // device may not use it
	}
	var a struct {
		Grants map[string]map[string]string `yaml:"grants"`
	}
	if err := yaml.Unmarshal(b, &a); err != nil {
		return []Issue{{File: rel, Line: yamlLine(err), Code: "E_YAML",
			Msg: "invalid YAML: " + yamlMsg(err)}}
	}
	groups := policyGroups(dir)
	levels := map[string]bool{"no": true, "read": true, "full": true}
	var issues []Issue
	for g, folders := range a.Grants {
		if _, ok := groups[g]; !ok {
			issues = append(issues, Issue{File: rel, Code: "E_ACCESS",
				Msg:  fmt.Sprintf("group %q isn't defined in policy.yaml", g),
				Hint: "define it under groups: in policy.yaml, or fix the name"})
		}
		for folder, lvl := range folders {
			if !levels[lvl] {
				issues = append(issues, Issue{File: rel, Code: "E_ACCESS",
					Msg:  fmt.Sprintf("%s → %s: %q", g, folder, lvl),
					Hint: "level must be: no, read, or full"})
			}
		}
	}
	return issues
}

// Frontmatter parses a fact file's optional frontmatter for other packages
// (best effort — validation is checkFrontmatter's job).
func Frontmatter(b []byte) (map[string]any, string) {
	fm, body, ok := splitFrontmatter(b)
	if !ok {
		return nil, string(b)
	}
	var m map[string]any
	if yaml.Unmarshal(fm, &m) != nil {
		return nil, string(body)
	}
	return m, string(body)
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
