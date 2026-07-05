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
  notes/         # scratch; never validated, never shared
  policy.yaml    # disclosure rules (default: nothing leaves)
  ledger/        # who was told what — one append-only file per device
  SKILL.md       # the one-pager agents follow
```

A fact file is markdown with optional YAML frontmatter. All fields are optional; git records time:

| Field | Values | Meaning |
| --- | --- | --- |
| `source` | owner / imported / inferred / peer | where this came from |
| `status` | active / suggested | suggested = unconfirmed, never disclosed; required when `source: inferred` |
| `confidence` | high / medium / low or 0–1 | how sure |
| `tags` | list of strings | free grouping |
| `verify_by` | YYYY-MM-DD | freshness contract; past due → tidy lists it |
| `evidence` | string | pointer to where this was learned |

## Command reference

| Command | Who runs it | When | What it does |
| --- | --- | --- | --- |
| `doss init` | you (or your agent, once per device) | setup | in a terminal, a **guided** setup (new vs. attach-to-cloud, commit identity, optional GitHub backup with token fallback); agents pass flags instead — `--github`/`--repo` for a new cloud repo, `--from <owner/repo or url>` to attach a device, `--remote <url>` for any git host, `--git-name`/`--git-email` for the commit identity (init refuses to guess when git has no identity); auto-runs `connect`. Guided mode triggers only on a real terminal, so agent runs are never blocked on a prompt. |
| `doss connect` | auto via init; rerun after installing a new agent tool | rarely | wires every installed agent (see Wiring below); `--remove` undoes |
| `doss check` | agents in hook-less tools; humans after hand edits | after editing | validates files; `--changed` = only files touched since last commit |
| `doss sync` | agents at wrap-up; humans anytime | after a batch | commit + pull + push; refuses if validation fails |
| `doss doctor` | anyone | curiosity / something feels off | full health on one screen — vault stats, sync, agent wiring, hooks, tidy hints; `--fix` repairs wiring (`status` is an alias) |
| `doss tidy` | anyone, when nudged | when doctor says "tidy due" | prints the janitor's list; read-only |
| `doss uninstall` | you | leaving a machine / starting over | deletes the local vault and unwires the agents; guided confirmation, git-style safety (see below) |
| `doss hook` | **never by hand** — harnesses call it | automatic | the hook endpoint (`post-edit`, `stop`) |
| `doss log` | agents (record) / owner (read) | on disclosure / audit | `--record --to X --shared Y` notes a disclosure; plain `doss log` reads "who knows what about me" |

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

`doss check --changed` is the same inspector, summoned by hand — for harnesses without hooks and for humans.

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

**Unknown agents wire themselves.** The install prompt tells the installing agent to check `connect`'s output, and that output explicitly instructs any agent whose tool wasn't listed to run `connect --file` on its own instruction file — the agent knows its own config layout better than we do. If a tool has no always-loaded file at all, the fallback is to add "read the vault's SKILL.md first" to wherever its standing instructions live; without any persistent instruction mechanism, no one can wire a tool permanently.

For Claude Code, connect also merges the two hooks into `~/.claude/settings.json` (your existing settings are preserved; `--remove` strips both cleanly).

Properties: injection is deterministic (harness behavior, not model judgment — verified live in Claude Code and Codex); rerunning connect updates the section in place; only tools already installed get wired, so rerun connect after installing a new agent; `doss doctor` audits all of it.

A per-agent skills layer was tried and cut: the global file alone proved sufficient, and one wiring layer is simpler to keep healthy.

## Disclosure: policy.yaml, not a command

There is no special "answer" command. When someone other than the owner asks, the agent finds the info the normal way (`grep`/read) and then follows `policy.yaml` — a plain file of rules it reads like any other.

`policy.yaml` maps **groups of people → folders under `self/` they may see**:

```yaml
groups:
  friends:  [kordi:pedro, kordi:qiancx]
  contacts: [kordi:jiaxin]
can-see:
  friends:  [profile, work]   # everything under self/profile/ and self/work/
  contacts: [profile]         # only self/profile/
  # anything not listed: nothing
```

Why by folder and not by fact: adding a new fact under `self/profile/` needs no policy edit (it inherits the folder's rule), and adding a group is one block here — never a per-fact change. Default is deny: unlisted group or folder → nothing leaves.

The rules the agent follows:

- A requester sees a fact ONLY if their group is granted that fact's folder. Identity is the platform's **authenticated** id (`kordi:pedro`), never what the message claims. No verified identity → stranger → nothing.
- Graded values are data, not a command: a fact with `public_value: "Toronto"` is shared as "Toronto", never the raw street. The owner authors the shareable version.
- `peers/` and `notes/` never leave.
- After disclosing, the agent records it: `doss log --record --to <who> --shared <topic>`. The ledger (one append-only file per device under `ledger/`, merged by `doss log`) is the owner's "who knows what about me".

Honest bounds:

- **This is discipline, not a wall, when the agent has raw vault access.** An agent that can `grep` the vault could bypass the rules. The hard guarantee only holds when the outward-facing agent has NO raw access and reaches owner info solely through a serving layer that applies the policy — a deployment choice (e.g. a hosted front desk), not something a local command can enforce.
- **The ledger is best-effort.** A disciplined agent records disclosures; a forgetful one leaves gaps. It's an honest audit aid, not a tamper-proof log.
- **Default-deny limits the blast radius.** The safe direction is baked in: unknown requester, unlisted folder, or forgotten rule all resolve to "share nothing".

## Sync and the cloud copy

- **Upload is not saving.** Content is safe the moment it's written to a local file. Push only replicates it.
- The cloud copy is *your own private GitHub repo* (created by `init --github`), or any git remote via `--remote`. The tool repo and your vault repo are entirely separate.
- Offline: commits still happen locally; pushes catch up on the next sync. Worst case, the cloud copy lags — the local vault is always complete.
- Conflicts (two devices edited the same file): sync aborts safely, both versions survive in git, and the message tells you what to do. No silent loss, ever.
- Only validated state syncs, and the receiving end re-validates.

## Removing a vault

`doss uninstall` is the inverse of setup: it unwires the agents (`connect --remove`) and deletes `~/.doss`. Like git, it refuses to quietly destroy work that isn't backed up:

- **Cloud copy exists and everything is pushed** → safe. It reminds you the memory lives in the cloud and that `doss init --from <repo>` brings it back on any device.
- **No cloud copy**, or **commits not pushed since the last sync**, or **uncommitted edits** → it warns and stops. Pass `--force` to override, or run `doss sync` first.
- In a terminal it asks you to type the vault's folder name to confirm; non-interactively it requires `--yes`.

Deleting the local vault never touches the cloud copy. The `doss` binary stays installed — remove it with `rm` if you want it gone too.

## Maintenance: the janitor

`doss tidy` prints what machines can flag but only judgment can resolve:

- check problems (these also block disclosure)
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
| Two devices edit the same fact | sync aborts safely; both versions in git; you pick |
| An outsider asks about the owner | the agent follows `policy.yaml` (group → folders, default deny) and shares only what's granted; with a raw-access agent this is discipline, not a wall — the hard guarantee needs a serving layer with no raw vault access |

## Design principles (why it's built this way)

1. **Efficiency is judged on the hot path only** — reading and writing memory is plain file I/O, always.
2. **Loose in, strict out** — writes are cheap and instantly checked; disclosure is a hard gate.
3. **The environment enforces the rules, not the agent's memory** — hooks re-inject errors at the point of use; public-facing agents don't hold what they must not leak. Competence may drift; safety cannot.
