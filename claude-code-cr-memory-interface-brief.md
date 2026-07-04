# Claude Code Brief: Why CR, And How To Build A Markdown-Shaped CR Interface

- Status: planning brief for Claude Code
- Last updated: 2026-07-03
- Intended reader: Claude Code or another implementation agent
- Related design note: `docs/plans/active/memory-management/hybrid-agent-memory-interface.md`
- Related current docs:
  - `docs/current/MCP_AUTHORIZATION.md`
  - `docs/current/PREFERENCE_SCHEMA.md`

## Executive Summary

We are trying to decide whether Context Router (CR) is genuinely needed when a
structured Markdown file can already work well as agent memory.

The current answer is:

> Structured Markdown is a strong local memory interface. CR is only justified
> when memory must become governed infrastructure: permissioned, audited,
> schema-validated, reusable by multiple clients, and safely exposed without
> giving agents raw document access.

Therefore the proposed technical direction is not "replace CR with Markdown"
and not "force agents to use verbose MCP JSON tools."

The proposed direction is:

> Keep CR as the structured, permissioned backend, but expose it to agents as a
> constrained Markdown-like virtual file: `/app/cr_memory.md`.

Agents should interact with something that feels like a normal `memory.md`.
The backend should still enforce CR semantics: schema validation, permission
grants, audit logs, provenance, confidence, status lifecycle, and structured
downstream access.

## Current CR System

This repo is a `pnpm` monorepo.

- `apps/backend`: NestJS, GraphQL, Prisma, MCP endpoint
- `apps/web`: Next.js dashboard
- PostgreSQL stores users, preference definitions, preferences, permission
  grants, MCP access logs, and preference audit events.

The relevant backend pieces are:

| Area | Files / modules | What it does |
| --- | --- | --- |
| Preference schema | `apps/backend/src/modules/preferences/preference-definition/**` | Stores available memory fields as definitions. |
| Preference values | `apps/backend/src/modules/preferences/preference/**` | Stores active/suggested/rejected user facts. |
| Permission grants | `apps/backend/src/modules/permission-grant/**` | Stores per-client, per-target, per-action allow/deny rules. |
| MCP tools | `apps/backend/src/mcp/tools/**` | Exposes preference read/search/mutate tools to external agents. |
| MCP authorization | `apps/backend/src/mcp/auth/mcp-authorization.service.ts` | Applies capability and target authorization. |
| Preference audit | `apps/backend/src/modules/preferences/audit/**` | Records before/after state for preference writes. |
| MCP access logs | `apps/backend/src/mcp/access-log/**` | Records tool/resource access metadata. |

## Current Data Model

The core CR memory model is not just a map from key to value.

### PreferenceDefinition

A preference definition describes a memory field.

Important fields:

- `namespace`: `GLOBAL` or `USER:<userId>`
- `slug`: canonical field id, e.g. `profile.full_name`
- `description`
- `valueType`: `STRING`, `BOOLEAN`, `ENUM`, `ARRAY`
- `scope`: `GLOBAL` or `LOCATION`
- `options`
- `isSensitive`
- `archivedAt`
- `ownerUserId`

This is the schema layer. It is what plain Markdown does not naturally enforce.

### Preference

A preference row stores a concrete fact for a user.

Important fields:

- `userId`
- `locationId`
- `contextKey`
- `definitionId`
- `value`
- `status`: `ACTIVE`, `SUGGESTED`, `REJECTED`
- `sourceType`: `USER`, `INFERRED`, `IMPORTED`, `SYSTEM`
- `confidence`
- `evidence`
- `lastActorType`
- `lastActorClientKey`
- `lastOrigin`

This is the durable memory layer.

### PermissionGrant

Permission grants are scoped by:

- `userId`
- `clientKey`
- `target`
- `action`
- `effect`

Actions:

- `READ`
- `SUGGEST`
- `WRITE`
- `DEFINE`

Targets can be:

- `*`
- `profile.*`
- `payroll.direct_deposit.*`
- exact slugs such as `profile.full_name`

This is the main CR capability that a single Markdown file does not naturally
provide. File permissions are usually file-level. CR permissions are field-level,
client-aware, and action-aware.

### PreferenceAuditEvent

Preference audit events record:

- `userId`
- `subjectSlug`
- `eventType`
- `actorType`
- `actorClientKey`
- `origin`
- `correlationId`
- `beforeState`
- `afterState`
- `metadata`

This gives CR a trusted audit trail. A Markdown file can contain a log, but that
log is agent-authored and therefore not a reliable system audit.

### McpAccessEvent

MCP access logs record:

- `userId`
- `clientKey`
- `surface`
- `operationName`
- `outcome`
- `correlationId`
- `latencyMs`
- request/response/error metadata

