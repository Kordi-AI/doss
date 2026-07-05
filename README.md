# doss

[![Documentation](https://img.shields.io/badge/Documentation-doss-000?style=flat&logo=vercel&logoColor=white)](https://doss-docs-git-doss-docs-shenzhe-zhus-projects.vercel.app)
[![Release](https://img.shields.io/github/v/release/Kordi-AI/doss?style=flat&color=000)](https://github.com/Kordi-AI/doss/releases)

> Plain-file memory for AI agents, synced with git, with default-deny rules for what can leave.

Doss creates a vault at `~/.doss`. Agents remember by editing files and recall by
reading or searching those files. The CLI handles setup, validation, sync, agent
wiring, and disclosure logs.

## At a Glance

| Concern | Where it lives | What it does |
| --- | --- | --- |
| Memory | `~/.doss/self/`, `peers/`, `notes/` | Stores facts and notes as markdown/yaml files. |
| Agent rules | `~/.doss/SKILL.md` | Tells agents how to read, write, check, sync, and disclose. |
| Disclosure policy | `~/.doss/policy.yaml` | Maps groups of people to the owner folders they may see. Default: deny. |
| Local access policy | `~/.doss/local/access.yaml` | Device-local rules for what non-owners may ask this machine to read or edit. |
| Audit ledger | `doss log` | Records who was told what about the owner. |
| Sync | `doss sync` | Validates, commits, pulls, and pushes the vault. |

## Quick Start

```sh
curl -fsSL https://raw.githubusercontent.com/Kordi-AI/doss/main/install.sh | sh
doss init
```

`doss init` walks through a new vault or an existing cloud vault, commit identity,
optional private GitHub backup, and agent wiring.

Common shortcuts:

```sh
doss init --github              # create a private GitHub-backed vault
doss init --from owner/repo     # attach this device to an existing vault
doss doctor                     # check vault health, sync, and agent wiring
```

Prebuilt binaries are available for macOS/Linux on amd64 and arm64.

## Agent Workflow

After setup, agents operate on the vault as normal files:

| Situation | Agent action |
| --- | --- |
| Learned a durable owner fact | Update one topic file under `self/`. |
| Needs owner context | Read or search the vault with normal file tools. |
| Edited vault files | Run `doss check --changed` and fix precise errors. |
| Finished a batch/session | Run `doss sync`. |
| Disclosed owner info | Run `doss log --record --to <who> --shared <topic>`. |

For agent-assisted install and multi-device setup, see the
[Getting Started docs](https://doss-docs-git-doss-docs-shenzhe-zhus-projects.vercel.app/getting-started).

## Commands

| Command | What it does |
| --- | --- |
| `doss init` | Create a vault, or use `--from owner/repo` to attach another device. |
| `doss connect` | Wire the vault into installed agents; rerun after installing a new agent. |
| `doss check` | Validate memory files; `--changed` checks only files touched since the last commit. |
| `doss sync` | Commit, pull, and push; refuses to sync invalid vault state. |
| `doss log` | Record or read the disclosure ledger. |
| `doss doctor` | Show vault health, sync, wiring, hooks, and tidy hints; `--fix` repairs wiring. |
| `doss tidy` | List stale facts, unconfirmed guesses, and notes backlog for owner judgment. |
| `doss uninstall` | Remove the local vault and unwire agents without touching the cloud copy. |

## Trust Boundary

Doss is local-first. If an agent has raw access to the vault, disclosure policy is
agent discipline plus an audit log, not a hard security boundary. Strong
enforcement requires a serving layer that applies policy without giving the
outward-facing agent raw vault access.

## Docs

- [Documentation site](https://doss-docs-git-doss-docs-shenzhe-zhus-projects.vercel.app)
