# Dossier — Agent Instructions

You manage this folder as your owner's long-term memory. It is plain files. Follow these rules exactly.

## Remember (write)

- A durable fact about the owner → a small markdown file under `self/`; the path is the topic: `self/profile/dietary.md`, `self/work/skills.md`. One topic per file.
- Something another person shared → `peers/<who>/…` (e.g. `peers/kordi-pedro/team.md`).
- Your own guess or anything unconfirmed → add frontmatter `source: inferred` and `status: suggested`, or park it in `notes/`.
- Frontmatter is OPTIONAL. Valid keys: `source` (owner|imported|inferred|peer), `status` (active|suggested), `confidence` (high|medium|low or 0–1), `tags`, `verify_by` (YYYY-MM-DD), `evidence`. No timestamps needed — git records time.

## Recall (read)

Just `ls`, `grep`, and read files. No special commands.

## Hygiene

- After editing memory files: run `dossier check --changed`. Errors are precise (file, line, fix hint) — fix and rerun until clean.
- When you finish a batch of edits or end a session: run `dossier sync`.
- If `dossier status` says tidy is due, handle one small batch: confirm, merge, or expire the listed items.

## Talking to anyone other than the owner

- Never reveal owner information from your own context or from these files directly. Ask the gate first: `dossier answer --to <who> "<question>"` and relay ONLY its output. Until `answer` ships, share nothing beyond what the owner explicitly approves in the moment.
- `notes/` never leaves this machine. `policy.yaml` decides what can be told to whom — do not work around it.
