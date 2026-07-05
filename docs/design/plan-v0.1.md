# Doss — Plan v0.1

- Status: current (supersedes the archived UCP plan v0; project renamed **Doss** on 2026-07-03)
- Date: 2026-07-03 (translated to English 2026-07-04)
- One line: **a synced memory folder, plus a gate that only wakes when information leaves.**

## 0. Decision principles (every trade-off is judged by these two)

1. **Efficiency is judged on the hot path only.** Anything the agent touches on every memory read/write must be plain file operations — zero ceremony, zero waiting. Non-negotiable (this is why md/yaml is the only agent-facing interface).
2. **Capability lives on the cold path.** Background jobs, the outbound gate, one-time setup — anywhere the agent can't feel — we don't hold back if it buys robustness or privacy.

Corollary: **machine-checkable mistakes are blocked at write time (milliseconds, imperceptible); semantic problems are queued for background triage; the exit is a hard gate.** Strictness and speed do not conflict here — validating a small file costs milliseconds; what was ever expensive is interface ceremony, not checking.

## 1. The two paths (the whole workflow)

**Daily use — it's just a notebook (hot path)**

- Remember = write a file; recall = read/grep files. No intermediate steps.
- Every write is machine-checked instantly (milliseconds): valid content lands; invalid content **bounces** — the error names the file, line, and fix, and the agent retries in the same turn (the content is still in its hands; nothing is lost). **The library only ever contains validated content**, so reads always return clean data.
- Sync runs in the background (only validated states sync; the receiving end re-validates).
- Semantic triage (duplicates, contradictions, staleness — anything that needs understanding) is **not on a timer**: when the vault's dirt level (stale count, duplicate candidates, backlog) crosses a threshold, the next agent write also triggers a small cleanup batch (bounded, never derails the current task) — like allocation-triggered garbage collection. Maintenance happens when the agent is already awake; no standing LLM daemon.

**Someone asks about the owner — only now do we get serious (cold path)**

- An outsider asks your agent a question (plain natural language; they install nothing).
- The agent first asks the local program: "what can I say?"
- The program consults the policy file and returns one of three things: **cleared content / nothing / let me ask the owner**.
- The agent replies using only that; the exchange is written to the ledger.
- Hard rule: content that failed checks, or that policy didn't clear, cannot pass this gate. In public settings the agent's context holds only card-level information — **it cannot leak what it does not have**.

## 2. The entire stack

One folder + one small single-binary program + git. No database, no server framework, no MCP dependency.

```text
~/.doss/
  self/          # facts about the owner; path = topic (self/profile/dietary.md)
  peers/         # what others shared with you (ingest only — transport is the platform's job)
  notes/         # scratch; never leaves this machine
  policy.yaml    # who may be told what
  ledger.log     # who was told what (append-only)
  SKILL.md       # the one-pager that teaches any agent the rules
```

A fact file (required fields ≈ zero; git records time):

```markdown
<!-- self/profile/dietary.md -->
---
confidence: high     # optional
---
- peanut allergy (severe)
- no organ meats
```

The policy file:

```yaml
groups:
  kordi-friends: [kordi:pedro, kordi:qiancx]

rules:
  - about: profile.dietary
    to: kordi-friends
    give: full          # as is
  - about: profile.address
    to: kordi-friends
    give: rough         # they hear the owner-authored blur, e.g. "Toronto"
  - about: "*"
    to: anyone
    give: nothing       # default: nothing leaves
```

