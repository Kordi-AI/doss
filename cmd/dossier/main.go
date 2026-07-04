package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func usage() {
	fmt.Print(`dossier — a synced memory folder + a disclosure gate

usage: dossier <command> [flags]

  init     create your vault (~/.dossier), optionally wire up cloud sync
  connect  add Dossier instructions to every agent's global config (Claude Code, Codex, …)
  check    validate memory files (run after edits; errors are precise)
  sync     commit + pull + push the vault (refuses if check fails)
  status   vault health: counts, check result, sync state, tidy hints
  tidy     dirt report: stale facts, unconfirmed guesses, notes backlog
  doctor   verify install, vault, and agent wiring; --fix repairs
  uninstall  delete the local vault and unwire agents (safe when a cloud copy exists)
  hook     harness hook endpoint (post-edit, stop) — wired by connect
  answer   the outbound gate: what may be told to whom (--to, --about)
  log      the ledger: who was told what (--who filters)
  version  print version

vault location: $DOSSIER_HOME, default ~/.dossier
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
	case "status":
		err = cmdStatus(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "uninstall":
		err = cmdUninstall(os.Args[2:])
	case "tidy":
		err = cmdTidy(os.Args[2:])
	case "hook":
		err = cmdHook(os.Args[2:])
	case "answer":
		err = cmdAnswer(os.Args[2:])
	case "log":
		err = cmdLog(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("dossier", version)
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
