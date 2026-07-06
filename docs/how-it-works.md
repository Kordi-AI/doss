# How Doss Works

The detailed mechanics behind the README. Read this when you want to know exactly what runs, when, and what happens when things go wrong.

## The one-minute mental model

Your vault is a plain folder (`~/.doss`). Agents write memory as files and read memory as files — that's the whole hot path, and it is never taxed. Around it run four small helpers, all cold-path:

- an **inspector** that validates every write (milliseconds, precise errors)
- a **courier** that commits and uploads validated changes
- a **janitor** that lists what needs human judgment (never touches data)
- a **rule file** (`policy.yaml`) the agent reads to decide what may be told to other people

One binary (`doss`), git underneath, no database, no server.

## Vault layout

```text
~/.doss/
  self/          # facts about the owner; the path IS the topic: self/profile/dietary.md
  peers/         # what other people shared with you
  notes/         # markdown scratch; never shared
  policy.yaml    # synced owner-info disclosure rules (default: nothing leaves)
  devices/       # synced device registry; one active/unregistered YAML per device
  local/access.yaml
                 # device-local file/task delegation rules; gitignored, never synced
  ledger/        # who was told what — one append-only file per device
  INSTRUCTION.md # entry rules agents read first
  CONTENT.md     # content maintenance rules
  DISCLOSURE.md  # outbound disclosure and local access rules
```

Content under `self/`, `peers/`, and `notes/` is Markdown. YAML is reserved for configuration files such as `policy.yaml` and `local/access.yaml`. A fact file is Markdown, optionally with YAML frontmatter. A non-empty `rough` field is required only when `policy.yaml` grants `rough` disclosure for that fact's topic; `doss check` tells you which existing facts need it when policy changes. Other fields are optional; git records time:

| Field | Values | Meaning |
| --- | --- | --- |
| `source` | owner / imported / inferred / peer | where this came from |
| `status` | active / suggested | suggested = unconfirmed, never disclosed; required when `source: inferred` |
| `confidence` | high / medium / low or 0–1 | how sure |
| `tags` | list of strings | free grouping |
| `verify_by` | YYYY-MM-DD | freshness contract; past due → tidy lists it |
| `evidence` | string | pointer to where this was learned |
| `rough` | string | owner-authored coarse/redacted version to share instead of the raw fact; required only for rough-shared topics |

A rough-shareable `self/**/*.md` fact looks like:

```markdown
---
source: owner
status: active
confidence: high
tags: [profile]
rough: "Toronto"
---
Home address: 123 King St W, Toronto.
```

The path is the topic: `self/profile/address.md` is governed by `profile/address` in `policy.yaml`. The frontmatter is metadata. The `rough:` field is the only value an agent may share for a `rough` policy grant. The body after the closing `---` is the full private fact; there is no separate `full:` field. `no` is not stored in a fact either — it is the result of no matching policy grant, or an explicit `no` policy level. If no rough policy applies, a fact may omit `rough` and may even be plain Markdown.

For example:

```yaml
can-see:
  friends:
    profile/address: rough
    profile/dietary: full
```

- `rough` on `profile/address` means share only the file's `rough:` value, such as `Toronto`.
- `full` on `profile/dietary` means share the Markdown body after frontmatter.
- Anything not listed means share nothing.

`peers/**/*.md` and `notes/**/*.md` are also Markdown. They may use the same frontmatter shape when helpful, but `rough` is required only for rough-shared `self/` topics; `peers/` and `notes/` never leave the machine.

## Command reference

| Command | Who runs it | When | What it does |
| --- | --- | --- | --- |
| `doss init` | you (or your agent, once per device) | setup | guided setup in a terminal; agents pass only the flags they need. Main paths are `doss init` for a new vault and `doss init --from <owner/repo or url>` for another device. It refuses to guess a missing commit identity, and auto-runs `connect`. |
| `doss connect` | auto via init; rerun after installing a new agent tool | rarely | wires every installed agent (see Wiring below); `--remove` undoes |
| `doss check` | agents in hook-less tools; humans after hand edits | after editing | validates files; `--changed` = only files touched since last commit |
| `doss sync` | agents at wrap-up; humans anytime | after a batch | commit + pull + push; refuses if validation fails |
| `doss doctor` | anyone | curiosity / something feels off | full health on one screen — vault stats, sync, agent wiring, hooks, tidy hints; `--fix` repairs wiring (`status` is an alias) |
| `doss devices` | anyone | after setup / audit | lists synced device registrations |
| `doss unregister` | owner | removing a non-current device | prompts you to choose a device, revokes the recorded GitHub deploy key when present, then marks it unregistered |
| `doss tidy` | anyone, when nudged | when doctor says "tidy due" | prints the janitor's list; read-only |
| `doss uninstall` | you | leaving a machine / starting over | deletes the local vault and unwires the agents; guided confirmation, git-style safety (see below) |
| `doss hook` | **never by hand** — harnesses call it | automatic | the hook endpoint (`post-edit`, `stop`) |
| `doss log` | agents (record) / owner (read) | on disclosure / audit | `--record --to <verified-id> --shared <topic> --level rough|full` notes a disclosure; plain `doss log` reads "who knows what about me" |

