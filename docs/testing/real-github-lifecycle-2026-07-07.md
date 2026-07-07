# Real GitHub lifecycle audit, 2026-07-07

Issue: https://github.com/Kordi-AI/doss/issues/57

This was a real GitHub remote test for the Doss MVP cloud path. Unlike the
state-machine and stress audits that used local bare git remotes, this run
created an actual temporary private repository under the logged-in GitHub
account, installed Doss deploy keys, pushed through those keys, attached a
second device, revoked a device key, uninstalled the current device, and then
deleted the temporary GitHub repository.

Temporary repository:

```text
ShenzheZhu/doss-realtest-20260707170300-25654
```

Git identity used inside Docker:

```text
Owner Name <zhushenzhe0531@gmail.com>
```

## Result

```text
SUMMARY pass=17 fail=0
CLEANUP_PASS repo_deleted=ShenzheZhu/doss-realtest-20260707170300-25654
```

Follow-up checks also found no remaining `doss-realtest` repositories and
`gh repo view ShenzheZhu/doss-realtest-20260707170300-25654` returned 404.

## Coverage

| Cell | Result |
| --- | --- |
| `doss init --github --repo <throwaway>` creates a private GitHub repo | pass |
| init records `github_repo` and `deploy_key_id` in the current device file | pass |
| GitHub repo has one writable deploy key after first device setup | pass |
| `doss sync` pushes a new fact through the device deploy key | pass |
| `git ls-remote origin main` works over the deploy-key SSH remote | pass |
| `doss init --from <owner/repo>` attaches a second Docker-isolated device | pass |
| attached device sees the fact pushed by the first device | pass |
| attached device records its own deploy key id | pass |
| GitHub repo has two deploy keys after attaching the second device | pass |
| original device pulls the attached device registration with `doss sync` | pass |
| `doss deactivate <attached-device>` revokes the attached deploy key | pass |
| GitHub repo has one deploy key after deactivating the attached device | pass |
| `doss uninstall --yes --keep-agents` deactivates the current device | pass |
| uninstall revokes the remaining current-device deploy key | pass |
| GitHub repo has zero deploy keys after uninstall | pass |
| local owner vault is removed by uninstall | pass |
| temporary GitHub repo is deleted after the test | pass |

## Notes

- The Docker container did not use the owner's local home directory or GitHub
  auth files. It received a `GH_TOKEN` environment variable and used a small
  `gh` shim that supported only `gh auth token`, `gh auth status`, and
  `gh repo clone`.
- The first dry run failed during second-device attach because the shim used a
  Bearer HTTP header for `git clone`; GitHub git HTTPS did not accept that mode.
  The temporary repo from that failed run was deleted. The passing run used
  `GIT_ASKPASS` with `x-access-token` and did not expose the token in remote
  URLs or output.
- The final cleanup was host-side and unconditional: `gh repo delete <repo>
  --yes`, followed by a repo lookup to confirm 404.