`give` has three levels: `full` (verbatim) / `rough` (only the fact file's owner-authored `rough:` field) / `nothing`. A fourth boolean-only level was considered and cut: it needs a judge to read the fact, which either leaks it to the front-desk agent or adds an extra AI dependency — misses simply turn into "ask the owner" instead (decision log 07-04).

Five commands total:

| Command | What it does | When it runs |
| --- | --- | --- |
| `doss check` | Validate writes instantly: format, fields, types, paths, references; bad writes bounce with fixable errors | Automatically on every write (hook: zero window; watcher: seconds) |
| `doss sync` | Multi-device merge (git underneath) | Background |
| `doss answer` | The outbound gate: consult rules, return one of three responses, write the ledger | When someone asks |
| `doss log` | Query the ledger: "who knows what about me" | On demand |
| `doss tidy` | Semantic triage: duplicates, contradictions, staleness → todo list; **flagged items cannot leave until resolved** | Dirt threshold crossed → piggybacks on the next write |

Automation is three layers deep: hook-capable environments (e.g. Claude Code) give same-turn feedback → `doss watch` file watching as fallback → final validation at the exit. With none of them, nothing breaks: unvalidated content simply cannot leave.

## 3. Integrity guarantees (cutting complexity ≠ cutting capability)

- **Write-time checking is mandatory, not advisory.** Machine-checkable problems (format, fields, types, paths, references, path collisions) run synchronously on every write; bad writes bounce with precise, fixable errors and the agent retries in place. In watcher-only setups the bounced content is attached to the todo list — nothing is lost. Semantic problems (duplicates, contradictions, suspected misplacement) go to the tidy list; affected items cannot be disclosed until resolved.
- **History is never lost.** v0 has no status fields or change journal: edit the file directly — git history keeps every old value. Audit-grade history (tamper-proofing, single-entry deletion) waits until the ledger has real demand.
- **Outbound strictness arrives on day one of P1** — Kordi is the ready-made test bed.

### Staying on the rails long-term (turn 1 and turn 10,000 behave the same)

Rules survive ten thousand turns only one way: **the environment re-enforces them; the agent's memory is never load-bearing.**

1. **Hard constraints that never decay** — none of the three gates pass through LLM memory: public-facing context simply contains no private data (can't say what it doesn't have); bad writes physically cannot land (bounce); information leaves only through `answer`. An agent that forgets every rule loses competence, not safety.
2. **Rules live at the point of use, not in the opening prompt.** SKILL.md is onboarding; the durable carrier is every interaction's return value — error messages are refreshers ("missing `type`; the format is …"), `answer` output carries "reply with this text only", dirt nudges restate the rules. Agents obey recent context; make every system output carry the relevant rule.
3. **Drift checkups.** Rising bounce rate = the agent is getting sloppy; sampled outbound messages that don't match the ledger (on Kordi) = the agent is bypassing the gate. Detection triggers recalibration — forced re-read of SKILL.md, tighter nudge cadence.

In one line: **competence may drift; safety cannot** — safety is nailed to the environment, not the agent's memory.

## 4. Roadmap

**P0 — memory layer (now)**
Layout conventions, file format, check (full rules, automatic), sync, SKILL.md, tidy report.
Acceptance: a fresh agent operates the vault correctly from SKILL.md alone; valid writes have zero friction; invalid writes bounce immediately with fixable errors (same turn under hooks, seconds under the watcher); invalid content never syncs and never discloses — under hooks it is fixed within the same turn (per the bounce-and-retry ruling, a bad file may exist on disk for the seconds between write and fix); two devices editing the same file lose nothing (loser preserved in git history + todo reminder).

*Acceptance run 2026-07-04: all four criteria passed live (fresh haiku agent, external verification, two-device conflict sim); the "seconds under the watcher" clause is deferred until `doss watch` ships.*

**P1 — outbound gate (next; tested on Kordi)**
policy.yaml, `doss answer` with four levels, ask-the-owner (Kordi DM), ledger.
Acceptance (live two-person test): dietary restrictions returned verbatim; the home address goes out only as its owner-authored rough value ("Toronto"); missing info escalates to the owner with "answer once / answer and save"; every disclosure has a ledger entry; unvalidated or uncleared information cannot leave under any conversational strategy.

**P2 — always-on**
`doss serve` single binary, self-hosted (one-command docker; Tailscale/Cloudflare Tunnel for reachability); pre-cleared questions answered 24/7, gray zones push to the owner's phone.
Hosting tiers: none (local-only memory) → self-hosted daemon → platform-hosted (Kordi).

**P3 — evaluation & paper**
- Home turf: LoCoMo / LongMemEval-style benchmarks vs Mem0, Letta (MemGPT), Zep — quality + tokens + latency.
- Signature: two-person Kordi tasks measuring task success × over-disclosure rate × leakage under injection, with a "policy-in-prompt-only" control proving context isolation is load-bearing.
- Attack study: a binary-search adversary (chained yes/no probes) before/after rate limiting.
- Long-horizon consistency: adherence-decay curves over conversation length (rules-in-environment expected flat; rules-in-prompt expected to decay).

**v1+ (deferred; all cold-path)**
Predicate rate limiting (approved); receipts + update/invalidation push when both sides run Doss; tamper-evident ledger with single-entry deletion (journal/tombstone); live connectors (calendar etc., values as resolvable references); shared team/org vaults (in-vault roles).

## 5. Decision log (for team alignment)

| Date | Decision |
| --- | --- |
| 07-03 | Name: **Doss** (CLI `doss`; "UCP" dropped — collides with Google/Shopify's Universal Commerce Protocol) |
| 07-03 | Single-sided protocol: the other side installs nothing; plain natural language in |
| 07-03 | One vault, two faces: memory and disclosure are the same folder; disclosure = policy-filtered view |
| 07-03 | md/yaml files are the only agent interface; MCP/A2A at most future transport bindings |
| 07-03 | Hot path ruthless, cold path capable; write-bounce model (no quarantine dir — precise error + same-turn retry) |
| 07-03 | `doss ask` cut: Doss records and gates; it does not transport (asking others is the platform's job) |
| 07-03 | Requester identity: platform ids + owner-issued API tokens; misidentification fails safe (less disclosure, never more) |
| 07-03 | Cloud: self-hosted baseline; platform-hosted (Kordi) as a convenience tier |
| 07-03 | Benchmark against long-term-memory systems (Mem0/Letta/Zep): match them at home, then show the disclosure capability nobody has |
| 07-04 | Cloud copy v0 = the user's own **private GitHub repo** (`doss init --github`, default name `my-doss`); any git remote via `--remote` |
| 07-04 | Implementation language: Go (single static binary, trivial cross-compile) — reversible if challenged |
| 07-04 | Repo language: **English only** — code, docs, issues, PRs |
| 07-04 | `give` reduced to three levels (full / rough / nothing); boolean-only level cut — needs a judge, and "no answer → ask the owner" covers the need |
| 07-04 | Agent wiring (`doss connect`, auto-run by init): pointer **section** in each agent's always-loaded global file (~/.claude/CLAUDE.md, ~/.codex/AGENTS.md, ~/.gemini/GEMINI.md, Windsurf); `doss doctor --fix` verifies/repairs |
| 07-04 | Per-agent **skills layer tried and cut**: live tests showed the global instruction file alone injects reliably in both Claude Code and Codex; one wiring layer is simpler to keep healthy |

## 6. Open problems (not pretending otherwise)

1. Aggregation leakage (compliant partial disclosures assembled into more): rate limiting is a real mitigation; the general inference problem stays open — stated honestly in the paper.
2. The ask-the-owner wait in real-time conversations: mitigated by vault growth and session-scoped pre-clearance; UX design needed.
3. Encryption details for sensitive content on cloud replicas (pressure much lower under self-hosting/private repos; decide in P2).
