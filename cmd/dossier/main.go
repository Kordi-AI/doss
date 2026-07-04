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
  doctor   verify install, vault, and agent wiring; --fix repairs
  answer   the disclosure gate (ships in P1)
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
	case "answer":
		fmt.Println("`dossier answer` (the disclosure gate) ships in P1.")
		fmt.Println("track: https://github.com/Kordi-AI/dossier/issues/2")
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
