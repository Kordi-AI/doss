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
	"time"

	"github.com/Kordi-AI/doss/internal/gitx"
	"github.com/Kordi-AI/doss/internal/vault"
)

type ledgerEntry struct {
	Ts, To, Shared, Note string
	device               string
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	record := fs.Bool("record", false, "append a disclosure to the ledger (use after telling someone about the owner)")
	to := fs.String("to", "", "with --record: who was told (platform-verified id, e.g. kordi:pedro)")
	shared := fs.String("shared", "", "with --record: what was shared, e.g. profile.dietary")
	note := fs.String("note", "", "with --record: why / context (optional)")
	who := fs.String("who", "", "when reading: only entries for this requester")
	device := fs.String("device", "", "when reading: only entries recorded on this device")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}

	if *record {
		if *to == "" || *shared == "" {
			return fmt.Errorf("--record needs --to <who> and --shared <what>")
		}
		return writeLedger(d, *to, *shared, *note)
	}

	entries, devices := readLedger(d)
	if len(entries) == 0 {
		fmt.Println("ledger is empty — nothing has ever been disclosed")
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ts < entries[j].Ts })

	shown := 0
	for _, e := range entries {
		if *who != "" && e.To != *who {
			continue
		}
		if *device != "" && e.device != *device {
			continue
		}
		shown++
		line := fmt.Sprintf("%s  %-12s %-16s %s", e.Ts, e.device, e.To, e.Shared)
		if e.Note != "" {
			line += "  (" + e.Note + ")"
		}
		fmt.Println(line)
	}
	if shown == 0 {
		fmt.Println("no matching entries")
		return nil
	}
	fmt.Printf("\n%d disclosure(s) · across %d device(s): %s\n", shown, len(devices), strings.Join(devices, ", "))
	return nil
}

// writeLedger appends one disclosure to this device's ledger file. One file
// per device (ledger/<id>.log) so syncing never conflicts.
func writeLedger(d, to, shared, note string) error {
	entry := map[string]string{
		"ts": time.Now().Format(time.RFC3339), "to": to, "shared": shared, "note": note,
	}
	b, _ := json.Marshal(entry)
	dir := filepath.Join(d, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, deviceID(d)+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	fmt.Printf("logged: %s → %s\n", shared, to)
	return nil
}

// readLedger merges every device's ledger file into one time-ordered list.
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
	sort.Strings(devices)
	return entries, devices
}

var deviceSanitize = regexp.MustCompile(`[^a-z0-9-]+`)

// deviceID is a stable, machine-local id stored in the vault's local git
// config (never syncs). Hostname makes it recognizable; a random suffix
// guarantees uniqueness across same-named hosts.
func deviceID(d string) string {
	if out, err := gitx.Run(d, "config", "--local", "--get", "doss.device"); err == nil {
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