These logs are sanitized and should not store raw sensitive values.

## Current MCP Surface

Current important MCP tools/resources:

- `listPreferenceSlugs`
- `searchPreferences`
- `mutatePreferences`
- `smartSearchPreferences`
- `consolidateSchema`
- `listPermissionGrants`
- `schema://graphql`

The current MCP implementation is functionally useful but not agent-friendly
enough. It exposes structured JSON tool calls and can produce verbose tool
responses. In our Harbor runs, this caused much higher token overhead than
Markdown.

Known current issue:

- Read-only MCP tools duplicate `structuredContent` into `content[0].text` as
  pretty JSON for clients that only expose text. This can inflate agent context.

## Why Structured Markdown Is A Strong Baseline

A structured Markdown file can do many things well:

| Capability | Structured Markdown can do it? | Notes |
| --- | --- | --- |
| Store key-value facts | Yes | `slug: value` is easy. |
| Human readability | Yes | Usually better than JSON tool responses. |
| Agent readability | Yes | Coding agents are very comfortable with files. |
| Simple search | Yes | `grep`, `read`, and file search work well. |
| Simple update | Yes | Agents know how to edit Markdown. |
| Basic structure | Yes | Sections and fenced blocks are easy. |
| Evidence notes | Yes | Agent can write source notes manually. |
| Local single-agent memory | Yes | Often the simplest and best option. |

If the target scenario is only:

```text
one agent + one local memory file + no permissions + no audit + no downstream API
```

then Markdown is probably better than CR: simpler, cheaper, and easier for
agents.

## What Markdown Does Not Naturally Provide

Markdown can be extended to do almost anything, but the extension cost matters.

| Capability | Can structured Markdown do it? | What must be added | Why CR is different |
| --- | --- | --- | --- |
| Value type checking | Possible | Parser/linter/schema registry | Backend rejects invalid values. |
| Unknown slug checking | Possible | Schema registry and validator | Definitions are already stored in DB. |
| Status lifecycle | Possible | Agent discipline or scripts | DB has `ACTIVE`, `SUGGESTED`, `REJECTED`. |
| Provenance reliability | Possible but fragile | Agent must write it correctly | Backend stores provenance fields. |
| Trusted audit log | Hard | External logger, snapshots, tamper controls | Backend writes before/after audit events. |
| Per-client access | Hard | File sharding or access router | CR uses `clientKey` in grants and logs. |
| Per-field permission | Hard | Split files or custom renderer | CR uses slug/prefix grants. |
| Action-level permission | Hard | Custom policy engine | CR has `READ`, `SUGGEST`, `WRITE`, `DEFINE`. |
| Permission revocation | Hard | Regenerate views and prevent stale reads | CR checks access on each request. |
| Sensitive-field masking | Possible | View generator | CR can generate filtered/masked views. |
| Downstream structured API | Possible | Markdown parser and API layer | CR already stores structured DB rows. |
| Concurrent writes | Hard | Locks, merge logic, conflict resolution | DB transactions and unique constraints. |
| Multi-agent safety | Hard | Ownership and write policy layer | CR has client identity and audit. |
| Compliance evidence | Hard | Trustworthy system logs | CR has access/audit records. |

The important point:

> Markdown can approximate these features only after we add a parser, schema
> registry, permission router, audit logger, transaction layer, view generator,
> and downstream API.

At that point, it is no longer "just Markdown." It becomes CR-lite.

## Core Product Claim

CR should not claim:

> We store facts better than Markdown in a single-agent local setting.

That claim is probably weak. Markdown is strong there.

CR should claim:

> We make personal/team memory governable: permissioned, audited,
> schema-validated, provenance-aware, and reusable across agents and downstream
> systems.

The hybrid interface should then claim:

> Agents should not pay a heavy interface cost to use governed memory. They
> should operate a Markdown-shaped view while CR enforces system semantics behind
> it.

## Proposed Technical Direction

Build a schema-backed Markdown memory interface.

Agent-facing surface:

```text
/app/cr_memory.md
```

System behavior:

```text
Agent
  reads/searches/edits
        |
        v
/app/cr_memory.md
  virtual, generated, constrained Markdown view
        |
        v
CR memory adapter
  render view
  detect patch
  parse candidate mutations
  validate slug/type/permission/evidence/conflict
  return concise diff/errors
        |
        v
CR backend
  preference definitions
  preference values
  permission grants
  provenance
  audit logs
  structured DB storage
```

The agent should feel like it is maintaining a Markdown memory note.

The system should behave like governed structured CR memory.

## Virtual File Contract

