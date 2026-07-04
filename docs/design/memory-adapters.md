# Memory Compatibility Layer — Draft

- Date: 2026-07-04 · Status: for discussion (issue #8)
- One line: **we don't compete with memory systems; we make any memory system governable.**

## Positioning

Existing memory work (Mem0, Letta/MemGPT, Zep, LangMem, …) all optimizes the same thing: **your own agent recalling better and cheaper**. Nobody handles **what happens when someone else asks**. That is our scenario: governance + sync + disclosure. We don't need to win at memory itself — we stay compatible: they make you remember well, we make it governable.

## Architecture: three layers, files always win

```text
┌─ ecosystem bridges (optional)   Mem0 / Letta / Zep ⇄ files (landed with provenance)
├─ retrieval boost (optional)     .index/: SQLite FTS (+ optional embeddings), built incrementally from files
└─ canonical files (always)       self/ peers/ notes/ — check, sync, policy, answer trust ONLY these
```

Three non-negotiable invariants:

1. **The folder is the single source of truth.** Permissions, check, sync, and disclosure only read files; if any adapter breaks or disappears, the vault is intact.
2. **Indexes are derived caches.** `.index/` is rebuildable at any time, gitignored, never synced to the cloud — so it never enters the security model.
3. **Bridged content lands with provenance:** anything written by an external system arrives as `source: imported` (with `evidence` pointing home); inferred material is `status: suggested` and goes through confirmation. Bridges cannot bypass check and never touch the disclosure path.

## Candidates (picked for maximum difference)

| Tier | Option | Character | Dependency |
| --- | --- | --- | --- |
| Ultra-light (default) | plain: ls/grep/read | Zero deps, zero index; native agent skills | none |
| High-performance (built-in) | indexed: SQLite FTS5 + optional local embeddings | ms full-text/semantic search, incremental from files | single-file SQLite |
| Biggest ecosystem | bridge: Mem0 | two-way import/export for existing Mem0 users | Mem0 API |
| Research frontier | bridge: Letta (MemGPT) | agentic self-editing memory; strong paper dialogue | Letta server |
| Enterprise-stable (watch) | bridge: Zep | temporal knowledge graph, mature commercially | Zep service |

Recommended set: **plain default + indexed built-in** (neither adds an external service — consistent with "one folder + one small program"), first bridge **Mem0** (largest ecosystem, simple API), second **Letta** (paper value), Zep on watch.

## Bridge directionality (to decide)

- **pull (recommended first):** external system → files. One-way, simple, safe (everything entering passes check + provenance).
- **push:** files → external system. This hands your vault to a third-party service, so it must pass policy — elegantly, `bridge push` can be treated as just another requester and reuse the `give` levels. Nice design; not now.

## For the paper

> Doss is not another memory system; it is the governance layer any memory system can sit behind. Memory work optimizes recall for the self; Doss governs disclosure to others — an orthogonal, previously unaddressed axis.

## To decide in the morning

1. Agree with the recommended set (plain + indexed built-in; bridges Mem0 then Letta; Zep on watch)?
2. Bridges start pull-only; push later reuses policy `give` levels?
3. Embeddings off by default (privacy: no vault content to embedding APIs; local models may default on)?
4. Name Claude Code's auto-memory (already a folder) in the README as a natively-compatible case?
