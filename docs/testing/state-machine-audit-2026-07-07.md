# State-machine audit, 2026-07-07

Issue: https://github.com/Kordi-AI/doss/issues/52

This was a Docker-isolated pass over the MVP CLI state machine in
`docs/testing/cli-state-machine.md`. The audit used the current source tree,
built `doss` inside a `golang:1.25` Linux container, and kept all vaults,
homes, git remotes, and build caches under `/tmp/doss-sm` inside the container.

Git identity used inside the container:

```text
Owner Name <zhushenzhe0531@gmail.com>
```

## Result

```text
SUMMARY pass=70 fail=0
```

No runtime fix was required.

## Coverage

| Area | State cells covered |
| --- | --- |
| Baseline | `go test ./...`, `go vet ./...`, `doss version`, `doss help`, unknown command refusal |
| `init` | local vault creation, existing-vault refusal, `--remote` with local bare git, `--from file://...` attach flow |
| `connect` / `doctor` | preset Codex wiring, stale section refresh, `--remove`, custom target, vault-internal target refusal, `doctor --fix`, `status` alias |
| `check` / `tidy` | clean vault, `--changed` missing-rough detection, rough/access fixes, missing-rough tidy report |
| `view` | verified requester success, invalid requester refusal, rough output, full output, missing-rough fallback omission, manifest blocked topic, local access projection, malformed local access refusal |
| `log` | missing level refusal, unverified recipient refusal, self-prefixed topic refusal, rough disclosure record, ledger read, ledger validation |
| `sync` | commit and push to local bare remote, no-upstream push, current-device deactivation after pull, no post-deactivation push |
| `devices` / `deactivate` | list registry, read-only `devices`, current-device refusal, missing-device refusal, other-device deactivation |
| `uninstall` | cloud-backed safe uninstall with pushed device deactivation, local-only refusal, forced local-only removal |
| `hook` | post-edit invalid vault file refusal, outside-file ignore, stop hook committing valid dirty vault, stop hook refusing invalid dirty vault |

## Boundaries

- Real GitHub deploy-key creation/deletion was not exercised because this audit
  did not use a GitHub token or live GitHub repository. Those API paths remain
  covered by the existing mocked Go tests.
- The Docker run used local bare git remotes to validate sync, attach,
  deactivate, and uninstall state transitions without touching the owner's local
  Doss vault or local git configuration.
- The source tree was copied inside the container without `.git` and with
  `GOFLAGS=-buildvcs=false`, so Go did not depend on the host worktree metadata.

## Command

The audit was run with:

```sh
docker run --rm -i -v "$PWD":/repo -w /repo golang:1.25 bash -s
```

The script copied `/repo` to `/tmp/doss-sm/src`, built
`/tmp/doss-sm/bin/doss`, and ran each state cell against isolated vaults under
`/tmp/doss-sm/cases`.