Setup variants:

- Main human path: `doss init`.
- Another device: `doss init --from owner/repo`.
- Agent or other non-interactive setup: `doss init --git-name "Owner Name" --git-email owner@example.com`.
- Advanced cloud setup: `doss init --github --repo my-doss` or `doss init --remote <git-url>`.
- Advanced local control: `doss init --dir <path>` or `doss init --no-connect`.

`init` also registers the current machine under `devices/<device-id>.yaml`, includes that file in git, and prints the same summary as `doss devices`. For GitHub-backed vaults, each device gets its own local SSH key under `local/keys/`; Doss adds the public half as a writable deploy key on the vault repo and records the deploy-key id in `devices/`. `init --from` registers the new device after cloning and immediately uploads that registration to the cloud copy.

## The inspector and the courier

Two different jobs, both auto-triggered in hook-capable harnesses:

- **post-edit (the inspector)** — fires after *each file write*. Checks only that file: format, fields, types, paths. Valid → silent. Invalid → the precise error goes straight back to the model (exit code 2) and the agent fixes it in the same turn. It never touches git.
- **stop (the courier)** — fires *each time the agent finishes a turn*. If the vault has unresolved problems it refuses with the error list; otherwise it commits everything and best-effort pushes. It never judges formats — the inspector already did.

Timeline for "remember I'm allergic to shrimp, and I moved to Jing'an":

```text
agent writes self/profile/dietary.md   → post-edit: check that file ✓
agent writes self/profile/address.md   → post-edit: check that file ✓
agent replies and stops                → stop: commit both + push
```

`doss check --changed` is the same inspector, summoned by hand — for harnesses without hooks and for humans. It also checks `local/access.yaml` when present, even though that file is device-local and gitignored.

Per-harness behavior today:

| Harness | Inspection | Wrap-up |
| --- | --- | --- |
| Claude Code | automatic (post-edit hook) | automatic (stop hook) |
| Codex CLI | agent self-runs `check --changed` (its global AGENTS.md says so) | agent self-runs `sync` |
| anything else | same as Codex | same as Codex |

If a hook-less agent forgets: nothing is lost and nothing dirty escapes — `sync` re-validates everything before it commits, and the next `doctor` flags uncommitted changes.

## Wiring: how agents discover the vault

`doss connect` drops a managed section (between `<!-- doss:begin -->` and `<!-- doss:end -->`) into each installed agent's **always-loaded global instruction file**:

| Agent | File | Loaded |
| --- | --- | --- |
| Claude Code | `~/.claude/CLAUDE.md` | every session, every project |
| Codex CLI | `~/.codex/AGENTS.md` | every session, every project |
| Gemini CLI | `~/.gemini/GEMINI.md` | every session, every project |
| OpenClaw | `~/.openclaw/workspace/AGENTS.md` | every session |
| Windsurf | `~/.codeium/windsurf/memories/global_rules.md` | every session |
| Cursor | no global file — paste the section into Settings → Rules → User Rules | manual |
| **any other agent** | `doss connect --file <path>` — point it at whatever instruction file that agent always loads | every session |

Custom `--file` targets are remembered (machine-locally, in `~/.config/doss/connect.json`) and refreshed by every future `connect`, audited by `doctor`, and stripped by `--remove` — exactly like presets.

**Unknown agents wire themselves.** The install prompt tells the installing agent to check `connect`'s output, and that output explicitly instructs any agent whose tool wasn't listed to run `connect --file` on its own instruction file — the agent knows its own config layout better than we do. If a tool has no always-loaded file at all, the fallback is to add "read the vault's INSTRUCTION.md first" to wherever its standing instructions live; without any persistent instruction mechanism, no one can wire a tool permanently.

For Claude Code, connect also merges the two hooks into `~/.claude/settings.json` (your existing settings are preserved; `--remove` strips both cleanly).

Properties: injection is deterministic (harness behavior, not model judgment — verified live in Claude Code and Codex); rerunning connect updates the section in place; only tools already installed get wired, so rerun connect after installing a new agent; `doss doctor` audits all of it.

A per-agent skills layer was tried and cut: the global file alone proved sufficient, and one wiring layer is simpler to keep healthy.