`/app/cr_memory.md` must not be arbitrary free-form memory. It should be
constrained and parseable.

Required properties:

- Generated from CR DB state.
- Permission-filtered before the agent sees it.
- Every editable fact preserves a CR slug.
- Values are typed enough for safe parsing.
- Writes are patch-based.
- Accepted writes go through CR validation and audit.
- Large memory views are bounded or query-scoped.

It is not:

- source of truth
- raw Markdown storage
- a bypass around permission checks
- a full dump of every schema and active record

## Suggested Markdown Shape

Use Markdown for readability and fenced YAML-like blocks for parseability.

````markdown
# CR Memory View

view_id: view_20260703_001
format_version: 1
scope: permitted

## profile.identity

```cr-memory
slug: profile.full_name
value: "Maya Li Chen"
status: active
confidence: 0.98
updated_at: 2026-07-01
evidence: hr-onboarding/006-hr-onboarding-profile-export.yaml
```

```cr-memory
slug: profile.date_of_birth
value: "1998-04-12"
status: active
confidence: 0.95
updated_at: 2026-07-01
evidence: identity/001-driver-license-upload-ocr.txt
```

## payroll.direct_deposit

```cr-memory
slug: payroll.direct_deposit.account_type
value: "checking"
status: active
confidence: 0.94
updated_at: 2026-07-01
evidence: payroll-tax/016-direct-deposit-confirmation.yaml
```

## ignored_or_stale

- ignored other-user direct deposit packet from `noise/020-sample-direct-deposit-packet-jordan.txt`
- stale recruiter address from 2024-11-02 was superseded by HRIS export
````

Notes:

- Editable facts live in fenced `cr-memory` blocks.
- Each editable block must contain `slug`.
- Free-text notes can help the agent reason, but they are not committed as
  structured facts unless converted into slug-backed blocks.
- Sensitive values can be masked or omitted according to grants.

## Desired Agent Experience

The agent should only need to:

- read `/app/cr_memory.md`
- search within it
- edit structured blocks
- react to concise validation errors

The agent should not normally need to:

- call `listPreferenceSlugs`
- browse a large schema
- manually construct `mutatePreferences` JSON
- JSON-stringify values
- parse full active memory snapshots
- understand permission grant internals

This is the design principle:

> Hide MCP/DB complexity behind a file-like memory interface.

## Adapter Responsibilities

The adapter is the key implementation layer.

It should:

1. Render CR preferences into constrained Markdown.
2. Apply permission filtering before rendering.
3. Include a stable `view_id` / revision.
4. Track the base view for patch comparison.
5. Parse changed `cr-memory` blocks.
6. Reject ambiguous free-form edits.
7. Convert valid edits into candidate CR mutations.
8. Validate candidates through CR schema and permission logic.
9. Commit accepted mutations through existing backend services.
10. Return concise diff/errors to the agent.

The adapter must never blindly save Markdown as source of truth.

## Proposed Implementation Phases

### Phase 0: Define The Contract

Goal: lock the `cr-memory` block grammar before writing major code.

Deliverables:

- Markdown grammar spec.
- Required fields.
- Optional fields.
- Sensitive/masked value behavior.
- Parse error taxonomy.
- Example valid and invalid views.
- Decision: writes default to `SUGGEST_PREFERENCE` or `SET_PREFERENCE`.

Acceptance criteria:

- A parser can deterministically reject malformed blocks.
- A human can tell what is editable and what is commentary.

### Phase 1: Concise MCP Output

Goal: reduce current CR-MCP overhead even before the virtual file exists.

Deliverables:

- Concise response mode for read tools and `mutatePreferences`.
- Mutation responses return changed slugs/status/errors, not full objects by
  default.
- Response budgets such as `maxItems` and `maxChars`.
- Unit tests for concise mutation output.

Acceptance criteria:

- Existing CR-MCP arm becomes cheaper.
- Tool response bytes are measurable and lower.

### Phase 2: Read-Only `readMemoryView`

Goal: expose CR memory as a compact generated Markdown view.

Deliverables:

- New backend renderer/service for permission-filtered memory views.
- MCP tool such as `readMemoryView`.
- Markdown output mode.
- Query/scoped output support if simple.
- Tests for denied slug filtering and response budgets.

Acceptance criteria:

- A downstream agent can read the view and answer tasks without raw docs.
- Denied slugs never render.
- Sensitive values follow mask/omit rules.

### Phase 3: Harbor Virtual File Prototype

Goal: in the eval harness, expose CR memory as `/app/cr_memory.md`.

Deliverables:

- Harbor sidecar or adapter that materializes the read-only view.
- `cr-hybrid-readonly` arm.
- Same task/scorer/model as markdown and current cr-mcp arms.
- Report fields for token/cost/latency/tool bytes.

