// Package vault knows where the doss lives and how to scaffold one.
package vault

import (
	"crypto/rand"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Kordi-AI/doss/internal/gitx"
)

//go:embed templates
var tmpl embed.FS

const (
	InstructionFile           = "INSTRUCTION.md"
	ContentInstructionFile    = "CONTENT.md"
	DisclosureInstructionFile = "DISCLOSURE.md"
)

// Device is one synced device registration record. One file per device keeps
// multi-device syncs from fighting over a single registry file.
type Device struct {
	ID                   string `yaml:"id"`
	Label                string `yaml:"label"`
	Status               string `yaml:"status"`
	RegisteredAt         string `yaml:"registered_at"`
	UnregisteredAt       string `yaml:"unregistered_at"`
	GitHubRepo           string `yaml:"github_repo,omitempty"`
	DeployKeyID          int64  `yaml:"deploy_key_id,omitempty"`
	DeployKeyTitle       string `yaml:"deploy_key_title,omitempty"`
	DeployKeyFingerprint string `yaml:"deploy_key_fingerprint,omitempty"`
}

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

// InstructionPath returns the agent instruction file to reference.
func InstructionPath(dir string) string {
	return filepath.Join(dir, InstructionFile)
}

// EnsureInstruction seeds current instruction files for vaults created before
// the split instruction layout existed. It never overwrites owner-edited files.
func EnsureInstruction(dir string) error {
	files := map[string]string{
		InstructionFile:           "templates/INSTRUCTION.md",
		ContentInstructionFile:    "templates/CONTENT.md",
		DisclosureInstructionFile: "templates/DISCLOSURE.md",
	}
	for dst, src := range files {
		if err := seedTemplateFile(dir, dst, src); err != nil {
			return err
		}
	}
	return nil
}

// Scaffold creates the vault layout. It never overwrites existing files.
func Scaffold(dir string) error {
	for _, sub := range []string{"self", "peers", "notes", "devices"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
		if sub == "devices" {
			continue
		}
		keep := filepath.Join(dir, sub, ".gitkeep")
		if _, err := os.Stat(keep); os.IsNotExist(err) {
			if err := os.WriteFile(keep, nil, 0o644); err != nil {
				return err
			}
		}
	}
	files := map[string]string{
		InstructionFile:           "templates/INSTRUCTION.md",
		ContentInstructionFile:    "templates/CONTENT.md",
		DisclosureInstructionFile: "templates/DISCLOSURE.md",
		"policy.yaml":             "templates/policy.yaml",
		"README.md":               "templates/vault-readme.md",
		".gitignore":              "templates/vault-gitignore",
	}
	for dst, src := range files {
		if err := seedTemplateFile(dir, dst, src); err != nil {
			return err
		}
	}
	return EnsureLocal(dir)
}

// DeviceID returns a stable, machine-local id stored in this vault's local git
// config. The id itself is safe to sync once written into devices/<id>.yaml.
func DeviceID(dir string) string {
	if out, err := gitx.Run(dir, "config", "--local", "--get", "doss.device"); err == nil {
		if id := strings.TrimSpace(out); id != "" {
			return id
		}
	}
	host, _ := os.Hostname()
	host = strings.Trim(deviceSanitize.ReplaceAllString(strings.ToLower(host), "-"), "-")
	host, _, _ = strings.Cut(host, ".")
	if len(host) > 16 {
		host = host[:16]
	}
	if host == "" {
		host = "device"
	}
	id := fmt.Sprintf("%s-%s", host, randHex(2))
	_, _ = gitx.Run(dir, "config", "--local", "doss.device", id)
	return id
}

// RegisterDevice makes the current device active in the synced registry.
func RegisterDevice(dir string) (Device, error) {
	id := DeviceID(dir)
	dev, err := readDeviceFile(dir, id)
	if err != nil && !os.IsNotExist(err) {
		return Device{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if os.IsNotExist(err) {
		dev = Device{
			ID:           id,
			Label:        defaultDeviceLabel(),
			Status:       "active",
			RegisteredAt: now,
		}
	} else {
		if dev.ID == "" {
			dev.ID = id
		}
		if dev.Label == "" {
			dev.Label = defaultDeviceLabel()
		}
		if dev.RegisteredAt == "" {
			dev.RegisteredAt = now
		}
		dev.Status = "active"
		dev.UnregisteredAt = ""
	}
	return dev, writeDeviceFile(dir, dev)
}

// UnregisterDevice marks one device as no longer active, preserving its record
// for audit and ledger interpretation.
func UnregisterDevice(dir, id string) (Device, error) {
	dev, err := readDeviceFile(dir, id)
	if err != nil {
		if !os.IsNotExist(err) {
			return Device{}, err
		}
		dev = Device{
			ID:           id,
			Label:        defaultDeviceLabel(),
			RegisteredAt: time.Now().UTC().Format(time.RFC3339),
		}
	}
	dev.Status = "unregistered"
	dev.UnregisteredAt = time.Now().UTC().Format(time.RFC3339)
	return dev, writeDeviceFile(dir, dev)
}

// DeviceRecord returns one synced device record by id.
func DeviceRecord(dir, id string) (Device, error) {
	return readDeviceFile(dir, id)
}

// SetDeviceDeployKey records the GitHub deploy key that gates this device's
// cloud sync access.
func SetDeviceDeployKey(dir, id, repo, title, fingerprint string, keyID int64) (Device, error) {
	dev, err := readDeviceFile(dir, id)
	if err != nil {
		return Device{}, err
	}
	dev.GitHubRepo = repo
	dev.DeployKeyID = keyID
	dev.DeployKeyTitle = title
	dev.DeployKeyFingerprint = fingerprint
	return dev, writeDeviceFile(dir, dev)
}

// Devices returns every synced device record sorted by id.
func Devices(dir string) ([]Device, error) {
	root := filepath.Join(dir, "devices")
	items, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Device
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(it.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(it.Name(), ".yaml")
		dev, err := readDeviceFile(dir, id)
		if err != nil {
			return nil, err
		}
		out = append(out, dev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func DeviceFile(id string) string {
	return filepath.Join("devices", id+".yaml")
}

func readDeviceFile(dir, id string) (Device, error) {
	b, err := os.ReadFile(filepath.Join(dir, DeviceFile(id)))
	if err != nil {
		return Device{}, err
	}
	var dev Device
	if err := yaml.Unmarshal(b, &dev); err != nil {
		return Device{}, err
	}
	return dev, nil
}

func writeDeviceFile(dir string, dev Device) error {
	if err := os.MkdirAll(filepath.Join(dir, "devices"), 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(dev)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, DeviceFile(dev.ID)), out, 0o644)
}

var deviceSanitize = regexp.MustCompile(`[^a-z0-9-]+`)

func defaultDeviceLabel() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "device"
	}
	return strings.TrimSpace(host)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "0000"
	}
	return fmt.Sprintf("%x", b)
}

func seedTemplateFile(dir, dst, src string) error {
	path := filepath.Join(dir, dst)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	b, err := fs.ReadFile(tmpl, src)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// EnsureLocal creates the device-only local/ area (gitignored, never synced)
// with a starter access.yaml. Called on both `init` and `init --from`, since a
// cloned vault never carries another device's local/ files.
func EnsureLocal(dir string) error {
	local := filepath.Join(dir, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		return err
	}
	access := filepath.Join(local, "access.yaml")
	if _, err := os.Stat(access); err == nil {
		return nil
	}
	b, err := fs.ReadFile(tmpl, "templates/access.yaml")
	if err != nil {
		return err
	}
	return os.WriteFile(access, b, 0o644)
}