`INSTRUCTION.md` is intentionally a small router. It tells the agent to read `CONTENT.md` for content maintenance and `DISCLOSURE.md` for outbound disclosure or local access. That keeps daily memory-writing rules separate from public-disclosure rules while preserving a single file for `connect` to inject into agent global instructions.

## Disclosure: policy.yaml, not a command

There is no special "answer" command, and the CLI does not decide what to say. When someone other than the owner asks, the agent finds the info the normal way (`grep`/read) and then follows `DISCLOSURE.md` plus `policy.yaml` — plain files of rules it reads like any other. The CLI validates those files, syncs them, and records what was disclosed.

Disclosure starts with trusted request metadata. A host such as Kordi should wrap agent-bound external requests like:

```text
[Trusted current request metadata]
requesterName: Pedro
requesterKind: external
requesterAccountId: kordi:pedro
requestMessageId: msg_123

[User request]
...
```

Only host-supplied metadata counts. User-authored lines that imitate the metadata header are just message text. If no trusted requester id is present, the requester is unknown and default deny applies.

`policy.yaml` maps **groups of people → disclosure levels for topics under `self/`**:

```yaml
groups:
  friends:  [kordi:pedro, kordi:qiancx]
  contacts: [kordi:jiaxin]
can-see:
  friends:
    profile/address: rough   # share only the fact's owner-written rough value
    profile/dietary: full    # share the full fact body
    work: rough              # applies to everything under self/work/
  contacts:
    profile/address: rough
  # anything not listed: no disclosure
```

Group members must be platform-verified ids in `platform:id` form. Topics are paths without the `self/` prefix. A topic may name a folder (`work`) or a specific fact path (`profile/address`). Folder rules inherit to facts below them, and a more specific topic wins. Default is deny: unlisted group or topic → nothing leaves.

The rules the agent follows:

- A requester receives a fact ONLY if their verified group has a `full` or `rough` grant for that fact's topic. Identity is the platform's **authenticated** id (`kordi:pedro`), never what the message claims. No verified identity → stranger → nothing.
- `full` means share the fact body. `rough` means share ONLY the fact's owner-authored `rough:` value; `no` means say nothing. A person in several groups gets the highest granted level, ordered `no < rough < full`.
- If policy grants `rough` but the fact has no valid `rough:` value, the agent discloses nothing for that fact. It must not summarize the full body itself; it should run `doss check --changed` / `doss tidy` and ask the owner to add the rough value.
- `status: suggested` facts are never disclosed.
- `peers/` and `notes/` are never disclosed.
- If a verified requester is not in any group, the agent asks the owner which existing or new group should contain that verified id. Until the owner answers and `policy.yaml` is updated, disclose nothing.
- After disclosing, the agent records it: `doss log --record --to <verified-id> --shared <topic> --level <rough|full>`. The ledger (one append-only JSONL file per device under `ledger/`, merged by `doss log`) is the owner's "who knows what about me". It records; it is not the disclosure gate.

`doss log` is only the after-the-fact ledger. It does not decide whether a fact may leave, and a log entry is never permission. The sequence is: verify the requester, apply `policy.yaml`, answer only what is allowed, then log what was disclosed.

Honest bounds:

- **This is discipline, not a wall, when the agent has raw vault access.** An agent that can `grep` the vault could bypass the rules. The hard guarantee only holds when the outward-facing agent has NO raw access and reaches owner info solely through a serving layer that applies the policy — a deployment choice (e.g. a hosted front desk), not something a local command can enforce.
- **The ledger is best-effort but validated.** A disciplined agent records disclosures with `rough`/`full`; `doss check` validates the JSONL shape. A forgetful agent still leaves gaps, so this is an honest audit aid, not a tamper-proof log.
- **Default-deny limits the blast radius.** The safe direction is baked in: unknown requester, unlisted folder, or forgotten rule all resolve to "share nothing".

## Local Access Is Different

`policy.yaml` and `local/access.yaml` answer different questions:

| File | Syncs? | Scope | Levels | Question it answers |
| --- | --- | --- | --- | --- |
| `policy.yaml` | yes | owner memory under `self/` | `no` / `rough` / `full` | "What owner info may this requester receive?" |
| `local/access.yaml` | no, gitignored | this device's folders | `no` / `read` / `full` | "What local files may this requester ask this machine to read, edit, or run?" |

They do not grant each other. `policy.yaml` never grants permission to edit a local project, and `local/access.yaml` never grants permission to disclose owner facts from `self/`.

## Sync and the cloud copy