Acceptance criteria:

- Agent uses `/app/cr_memory.md` instead of direct CR-MCP JSON tools.
- The run validator confirms the arm does not use `/app/memory.md`.
- Compare `markdown`, `cr-mcp`, and `cr-hybrid-readonly`.

### Phase 4: Patch Preview

Goal: allow edits but do not commit yet.

Deliverables:

- Diff against `view_id`.
- Parser for changed `cr-memory` blocks.
- Candidate mutation preview.
- Rejection reasons for ambiguous edits.
- Tests for unknown slug, denied slug, malformed type, and free-form edits.

Acceptance criteria:

- Agent receives actionable preview/errors.
- No DB writes occur in preview mode.

### Phase 5: Patch Commit

Goal: commit validated patches into CR.

Deliverables:

- Commit accepted candidates through existing preference services.
- Preserve confidence/evidence/provenance.
- Write audit events.
- Write sanitized MCP/access logs.
- Dry-run support.
- Tests for successful update, denied update, masked sensitive write, malformed
  value, and stale/conflict behavior.

Acceptance criteria:

- `/app/cr_memory.md` becomes a real memory maintenance surface.
- CR remains the source of truth.
- Markdown is only projection and patch input.

## What Claude Code Should Be Careful About

Do not implement this as a raw Markdown file store.

Avoid these mistakes:

- Do not make `/app/cr_memory.md` the durable source of truth.
- Do not bypass `PreferenceService`.
- Do not bypass `McpAuthorizationService` or `PermissionGrantService`.
- Do not commit free-form notes as facts without slugs.
- Do not expose denied slugs in rendered views.
- Do not return full active memory snapshots after every mutation.
- Do not build a large new abstraction before the read-only prototype proves
  value.

Prefer the smallest useful implementation:

1. renderer
2. read-only view tool
3. Harbor virtual file arm
4. eval comparison
5. only then patch preview/commit

## Evaluation Plan

The evaluation should test the real CR value proposition.

Do not only test:

```text
Can the agent remember facts?
```

Markdown is strong there.

Also test:

```text
Can the memory system enforce permissions, preserve audit/provenance, support
structured downstream reuse, and avoid exposing raw private documents?
```

Suggested arms:

| Arm | Agent-facing interface | Backend substrate |
| --- | --- | --- |
| context-only | conversation context | none |
| markdown | `/app/memory.md` | raw file |
| cr-mcp | MCP JSON tools | CR DB |
| cr-hybrid-readonly | `/app/cr_memory.md` generated view | CR DB |
| cr-hybrid-patch | `/app/cr_memory.md` patch interface | CR DB |

Metrics:

- LLM judge reward
- state mean
- service mean
- input tokens
- output tokens
- latency
- cost
- model call count
- tool call count
- tool response bytes
- permission violations
- denied slug leakage
- audit coverage
- patch rejection count
- downstream structured read success

Expected research result:

- Markdown may remain competitive on simple local memory tasks.
- CR-hybrid should reduce overhead compared with current CR-MCP.
- CR should dominate Markdown on permission/audit/governance tasks.

## Core Research Framing

The paper should not frame CR as:

> A more accurate memory file.

That is too weak and probably false in simple settings.

The paper should frame CR as:

> A governed memory layer for agents.

And frame the hybrid interface as:

> A way to give agents Markdown-like ergonomics without giving up structured,
> permissioned, auditable memory.

## Concrete Next Task For Claude Code

Start with a planning-and-prototype PR, not a full patch-write implementation.

Recommended first PR:

1. Add a Markdown memory view renderer over existing preferences.
2. Add a read-only MCP tool, likely `readMemoryView`.
3. Add tests:
   - renders known active preferences
   - groups by slug prefix
   - omits denied slugs
   - masks sensitive values when required
   - obeys `maxChars`
4. Add or update Harbor sidecar logic to expose the tool result as
   `/app/cr_memory.md` for `cr-hybrid-readonly`.
5. Run a small Harbor smoke comparing:
   - markdown
   - current cr-mcp
   - cr-hybrid-readonly

Only after this shows lower interface overhead should we build patch preview and
commit.

## Bottom Line

Markdown is not the enemy. Markdown is the agent-friendly interface.

CR is justified only if it provides system guarantees that Markdown alone does
not naturally provide:

- permission
- audit
- provenance
- schema validation
- multi-client safety
- structured downstream reuse
- raw document isolation

The best architecture is therefore:

```text
Markdown-shaped interface
CR-backed semantics
```

The agent gets the simple file workflow it already understands.
The system keeps the governed memory layer that makes CR worth having.
