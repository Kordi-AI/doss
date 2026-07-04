package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kordi-AI/dossier/internal/check"
	"github.com/Kordi-AI/dossier/internal/policy"
	"github.com/Kordi-AI/dossier/internal/vault"
)

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// The outward text for deny, no-data, unconfirmed, and not-yet-supported is
// identical on purpose: a refusal must not confirm that something exists.
const nothingToShare = "nothing to share"

func cmdAnswer(args []string) error {
	fs := flag.NewFlagSet("answer", flag.ExitOnError)
	to := fs.String("to", "", "who is asking, e.g. kordi:pedro (required)")
	purpose := fs.String("purpose", "", "why they ask (recorded in the ledger)")
	var about stringList
	fs.Var(&about, "about", "topic the question maps to, e.g. profile.dietary (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	question := strings.Join(fs.Args(), " ")

	if *to == "" {
		return fmt.Errorf("--to is required: the gate must know who is asking")
	}
	if len(about) == 0 {
		return fmt.Errorf("--about is required: map the question to topics first (e.g. --about profile.dietary)")
	}
	d, err := vault.MustExist()
	if err != nil {
		return err
	}
	pol, err := policy.Load(d)
	if err != nil {
		return err // fail closed: no policy, no disclosure
	}

	fmt.Printf("reply to %s using ONLY the lines below — do not add, infer, or confirm anything else about the owner:\n\n", *to)
	for _, raw := range about {
		topic := strings.ToLower(strings.TrimSpace(raw))
		give := pol.Give(*to, topic)
		text, outcome := resolveTopic(d, topic, give)
		writeLedger(d, *to, *purpose, question, topic, give, outcome)
		if strings.Contains(text, "\n") {
			fmt.Printf("%s:\n", topic)
			for _, ln := range strings.Split(text, "\n") {
				fmt.Println("  " + ln)
			}
		} else {
			fmt.Printf("%s: %s\n", topic, text)
		}
	}
	return nil
}

// resolveTopic turns (topic, give-level) into the outward text plus an
// owner-side outcome for the ledger. Topics live under self/ only.
func resolveTopic(d, topic, give string) (text, outcome string) {
	if give == "nothing" {
		return nothingToShare, "denied"
	}
	path := filepath.Join(d, "self", filepath.FromSlash(strings.ReplaceAll(topic, ".", "/"))+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nothingToShare, "no-data"
	}
	meta, body := check.Frontmatter(raw)
	if st, _ := meta["status"].(string); st == "suggested" {
		return nothingToShare, "unconfirmed-withheld"
	}
	switch give {
	case "full":
		return strings.TrimSpace(body), "full"
	case "rough":
		if r, ok := meta["rough"].(string); ok && strings.TrimSpace(r) != "" {
			return strings.TrimSpace(r), "rough"
		}
		fmt.Fprintf(os.Stderr, "owner-side note: policy allows rough for %s but the file has no `rough:` field — add one to enable it\n", topic)
		return nothingToShare, "rough-unavailable"
	case "yes-no":
		fmt.Fprintf(os.Stderr, "owner-side note: yes-no needs the in-gate evaluator (issue #2); withholding %s for now\n", topic)
		return nothingToShare, "yes-no-pending"
	default:
		fmt.Fprintf(os.Stderr, "owner-side note: unknown give level %q for %s; treating as nothing\n", give, topic)
		return nothingToShare, "denied"
	}
}

func writeLedger(d, to, purpose, question, topic, give, outcome string) {
	entry := map[string]string{
		"ts": time.Now().Format(time.RFC3339), "to": to, "purpose": purpose,
		"question": question, "topic": topic, "give": give, "outcome": outcome,
	}
	b, _ := json.Marshal(entry)
	f, err := os.OpenFile(filepath.Join(d, "ledger.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "owner-side note: could not write the ledger:", err)
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
