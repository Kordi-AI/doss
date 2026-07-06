package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Kordi-AI/doss/internal/vault"
)

type ledgerEntry struct {
	Ts, To, Shared, Level, Note string
	device                      string
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	record := fs.Bool("record", false, "append a disclosure to the ledger (use after telling someone about the owner)")
	to := fs.String("to", "", "with --record: who was told (platform-verified id, e.g. kordi:pedro)")
	shared := fs.String("shared", "", "with --record: what was shared, e.g. profile/address")
	level := fs.String("level", "", "with --record: disclosure level, rough or full")
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
		if *to == "" || *shared == "" || *level == "" {
			return fmt.Errorf("--record needs --to <verified-id>, --shared <topic>, and --level <rough|full>")
		}
		if !requesterIDRe.MatchString(*to) {
			return fmt.Errorf("--to must be a platform-verified id in platform:id form, e.g. kordi:pedro")
		}
		if !validSharedTopic(*shared) {
			return fmt.Errorf("--shared must be a topic path under self/ without the self/ prefix, e.g. profile/address")
		}
		if *level != "rough" && *level != "full" {
			return fmt.Errorf("--level must be rough or full")
		}
		return writeLedger(d, *to, *shared, *level, *note)
	}

	entries, devices, err := readLedger(d)
	if err != nil {
		return err
	}
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
		line := fmt.Sprintf("%s  %-12s %-16s %-5s %s", e.Ts, e.device, e.To, e.Level, e.Shared)
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
func writeLedger(d, to, shared, level, note string) error {
	entry := map[string]string{
		"ts": time.Now().Format(time.RFC3339), "to": to, "shared": shared, "level": level, "note": note,
	}
	b, _ := json.Marshal(entry)
	dir := filepath.Join(d, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, vault.DeviceID(d)+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	fmt.Printf("logged: %s (%s) → %s\n", shared, level, to)
	return nil
}

// readLedger merges every device's ledger file into one time-ordered list.
func readLedger(d string) (entries []ledgerEntry, devices []string, err error) {
	seen := map[string]bool{}
	add := func(path, device string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !seen[device] && device != "" {
			seen[device] = true
			devices = append(devices, device)
		}
		for i, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if ln == "" {
				continue
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(ln), &raw); err != nil || raw == nil {
				return fmt.Errorf("%s:%d malformed ledger JSON — run `doss check` for details", path, i+1)
			}
			var e ledgerEntry
			if err := json.Unmarshal([]byte(ln), &e); err != nil {
				return fmt.Errorf("%s:%d malformed ledger JSON — run `doss check` for details", path, i+1)
			}
			e.device = device
			entries = append(entries, e)
		}
		return nil
	}

	dir := filepath.Join(d, "ledger")
	if _, err := os.Stat(filepath.Join(d, "ledger.log")); err == nil {
		if err := add(filepath.Join(d, "ledger.log"), "legacy"); err != nil {
			return nil, nil, err
		}
	}
	if items, err := os.ReadDir(dir); err == nil {
		for _, it := range items {
			if it.IsDir() || !strings.HasSuffix(it.Name(), ".log") {
				continue
			}
			if err := add(filepath.Join(dir, it.Name()), strings.TrimSuffix(it.Name(), ".log")); err != nil {
				return nil, nil, err
			}
		}
	}
	sort.Strings(devices)
	return entries, devices, nil
}

var requesterIDRe = regexp.MustCompile(`^[a-z][a-z0-9._-]*:[A-Za-z0-9][A-Za-z0-9._@-]*$`)
var topicPartRe = regexp.MustCompile(`^[a-z0-9._-]+$`)

func validSharedTopic(topic string) bool {
	if topic == "" || strings.HasPrefix(topic, "/") || strings.HasPrefix(topic, "self/") {
		return false
	}
	clean := path.Clean(topic)
	if clean != topic || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return false
	}
	for _, part := range strings.Split(topic, "/") {
		if part == "" || !topicPartRe.MatchString(part) {
			return false
		}
	}
	return true
}
