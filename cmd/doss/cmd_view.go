package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Kordi-AI/doss/internal/check"
	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

const viewMarkerFile = ".doss-view"

type viewPolicyFile struct {
	Groups map[string][]string          `yaml:"groups"`
	CanSee map[string]map[string]string `yaml:"can-see"`
}

type viewAccessFile struct {
	Grants map[string]map[string]string `yaml:"grants"`
}

type viewManifest struct {
	DossView          bool          `json:"doss_view"`
	Requester         string        `json:"requester"`
	GeneratedAt       string        `json:"generated_at"`
	ExpiresAt         string        `json:"expires_at"`
	SourceVaultCommit string        `json:"source_vault_commit"`
	PolicyHash        string        `json:"policy_hash"`
	LocalAccessHash   string        `json:"local_access_hash"`
	SelfTreeHash      string        `json:"self_tree_hash"`
	Blocked           []viewBlocked `json:"blocked,omitempty"`
}

type viewBlocked struct {
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
}

type viewAccessOut struct {
	Requester string             `json:"requester"`
	Folders   []viewAccessFolder `json:"folders"`
}

type viewAccessFolder struct {
	Path  string `json:"path"`
	Level string `json:"level"`
}

func cmdView(args []string) error {
	if len(args) > 0 && args[0] == "cleanup" {
		return cmdViewCleanup(args[1:])
	}

	fs := flag.NewFlagSet("view", flag.ExitOnError)
	requester := fs.String("for", "", "platform-verified requester id, e.g. kordi:pedro")
	out := fs.String("out", "", "fresh output directory for the requester-scoped view")
	ttl := fs.Duration("ttl", 30*time.Minute, "view lifetime, e.g. 30m")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *requester == "" || *out == "" {
		return fmt.Errorf("usage: doss view --for <verified-id> --out <dir>")
	}
	if !requesterIDRe.MatchString(*requester) {
		return fmt.Errorf("--for must be a platform-verified id in platform:id form, e.g. kordi:pedro")
	}
	if *ttl <= 0 {
		return fmt.Errorf("--ttl must be positive")
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}

	policy, policyHash, err := loadViewPolicy(d)
	if err != nil {
		return err
	}
	localAccessHash, err := hashOptionalFile(filepath.Join(d, "local", "access.yaml"))
	if err != nil {
		return err
	}
	selfHash, err := hashSelfTree(d)
	if err != nil {
		return err
	}
	groups := requesterGroups(policy, *requester)
	access, err := requesterAccess(d, *requester, groups)
	if err != nil {
		return err
	}
	absOut, err := prepareViewDir(d, *out)
	if err != nil {
		return err
	}

	blocked, facts, err := writeRequesterSelfView(d, absOut, policy, groups)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	manifest := viewManifest{
		DossView:          true,
		Requester:         *requester,
		GeneratedAt:       now.Format(time.RFC3339),
		ExpiresAt:         now.Add(*ttl).Format(time.RFC3339),
		SourceVaultCommit: sourceVaultCommit(d),
		PolicyHash:        policyHash,
		LocalAccessHash:   localAccessHash,
		SelfTreeHash:      selfHash,
		Blocked:           blocked,
	}
	if err := writeJSON(filepath.Join(absOut, "access.json"), access); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(absOut, "manifest.json"), manifest); err != nil {
		return err
	}
	if err := writeViewREADME(absOut); err != nil {
		return err
	}
	fmt.Printf("view ready: %s (%d fact(s), %d local grant(s))\n", absOut, facts, len(access.Folders))
	if len(blocked) > 0 {
		fmt.Printf("warning: %d fact(s) omitted because rough values are missing\n", len(blocked))
	}
	return nil
}

func cmdViewCleanup(args []string) error {
	fs := flag.NewFlagSet("view cleanup", flag.ExitOnError)
	root := fs.String("dir", os.TempDir(), "parent directory to scan for expired doss views")
	if err := fs.Parse(args); err != nil {
		return err
	}
	items, err := os.ReadDir(*root)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	removed := 0
	for _, it := range items {
		if !it.IsDir() {
			continue
		}
		dir := filepath.Join(*root, it.Name())
		m, err := readViewManifest(dir)
		if err != nil || !m.DossView || m.ExpiresAt == "" {
			continue
		}
		expires, err := time.Parse(time.RFC3339, m.ExpiresAt)
		if err != nil || now.Before(expires) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		removed++
	}
	fmt.Printf("removed %d expired view(s)\n", removed)
	return nil
}

func loadViewPolicy(dir string) (viewPolicyFile, string, error) {
	path := filepath.Join(dir, "policy.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return viewPolicyFile{}, "", err
	}
	var p viewPolicyFile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return viewPolicyFile{}, "", fmt.Errorf("policy.yaml: %w", err)
	}
	if p.Groups == nil {
		p.Groups = map[string][]string{}
	}
	if p.CanSee == nil {
		p.CanSee = map[string]map[string]string{}
	}
	return p, shaHex(b), nil
}

func requesterGroups(policy viewPolicyFile, requester string) []string {
	var groups []string
	for group, members := range policy.Groups {
		for _, member := range members {
			if member == requester {
				groups = append(groups, group)
				break
			}
		}
	}
	sort.Strings(groups)
	return groups
}

