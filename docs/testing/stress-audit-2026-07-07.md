# Stress audit, 2026-07-07

Issue: https://github.com/Kordi-AI/doss/issues/55

This was a Docker-isolated stress pass over the Doss CLI MVP. The run used a
`golang:1.25` Linux container, copied the source tree into `/tmp/doss-stress/src`
without `.git`, built `/tmp/doss-stress/bin/doss`, and kept all generated homes,
vaults, remotes, and caches inside `/tmp/doss-stress`.

Git identity used inside the container:

```text
Owner Name <zhushenzhe0531@gmail.com>
```

## Result

```text
SUMMARY pass=25 fail=0
```

No runtime fix was required.

## Workload

| Item | Count |
| --- | ---: |
| `self/**/*.md` facts | 1,105 |
| rough-shareable `self/profile/*.md` facts | 800 |
| full-shareable `self/public/*.md` facts | 200 |
| denied `self/private/*.md` facts | 100 |
| suggested `self/profile/*.md` facts | 5 |
| `peers/**/*.md` notes | 120 |
| `notes/**/*.md` scratch files | 65 |
| `local/access.yaml` grants | 100 |
| disclosure ledger entries recorded through `doss log --record` | 100 |
| files pushed to the local bare remote | 1,301 |

## Timings

These are single-run wall-clock timings inside Docker on the local machine.

| Operation | Time |
| --- | ---: |
| `doss init --remote <local-bare>` | 35 ms |
| `doss check --quiet` on large vault | 857 ms |
| `doss view --for kordi:pedro` | 90 ms |
| `doss tidy` | 924 ms |
| 100 `doss log --record` calls | 215 ms |
| `doss log --who kordi:pedro` after 100 entries | 3 ms |
| `doss check --quiet` after ledger writes | 854 ms |
| `doss sync --quiet` large vault to local bare remote | 280 ms |
| `doss hook stop` after one additional fact | 1,381 ms |

## Assertions

- Large vault validation passed.
- Requester view contained exactly the 1,000 facts allowed by policy: rough
  profile facts plus full public facts.
- Rough requester files contained only `rough:` values and did not include full
  private bodies.
- Denied private facts and suggested facts were omitted from requester view.
- `access.json` projected all 100 local access grants.
- `doss tidy` reported the 65-file notes pressure.
- 100 disclosure records were written and read successfully.
- Ledger state still passed validation.
- `doss sync` pushed the large vault to a local bare remote.
- `doss hook stop` committed and pushed one additional valid fact, leaving the
  vault clean.

## Boundaries

- This was a scale/stress pass, not a live GitHub deploy-key test. It used a
  local bare git remote and did not call GitHub APIs.
- All generated stress data lived inside the container and was discarded when
  the Docker run exited.
