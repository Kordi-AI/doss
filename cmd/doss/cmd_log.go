package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

type ledgerEntry struct {
	Ts, To, Purpose, Question, Topic, Give, Outcome string
	device                                          string
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	who := fs.String("who", "", "only entries for this requester")
	device := fs.String("device", "", "only entries recorded on this device")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}

	entries, devices := readLedger(d)
	if len(entries) == 0 {
		fmt.Println("ledger is empty — nothing has ever gone out")
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ts < entries[j].Ts })

	shown, shared := 0, 0
	for _, e := range entries {
		if *who != "" && e.To != *who {
			continue
		}
		if *device != "" && e.device != *device {
			continue
		}
		shown++
		if e.Outcome == "full" || e.Outcome == "rough" {
			shared++
		}
		fmt.Printf("%s  %-12s %-16s %-24s %-7s %s\n",
			e.Ts, e.device, e.To, e.Topic, e.Give, e.Outcome)
	}
	if shown == 0 {
		fmt.Println("no matching entries")
		return nil
	}
	fmt.Printf("\n%d request(s), %d actually shared something · across %d device(s): %s\n",
		shown, shared, len(devices), strings.Join(devices, ", "))
	return nil
}

// readLedger merges every device's ledger file (plus a legacy single file if
// present) into one time-ordered list.
func readLedger(d string) (entries []ledgerEntry, devices []string) {
	seen := map[string]bool{}
	add := func(path, device string) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		if !seen[device] && device != "" {
			seen[device] = true
			devices = append(devices, device)
		}
		for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if ln == "" {
				continue
			}
			var e ledgerEntry
			if json.Unmarshal([]byte(ln), &e) == nil {
				e.device = device
				entries = append(entries, e)
			}
		}
	}

	dir := filepath.Join(d, "ledger")
	if items, err := os.ReadDir(dir); err == nil {
		for _, it := range items {
			if it.IsDir() || !strings.HasSuffix(it.Name(), ".log") {
				continue
			}
			add(filepath.Join(dir, it.Name()), strings.TrimSuffix(it.Name(), ".log"))
		}
	}
	// Back-compat: the old single ledger.log at the vault root.
	add(filepath.Join(d, "ledger.log"), "legacy")
	sort.Strings(devices)
	return entries, devices
}

var deviceSanitize = regexp.MustCompile(`[^a-z0-9-]+`)

// deviceID is a stable, machine-local id for this device, stored in the
// vault's local git config (which never syncs). Hostname makes it
// recognizable; a random suffix guarantees uniqueness across same-named hosts.
func deviceID(d string) string {
	if out, err := gitx.Run(d, "config", "--local", "--get", "doss.device"); err == nil {
		if id := strings.TrimSpace(out); id != "" {
			return id
		}
	}
	host, _ := os.Hostname()
	host = strings.Trim(deviceSanitize.ReplaceAllString(strings.ToLower(host), "-"), "-")
	host, _, _ = strings.Cut(host, ".") // drop any domain suffix
	if len(host) > 16 {
		host = host[:16]
	}
	if host == "" {
		host = "device"
	}
	id := fmt.Sprintf("%s-%s", host, randHex(2))
	_, _ = gitx.Run(d, "config", "--local", "doss.device", id)
	return id
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "0000"
	}
	return fmt.Sprintf("%x", b)
}