func writeRequesterSelfView(dir, out string, policy viewPolicyFile, groups []string) ([]viewBlocked, int, error) {
	var files []string
	root := filepath.Join(dir, "self")
	_ = filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") || filepath.Ext(e.Name()) != ".md" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)

	var blocked []viewBlocked
	written := 0
	for _, path := range files {
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		topic := strings.TrimSuffix(strings.TrimPrefix(rel, "self/"), ".md")
		level := effectiveDisclosureLevel(topic, groups, policy.CanSee)
		if level == "" || level == "no" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, 0, err
		}
		meta, body := check.Frontmatter(raw)
		if status, _ := meta["status"].(string); status == "suggested" {
			continue
		}
		var content string
		switch level {
		case "rough":
			rough, _ := meta["rough"].(string)
			if strings.TrimSpace(rough) == "" {
				blocked = append(blocked, viewBlocked{Topic: topic, Reason: "missing rough"})
				continue
			}
			content = strings.TrimSpace(rough) + "\n"
		case "full":
			content = body
		default:
			continue
		}
		dst := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, 0, err
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			return nil, 0, err
		}
		written++
	}
	return blocked, written, nil
}

func effectiveDisclosureLevel(topic string, groups []string, canSee map[string]map[string]string) string {
	best := "no"
	for _, group := range groups {
		level := groupDisclosureLevel(topic, canSee[group])
		if disclosureRank(level) > disclosureRank(best) {
			best = level
		}
	}
	return best
}

func groupDisclosureLevel(topic string, rules map[string]string) string {
	bestLevel := "no"
	bestLen := -1
	for ruleTopic, level := range rules {
		if level != "no" && level != "rough" && level != "full" {
			continue
		}
		if topic == ruleTopic || strings.HasPrefix(topic, ruleTopic+"/") {
			if len(ruleTopic) > bestLen {
				bestLen = len(ruleTopic)
				bestLevel = level
			}
		}
	}
	return bestLevel
}

func disclosureRank(level string) int {
	switch level {
	case "full":
		return 2
	case "rough":
		return 1
	default:
		return 0
	}
}

func requesterAccess(dir, requester string, groups []string) (viewAccessOut, error) {
	out := viewAccessOut{Requester: requester}
	path := filepath.Join(dir, "local", "access.yaml")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var access viewAccessFile
	if err := yaml.Unmarshal(b, &access); err != nil {
		return out, fmt.Errorf("local/access.yaml: %w", err)
	}
	levels := map[string]string{}
	for _, group := range groups {
		for folder, level := range access.Grants[group] {
			if level != "read" && level != "full" {
				continue
			}
			if accessRank(level) > accessRank(levels[folder]) {
				levels[folder] = level
			}
		}
	}
	var folders []string
	for folder := range levels {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	for _, folder := range folders {
		out.Folders = append(out.Folders, viewAccessFolder{Path: folder, Level: levels[folder]})
	}
	return out, nil
}

func accessRank(level string) int {
	if level == "full" {
		return 2
	}
	if level == "read" {
		return 1
	}
	return 0
}

func prepareViewDir(vaultDir, out string) (string, error) {
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("--out is required")
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	absVault, err := filepath.Abs(vaultDir)
	if err != nil {
		return "", err
	}
	if absOut == string(filepath.Separator) || absOut == absVault || strings.HasPrefix(absOut, absVault+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing unsafe view output directory: %s", absOut)
	}
	if st, err := os.Stat(absOut); err == nil {
		if !st.IsDir() {
			return "", fmt.Errorf("--out exists and is not a directory: %s", absOut)
		}
		if !isDossViewDir(absOut) {
			return "", fmt.Errorf("--out exists and is not a doss-owned view directory: %s", absOut)
		}
		if err := os.RemoveAll(absOut); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(absOut, viewMarkerFile), []byte("doss view\n"), 0o644); err != nil {
		return "", err
	}
	return absOut, nil
}

func isDossViewDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, viewMarkerFile)); err == nil {
		return true
	}
	m, err := readViewManifest(dir)
	return err == nil && m.DossView
}

func readViewManifest(dir string) (viewManifest, error) {
	var m viewManifest
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func writeViewREADME(out string) error {
	body := `# Doss Requester View

Use this generated view for the current external requester task. Do not read the raw vault unless the owner explicitly switches you back into owner/maintenance mode.

- ` + "`self/`" + ` contains only owner facts this requester may receive.
- ` + "`access.json`" + ` lists local folders the host may expose for this requester.
- ` + "`manifest.json`" + ` records freshness metadata; expired views must be regenerated.
`
	return os.WriteFile(filepath.Join(out, "README.md"), []byte(body), 0o644)
}

func sourceVaultCommit(dir string) string {
	out, err := gitx.Run(dir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func hashOptionalFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return shaHex(b), nil
}

func hashSelfTree(dir string) (string, error) {
	var rels []string
	root := filepath.Join(dir, "self")
	_ = filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(rels)
	h := sha256.New()
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func shaHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
