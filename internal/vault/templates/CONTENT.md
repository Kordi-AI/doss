# Doss — Content Maintenance

Use this file when writing, updating, reconciling, or reading owner memory for the owner.

## Remember

- A durable fact about the owner -> a small Markdown file under `self/`; the path is the topic: `self/profile/dietary.md`, `self/work/skills.md`. One topic per file.
- Something another person shared -> a Markdown file under `peers/<who>/...` (e.g. `peers/kordi-pedro/team.md`).
- Your own guess or anything unconfirmed -> add frontmatter `source: inferred` and `status: suggested`, or park it in `notes/`.
- Reconcile as you write. Before writing, check whether that topic's file already exists. If so, edit it in place: update it if the value changed, replace it if the new info supersedes the old, or leave it if nothing is new. Never create `dietary-2.md` / `dietary-new.md`.
- Content under `self/`, `peers/`, and `notes/` is Markdown. YAML is only for config files such as `policy.yaml` and `local/access.yaml`.
- Every Markdown fact under `self/` MUST use the standard fact shape below. No timestamps needed; git records time.

Standard `self/**/*.md` fact shape:

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

- The file path is the topic. `self/profile/address.md` becomes policy topic `profile/address`.
- YAML frontmatter is metadata. Valid keys: `source` (owner|imported|inferred|peer), `status` (active|suggested), `confidence` (high|medium|low or 0-1), `tags`, `verify_by` (YYYY-MM-DD), `evidence`, `rough`.
- The `rough:` field is the ONLY rough value. It must be the owner's safe coarse/redacted version of the fact, written as a string.
- Everything after the closing `---` is the full private fact body. There is no `full:` field.
- There is no `no:` field inside a fact. `no` is a disclosure-policy result.
- Keep each body focused on one topic. If a file starts collecting unrelated facts, split it by topic before syncing.
- `peers/**/*.md` and `notes/**/*.md` are also Markdown. They may use the same frontmatter shape when helpful, but `rough` is required only under `self/`; `peers/` and `notes/` never leave this machine.

## Structure Examples

Vault paths:

```text
self/
  profile/
    address.md
    dietary.md
  work/
    style.md
peers/
  kordi-pedro/
    team.md
notes/
  inbox.md
policy.yaml
local/access.yaml
```

Private fact at `self/profile/address.md`:

```markdown
---
source: owner
status: active
confidence: high
rough: "Toronto"
---
Home address: 123 King St W, Toronto.
```

Another private fact at `self/profile/dietary.md`:

```markdown
---
source: owner
status: active
rough: "nut allergy"
---
Severe peanut allergy; avoid peanut oil and mixed-nut desserts.
```

Peer note at `peers/kordi-pedro/team.md`:

```markdown
---
source: peer
confidence: medium
tags: [team]
---
Pedro prefers async status updates before meetings.
```

Scratch note at `notes/inbox.md`:

```markdown
- Maybe confirm preferred invoice address later.
```

## Recall

Use normal file tools: `ls`, `rg`, and read files. No special command is needed to read memory.

## Hygiene

- After editing memory files: run `doss check --changed`. Errors are precise: file, line, and fix hint.
- When you finish a batch of edits or end a session: run `doss sync`.
- If `doss doctor` says tidy is due, handle one small batch: confirm, merge, or expire the listed items.
