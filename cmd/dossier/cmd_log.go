package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kordi-AI/dossier/internal/vault"
)

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	who := fs.String("who", "", "only entries for this requester")
	if err := fs.Parse(args); err != nil {
		return err
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(d, "ledger.log"))
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		fmt.Println("ledger is empty — nothing has ever gone out")
		return nil
	}

	shown, shared := 0, 0
	for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var e struct {
			Ts, To, Purpose, Question, Topic, Give, Outcome string
		}
		if json.Unmarshal([]byte(ln), &e) != nil {
			continue
		}
		if *who != "" && e.To != *who {
			continue
		}
		shown++
		if e.Outcome == "full" || e.Outcome == "rough" {
			shared++
		}
		fmt.Printf("%s  %-16s %-26s %-7s %s\n", e.Ts, e.To, e.Topic, e.Give, e.Outcome)
	}
	if shown == 0 {
		fmt.Printf("no entries for %s\n", *who)
		return nil
	}
	fmt.Printf("\n%d request(s), %d actually shared something\n", shown, shared)
	return nil
}
