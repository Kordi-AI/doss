# Doss CLI state machine

This matrix is for finding CLI bugs before they turn into release behavior. Doss
has a small command surface, but each command depends on hidden local state:
vault files, git state, device registration, requester identity, and agent
wiring.

## State axes

| Axis | States to cover |
| --- | --- |
| Vault | missing, fresh local vault, attached cloud vault |
| Git | clean, dirty, ahead, behind, no remote, pull conflict |
| Current device | unknown, active, deactivated |
| Other devices | none, active, deactivated |
| Policy | valid, malformed, no match, rough, full, specific override |
| Rough values | not required, present, missing, empty |
| Local access | absent, valid, malformed, unknown group |
| Requester | owner mode, verified external id, invalid id |
| Agent wiring | absent, current, outdated, custom target |
| Ledger | no disclosure, recorded rough/full, malformed entry |

## Command responsibilities

| Command | Reads | Writes | Must refuse when |
| --- | --- | --- | --- |
| `doss init` | flags, git config, optional remote | vault scaffold, git repo, device file, optional agent wiring | target vault exists, identity missing, remote attach is invalid |
| `doss connect` | vault path, known/custom agent files | managed instruction section, local custom target config | vault missing, except `--remove` |
| `doss check` | content files, `policy.yaml`, `local/access.yaml`, ledger, devices | nothing | any structural issue exists |
| `doss view` | `policy.yaml`, `local/access.yaml`, `self/` | requester-scoped temp view | requester id invalid, non-rough validation errors exist, output dir is unsafe |
| `doss sync` | full vault, git, current device | commit, pull, push, device registration | check fails, current device is deactivated, git cannot merge |
| `doss deactivate` | synced device registry, optional GitHub deploy-key config | target device status, optional deploy-key deletion | target missing, target is current device, target already deactivated |
| `doss devices` | synced device registry | nothing | mutation subcommands are used |
| `doss tidy` | facts, notes, check results | nothing | never blocks; it reports maintenance pressure |
| `doss doctor --fix` | full local installation state | safe repairs for hooks/wiring | vault missing for repairs that need a vault |
| `doss log --record` | disclosure arguments | ledger entry | recipient id, topic, or level is invalid |

## Review workflow

Use this document as the first pass whenever a command changes:

1. Pick the command and fill in its row: what it reads, what it writes, and what
   must stop it.
2. Cross the command with every relevant state axis above. Ignore only states
   that the command truly cannot observe.
3. For each reachable state, classify the behavior as one of:
   - **success**: the command completes and writes only its declared outputs.
   - **refuse**: the command stops before mutation with a concrete error.
   - **fallback**: the command safely does less, never more. Example: requester
     views omit facts missing required `rough` values.
4. Check consistency with neighboring commands. If `doss check --changed`,
   hooks, full `doss check`, and `doss view` see the same bad state, they should
   agree on whether it blocks, nudges, or safely omits.
5. Lock important cells with focused Go tests when the command has code paths
   that are easy to regress.

## MVP cells to keep covered

| Command | State combination | Expected behavior |
| --- | --- | --- |
| `doss connect` | custom target exists, managed section is stale | refresh in place without deleting user text |
| `doss connect --remove` | saved custom target exists | remove managed section and clear local custom target config |
| `doss check --changed` | `policy.yaml` now grants `rough` for an existing fact without `rough:` | report `E_ROUGH` for that fact |
| `doss view` | same missing-rough state | omit that fact and record it in `manifest.json` |
| `doss view` | malformed `local/access.yaml` | refuse export |
| `doss sync` | current device is `deactivated` after pull | refuse before pushing new owner facts |
| `doss deactivate` | target is current device | refuse |
| `doss uninstall` | cloud copy exists and clean | deactivate current device, push that state, then remove local vault |