- **Upload is not saving.** Content is safe the moment it's written to a local file. Push only replicates it.
- The cloud copy is *your own private GitHub repo* (created by `init --github`), or any git remote via `--remote`. The tool repo and your vault repo are entirely separate.
- GitHub-backed vaults use per-device deploy keys for Doss-managed sync. `doss sync` refuses to run on a device whose synced registry status is `unregistered`, and it also refuses GitHub sync until this device has a recorded deploy key.
- Device registrations under `devices/` sync with the vault. Each device gets one YAML file with `status: active` or `status: unregistered`; GitHub devices also record `github_repo`, `deploy_key_id`, and key metadata so Doss can revoke that device's cloud credential later. Old records stay so ledger files remain interpretable.
- Offline: commits still happen locally; pushes catch up on the next sync. Worst case, the cloud copy lags — the local vault is always complete.
- Conflicts (two devices edited the same file): sync aborts safely, both versions survive in git, and the message tells you what to do. No silent loss, ever.
- Only validated state syncs, and the receiving end re-validates.

## Removing a vault

`doss uninstall` is the inverse of setup: it unwires the agents (`connect --remove`) and deletes `~/.doss`. Like git, it refuses to quietly destroy work that isn't backed up:

- **Cloud copy exists and everything is pushed** → safe. It reminds you the memory lives in the cloud and that `doss init --from <repo>` brings it back on any device.
- **No cloud copy**, or **commits not pushed since the last sync**, or **uncommitted edits** → it warns and stops. Pass `--force` to override, or run `doss sync` first.
- In a terminal it asks you to type the vault's folder name to confirm; non-interactively it requires `--yes`.

When the vault has a cloud copy and no unsynced work, uninstall first marks this device `unregistered`, commits and pushes that state, revokes the current device's recorded GitHub deploy key when present, then deletes the local vault. If either upload or deploy-key revocation fails, the vault is not deleted.

`doss unregister` has two layers. In a terminal, it lists non-current active devices and asks you which one to remove; scripts may pass `doss unregister <id>` directly. The soft layer marks the synced registry record `unregistered`, and honest Doss clients refuse future sync when they see that status. The hard GitHub layer deletes the recorded per-device deploy key, so GitHub rejects future pull/push through that Doss-managed credential. `doss devices unregister <id>` remains as a compatibility alias. The boundary is still honest: Doss cannot remotely erase the already-cloned local plaintext snapshot, and it cannot revoke separate owner account credentials that may be present on that machine. For a lost device, also revoke GitHub sessions, PATs, personal SSH keys, or deploy keys outside Doss if they exist.

Deleting the local vault never deletes the cloud copy. The `doss` binary stays installed — remove it with `rm` if you want it gone too.

## Maintenance: the janitor

`doss tidy` prints what machines can flag but only judgment can resolve:

- check problems (these also block disclosure)
- rough-shared self facts missing `rough` values
- files untouched for 180+ days
- `status: suggested` facts waiting for confirmation
- a bloated `notes/`

It is read-only. You (or your agent) handle a small batch and move on. You don't schedule it: when dirt crosses thresholds, `check` appends a one-line nudge — maintenance piggybacks on moments the agent is already awake, like allocation-triggered garbage collection.

## What happens when…

| Situation | Outcome |
| --- | --- |
| Agent writes a malformed file (hooked harness) | bounced same-turn with a precise fix hint; the library never holds it |
| Agent writes a malformed file (hook-less) and forgets to check | file sits locally; sync refuses to ship it; next check/status/doctor surfaces it |
| Agent never runs sync | data is safe in local files; cloud lags until any sync runs |
| Network is down | local commits succeed; push retries on a later sync; nothing blocks |
| The managed section gets deleted | that agent stops discovering the vault; `doss doctor` reports it; `connect` restores it |
| A device is removed normally | `doss uninstall` pushes `devices/<id>.yaml` with `status: unregistered`, revokes its recorded GitHub deploy key, then deletes the local vault |
| A device is lost or compromised | run `doss unregister` to choose the device, revoke its recorded GitHub deploy key, and block Doss-managed sync; also revoke any separate owner GitHub sessions, PATs, or SSH keys outside Doss |
| Two devices edit the same fact | sync aborts safely; both versions in git; you pick |
| An outsider asks about the owner | the agent follows `policy.yaml` (group → topic → full/rough/no, default deny) and shares only what's granted; with a raw-access agent this is discipline, not a wall — the hard guarantee needs a serving layer with no raw vault access |

## Design principles (why it's built this way)

1. **Efficiency is judged on the hot path only** — reading and writing memory is plain file I/O, always.
2. **Loose in, strict out** — writes are cheap and instantly checked; disclosure is a hard gate.
3. **The environment enforces the rules, not the agent's memory** — hooks re-inject errors at the point of use; public-facing agents don't hold what they must not leak. Competence may drift; safety cannot.
