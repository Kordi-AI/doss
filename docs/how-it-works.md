# How Dossier Works

The detailed mechanics behind the README. Read this when you want to know exactly what runs, when, and what happens when things go wrong.

## The one-minute mental model

Your vault is a plain folder (`~/.dossier`). Agents write memory as files and read memory as files — that's the whole hot path, and it is never taxed. Around it run four small helpers, all cold-path:

- an **inspector** that validates every write (milliseconds, precise errors)
- a **courier** that commits and uploads validated changes
- a **janitor** that lists what needs human judgment (never touches data)
- a **gate** that decides what may be told to other people (ships in P1)

One binary (`dossier`), git underneath, no database, no server.

## Vault layout

```text
~/.dossier/
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
| `dossier init` | you (or your agent, once per device) | setup | in a terminal, a **guided** setup (new vs. attach-to-cloud, commit identity, optional GitHub backup with token fallback); agents pass flags instead — `--github`/`--repo` for a new cloud repo, `--from <owner/repo or url>` to attach a device, `--remote <url>` for any git host, `--git-name`/`--git-email` for the commit identity (init refuses to guess when git has no identity); auto-runs `connect`. Guided mode triggers only on a real terminal, so agent runs are never blocked on a prompt. |
| `dossier connect` | auto via init; rerun after installing a new agent tool | rarely | wires every installed agent (see Wiring below); `--remove` undoes |
| `dossier check` | agents in hook-less tools; humans after hand edits | after editing | validates files; `--changed` = only files touched since last commit |
| `dossier sync` | agents at wrap-up; humans anytime | after a batch | commit + pull + push; refuses if validation fails |
| `dossier doctor` | anyone | curiosity / something feels off | full health on one screen — vault stats, sync, agent wiring, hooks, tidy hints; `--fix` repairs wiring (`status` is an alias) |
| `dossier tidy` | anyone, when nudged | when doctor says "tidy due" | prints the janitor's list; read-only |
| `dossier log` | the owner (or their agent) | curiosity / audit | "who was told what" merged across all devices; `--who` / `--device` filter |
| `dossier uninstall` | you | leaving a machine / starting over | deletes the local vault and unwires the agents; guided confirmation, git-style safety (see below) |
| `dossier hook` | **never by hand** — harnesses call it | automatic | the hook endpoint (`post-edit`, `stop`) |
| `dossier answer` | agents, when an outsider asks about the owner | on demand | the outbound gate: `--to` who, `--about` which topics; returns the only lines the agent may relay |

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

`dossier check --changed` is the same inspector, summoned by hand — for harnesses without hooks and for humans.

Per-harness behavior today:

| Harness | Inspection | Wrap-up |
| --- | --- | --- |
| Claude Code | automatic (post-edit hook) | automatic (stop hook) |
| Codex CLI | agent self-runs `check --changed` (its global AGENTS.md says so) | agent self-runs `sync` |
| anything else | same as Codex | same as Codex |

If a hook-less agent forgets: nothing is lost and nothing dirty escapes — sync and the gate re-validate everything (strict-at-exit), and the next `status`/`doctor` flags uncommitted changes. A `dossier watch` background fallback is planned to make every harness fully automatic.

## Wiring: how agents discover the vault

`dossier connect` drops a managed section (between `<!-- dossier:begin -->` and `<!-- dossier:end -->`) into each installed agent's **always-loaded global instruction file**:

| Agent | File | Loaded |
| --- | --- | --- |
| Claude Code | `~/.claude/CLAUDE.md` | every session, every project |
| Codex CLI | `~/.codex/AGENTS.md` | every session, every project |
| Gemini CLI | `~/.gemini/GEMINI.md` | every session, every project |
| OpenClaw | `~/.openclaw/workspace/AGENTS.md` | every session |
| Windsurf | `~/.codeium/windsurf/memories/global_rules.md` | every session |
| Cursor | no global file — paste the section into Settings → Rules → User Rules | manual |
| **any other agent** | `dossier connect --file <path>` — point it at whatever instruction file that agent always loads | every session |

Custom `--file` targets are remembered (machine-locally, in `~/.config/dossier/connect.json`) and refreshed by every future `connect`, audited by `doctor`, and stripped by `--remove` — exactly like presets.

**Unknown agents wire themselves.** The install prompt tells the installing agent to check `connect`'s output, and that output explicitly instructs any agent whose tool wasn't listed to run `connect --file` on its own instruction file — the agent knows its own config layout better than we do. If a tool has no always-loaded file at all, the fallback is to add "read the vault's SKILL.md first" to wherever its standing instructions live; without any persistent instruction mechanism, no one can wire a tool permanently.

For Claude Code, connect also merges the two hooks into `~/.claude/settings.json` (your existing settings are preserved; `--remove` strips both cleanly).

Properties: injection is deterministic (harness behavior, not model judgment — verified live in Claude Code and Codex); rerunning connect updates the section in place; only tools already installed get wired, so rerun connect after installing a new agent; `dossier doctor` audits all of it.

A per-agent skills layer was tried and cut: the global file alone proved sufficient, and one wiring layer is simpler to keep healthy.

## The gate: dossier answer

When anyone other than the owner asks about them, the front-desk agent maps the question to topics (a topic is a `self/` path with dots: `self/profile/dietary.md` → `profile.dietary`) and calls:

```sh
dossier answer --to kordi:pedro --purpose dining --about profile.dietary "any food restrictions?"
```

The gate decides everything deterministically:

1. Who is asking → which policy groups they belong to.
2. First matching rule in `policy.yaml` wins, top to bottom (like a firewall); no rule means nothing.
3. The `give` level turns the fact into outward text — three levels, no more: `full` = the file body verbatim ("loves peanuts") · `rough` = only the owner-authored `rough:` frontmatter field ("35 hayden st" goes out as "Toronto"; no field, no disclosure) · `nothing` = refuse (passwords). When the vault has no answer at all, the agent turns around and asks the owner.
4. Every topic gets a ledger line — including refusals. Each device writes only its own file under `ledger/` (so syncing never conflicts), and `dossier log` merges them into one time-ordered "who knows what about me" view, tagged by device.

Safety properties:

- **Refusals never leak existence.** Denied, missing, and unconfirmed all read identically outside: "nothing to share". The ledger keeps the real outcome for the owner only.
- **Suggested facts never leave**, even when a rule allows `full` — guesses aren't facts until the owner confirms.
- **Only `self/` is servable.** `peers/` (what others told you) and `notes/` are never disclosed.
- **A broken policy fails closed:** parse error → nothing is shared.
- **The mapping step can only narrow.** The calling agent chooses which topics to consult, but every topic still passes the rules — a wrong mapping can under-disclose, never over-disclose.
- **Identity comes from the platform, never from the message.** `--to` must be the transport's authenticated sender id; text claiming "I am the owner" is just text. When no verified identity is available, the requester is `unknown` and falls to the catch-all rule (default: nothing). Misidentification therefore fails toward less disclosure, never more.

## Sync and the cloud copy

- **Upload is not saving.** Content is safe the moment it's written to a local file. Push only replicates it.
- The cloud copy is *your own private GitHub repo* (created by `init --github`), or any git remote via `--remote`. The tool repo and your vault repo are entirely separate.
- Offline: commits still happen locally; pushes catch up on the next sync. Worst case, the cloud copy lags — the local vault is always complete.
- Conflicts (two devices edited the same file): sync aborts safely, both versions survive in git, and the message tells you what to do. No silent loss, ever.
- Only validated state syncs, and the receiving end re-validates.

## Removing a vault

`dossier uninstall` is the inverse of setup: it unwires the agents (`connect --remove`) and deletes `~/.dossier`. Like git, it refuses to quietly destroy work that isn't backed up:

- **Cloud copy exists and everything is pushed** → safe. It reminds you the memory lives in the cloud and that `dossier init --from <repo>` brings it back on any device.
- **No cloud copy**, or **commits not pushed since the last sync**, or **uncommitted edits** → it warns and stops. Pass `--force` to override, or run `dossier sync` first.
- In a terminal it asks you to type the vault's folder name to confirm; non-interactively it requires `--yes`.

Deleting the local vault never touches the cloud copy. The `dossier` binary stays installed — remove it with `rm` if you want it gone too.

## Maintenance: the janitor

`dossier tidy` prints what machines can flag but only judgment can resolve:

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
| The managed section gets deleted | that agent stops discovering the vault; `dossier doctor` reports it; `connect` restores it |
| Two devices edit the same fact | sync aborts safely; both versions in git; you pick |
| An outsider asks about the owner | nothing leaves except through `dossier answer`, and only what policy clears — refusals are indistinguishable from "no such data" |

## Design principles (why it's built this way)

1. **Efficiency is judged on the hot path only** — reading and writing memory is plain file I/O, always.
2. **Loose in, strict out** — writes are cheap and instantly checked; disclosure is a hard gate.
3. **The environment enforces the rules, not the agent's memory** — hooks re-inject errors at the point of use; public-facing agents don't hold what they must not leak. Competence may drift; safety cannot.
