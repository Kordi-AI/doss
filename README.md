# doss

[![Documentation](https://img.shields.io/badge/docs-doss-6f6a60?style=flat&logo=readthedocs&logoColor=white)](https://doss-docs.vercel.app/)
[![Release](https://img.shields.io/github/v/release/Kordi-AI/doss?style=flat&logo=github&logoColor=white&color=8b7a8f)](https://github.com/Kordi-AI/doss/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/Kordi-AI/doss?style=flat&logo=go&logoColor=white&color=71879b)](go.mod)
[![macOS](https://img.shields.io/badge/macOS-supported-9c8068?style=flat&logo=apple&logoColor=white)](install.sh)
[![Linux](https://img.shields.io/badge/Linux-supported-8b8675?style=flat&logo=linux&logoColor=white)](install.sh)

> A user-owned, cross-platform vault for long-term personal preferences, synced across devices with owner-controlled public disclosure.

Doss gives the user one durable preference library at `~/.doss`. Agents read and
update it as plain files, while the CLI handles setup, validation, sync, agent
wiring, and policy-backed disclosure controls and logs.

## At a Glance

| Concern | Where it lives | What it does |
| --- | --- | --- |
| Preference vault | `~/.doss/self/`, `peers/`, `notes/` | Stores long-term preferences, facts, and notes as Markdown files. |
| Agent rules | `~/.doss/INSTRUCTION.md`, `CONTENT.md`, `DISCLOSURE.md` | Splits entry routing, content maintenance, and outbound disclosure rules. |
| Disclosure policy | `~/.doss/policy.yaml` | Maps groups of people to `full`, `rough`, or `no` disclosure for owner topics. Default: deny. |
| Device registry | `~/.doss/devices/` | Synced records of devices attached to the vault, including GitHub deploy-key metadata when available. |
| Local access policy | `~/.doss/local/access.yaml` | Gitignored, device-local rules for non-owner tasks on this machine's files. |
| Requester view | `doss view --for <verified-id> --out <dir>` | Builds a short-lived, redacted context directory for an external requester. |
| Audit ledger | `doss log` | Records who was told what; it does not authorize disclosure. |
| Sync | `doss sync` | Validates, commits, pulls, and pushes the vault. |

## Quick Start

**For people**

```sh
curl -fsSL https://raw.githubusercontent.com/Kordi-AI/doss/main/install.sh | sh
doss init
```

**For agents**

```text
Please install Doss and set up my memory vault:
1. Install https://github.com/Kordi-AI/doss, then run `doss init`.
2. Ask me for the vault git name/email if needed.
3. Check the `doss connect` output. If your tool is not listed, run
   `doss connect --file <the instruction file you load every session>`.
4. Read `~/.doss/INSTRUCTION.md` and manage my memory by its rules from now on.
```

## Agent Workflow

After setup, agents operate on the vault as normal files:

| Situation | Agent action |
| --- | --- |
| Learned a durable owner fact | Update one topic file under `self/`. |
| Needs owner context | Read or search the vault with normal file tools. |
| Edited vault files | Run `doss check --changed` and fix precise errors. |
| Finished a batch/session | Run `doss sync`. |
| Serving an external requester | Prefer a fresh `doss view --for <verified-id> --out <dir>` and answer from that directory. |
| Disclosed owner info | Run `doss log --record --to <verified-id> --shared <topic> --level <rough|full>`. |

For cloud sync and multi-device setup, see the
[Getting Started docs](https://doss-docs.vercel.app/getting-started).

## Commands

| Command | What it does |
| --- | --- |
| `doss init` | Create a vault, or use `--from owner/repo` to attach another device. |
| `doss connect` | Wire the vault into installed agents; rerun after installing a new agent. |
| `doss check` | Validate memory files; `--changed` checks only files touched since the last commit. |
| `doss sync` | Commit, pull, and push; refuses to sync invalid vault state. |
| `doss devices` | List synced device registrations. |
| `doss deactivate` | Choose a non-current device, revoke its recorded GitHub deploy key, then mark it inactive. |
| `doss view` | Generate a requester-scoped redacted view with `self/`, `access.json`, and `manifest.json`. |
| `doss log` | Record or read the disclosure ledger; records include `--level rough|full`. |
| `doss doctor` | Show vault health, sync, wiring, hooks, and tidy hints; `--fix` repairs wiring. |
| `doss tidy` | List stale facts, unconfirmed guesses, and notes backlog for owner judgment. |
| `doss uninstall` | Remove the local vault and unwire agents without touching the cloud copy. |

## Trust Boundary

Doss is local-first. If an agent has raw access to the vault, disclosure policy is
agent discipline plus an audit log, not a hard security boundary. Strong
enforcement requires a serving layer that applies policy without giving the
outward-facing agent raw vault access.

`doss view` is the local building block for that serving layer: it exports only
facts allowed by `policy.yaml` for one verified requester, plus a separate
`access.json` projection from `local/access.yaml`. It validates the vault before
export and refuses policy, access, device, or ledger problems other than missing
`rough` values. Missing `rough` values, suggested facts, peers, notes, and denied
topics are omitted.

For GitHub-backed vaults, Doss gives each registered device its own writable
deploy key and removes that key on deactivate. This stops future Doss-managed
sync from that device, but it cannot erase any local snapshot already cloned;
also revoke any separate owner account tokens or SSH keys left on a lost device.

## Docs

- [Documentation site](https://doss-docs.vercel.app/)
