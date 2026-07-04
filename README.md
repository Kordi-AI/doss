# Dossier

> A synced memory folder, plus a gate that only wakes when information leaves.
> Your agent's memory, your rules.

For your agent: long-term memory as plain md/yaml files — remember = write a file, recall = read a file. Zero ceremony.
For everyone else: when someone asks about you, a local program consults `policy.yaml` and returns one of three things — **cleared text / nothing / let me ask the owner** — and writes a ledger entry.
The other side installs nothing. Useful at n=1 (multi-device memory + disclosure discipline); upgrades automatically when both sides run it.

Status: v0 in development · [Design](docs/design/plan-v0.1.md) · [Roadmap issues](https://github.com/Kordi-AI/dossier/issues)

## Install

**Option 1 — do it yourself**

```sh
git clone https://github.com/Kordi-AI/dossier && cd dossier && ./install.sh
dossier init            # guided setup — just answer the questions
```

Run in a terminal, `dossier init` walks you through it: new vault or connect to an existing cloud one · the name/email your vault commits as · whether to back it up to a private GitHub repo (it takes a token if `gh` isn't logged in).

**Option 2 — paste this to your agent**

> Please install Dossier and set up my memory vault:
> 1. `git clone https://github.com/Kordi-AI/dossier && cd dossier && ./install.sh`
> 2. **Ask me** which name and email my vault's git commits should use, then run `dossier init --github --git-name "…" --git-email "…"` (drop `--github` if I don't have gh or don't want a cloud copy; use `--from owner/repo` if I already have a cloud vault)
> 3. Check the `connect` output it prints: if your own tool is NOT listed there, wire yourself with `dossier connect --file <the instruction file you load every session>`.
> 4. Read `~/.dossier/SKILL.md` and manage my memory by its rules from now on.

**Second device?** Run `dossier init` and pick *"connect to my existing cloud vault"* — give it your repo (e.g. `ShenzheZhu/my-dossier`) and every device shares one memory, kept aligned by `dossier sync`. Agents can skip the questions with `dossier init --from owner/repo`.

`init` automatically runs `dossier connect`, which drops a small managed section into each installed agent's **always-loaded global instruction file** — `~/.claude/CLAUDE.md` (Claude Code), `~/.codex/AGENTS.md` (Codex), `~/.gemini/GEMINI.md` (Gemini CLI), OpenClaw's workspace `AGENTS.md`, Windsurf's global rules. Agents we've never heard of work too: `dossier connect --file <the instruction file it always loads>`. The section carries the vault path and the non-negotiables and is injected deterministically into every session of every project. Whichever agent sets Dossier up, every other agent on the machine discovers it in its next session.

Installed a new agent tool later? Rerun `dossier connect`. Verify anytime with `dossier doctor` (`--fix` repairs). Undo with `dossier connect --remove`.

## Usage

After setup, an agent only needs four habits (details in the generated `~/.dossier/SKILL.md`):

| When | Do |
| --- | --- |
| Learned something durable | Write a small file under `self/` — the path is the topic |
| Need to recall | Just `ls` / `grep` / read files |
| Finished editing | `dossier check --changed` (errors are precise; fix and rerun) |
| Wrapping up | `dossier sync` (unvalidated content never syncs) |

## Commands

| Command | What it does |
| --- | --- |
| `dossier init` | Create a vault, or `--from owner/repo` to attach another device |
| `dossier connect` | Wire the vault into every installed agent (auto-run by init) |
| `dossier check` | Validate memory files; bad writes bounce with precise errors |
| `dossier sync` | Commit + pull + push; only validated state ever leaves |
| `dossier answer` | The outbound gate — what may be told to whom (`--to`, `--about`) |
| `dossier log` | Who was told what, merged across all your devices |
| `dossier doctor` | Full health: vault, sync, wiring, hooks; `--fix` repairs (alias: `status`) |
| `dossier tidy` | What needs your judgment: stale facts, unconfirmed guesses, notes backlog |
| `dossier uninstall` | Delete the local vault and unwire agents (safe when a cloud copy exists) |

## Architecture

![Dossier architecture](docs/architecture.png)

## Docs

- [How it works](docs/how-it-works.md) — the detailed mechanics: commands, hooks, wiring, sync, failure modes
- [Plan v0.1](docs/design/plan-v0.1.md) — all design decisions, with a decision log
- [Memory compatibility draft](docs/design/memory-adapters.md) — we don't compete with memory systems; we make them governable
- [Prior research](docs/design/archive/)
