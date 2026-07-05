package main

import (
	"fmt"
	"os"
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func usage() {
	fmt.Print(`doss — a synced memory folder your agents read and write as plain files

usage: doss <command> [flags]

  init     create your vault (~/.doss), optionally wire up cloud sync
  connect  add Doss instructions to every agent's global config (Claude Code, Codex, …)
  check    validate memory files (run after edits; errors are precise)
  sync     commit + pull + push the vault (refuses if check fails)
  doctor   full health: vault stats, sync, wiring, hooks, tidy hints; --fix repairs (alias: status)
  tidy     dirt report: stale facts, unconfirmed guesses, notes backlog
  uninstall  delete the local vault and unwire agents (safe when a cloud copy exists)
  hook     harness hook endpoint (post-edit, stop) — wired by connect
  log      the disclosure ledger: --record to note a disclosure, plain to read it
  version  print version

vault location: $DOSS_HOME, default ~/.doss
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "connect":
		err = cmdConnect(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
	case "sync":
		err = cmdSync(os.Args[2:])
	case "doctor", "status":
		err = cmdDoctor(os.Args[2:])
	case "uninstall":
		err = cmdUninstall(os.Args[2:])
	case "tidy":
		err = cmdTidy(os.Args[2:])
	case "hook":
		err = cmdHook(os.Args[2:])
	case "log":
		err = cmdLog(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("doss", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
