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
dossier init --github   # creates your own private GitHub repo as the cloud copy; drop --github to stay local
```

**Option 2 — paste this to your agent**

> Please install Dossier and set up my memory vault:
> 1. `git clone https://github.com/Kordi-AI/dossier && cd dossier && ./install.sh`
> 2. Run `dossier init --github` (use plain `dossier init` if I don't have gh or don't want a cloud copy)
> 3. Read `~/.dossier/SKILL.md` and manage my memory by its rules from now on.

## Usage

After setup, an agent only needs four habits (details in the generated `~/.dossier/SKILL.md`):

| When | Do |
| --- | --- |
| Learned something durable | Write a small file under `self/` — the path is the topic |
| Need to recall | Just `ls` / `grep` / read files |
| Finished editing | `dossier check --changed` (errors are precise; fix and rerun) |
| Wrapping up | `dossier sync` (unvalidated content never syncs) |

## Architecture

![Dossier architecture](docs/architecture.png)

## Docs

- [Plan v0.1](docs/design/plan-v0.1.md) — all design decisions, with a decision log
- [Memory compatibility draft](docs/design/memory-adapters.md) — we don't compete with memory systems; we make them governable
- [Prior research](docs/design/archive/)
