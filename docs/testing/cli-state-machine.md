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

## Test layers

| Layer | Use it for | Do not use it for |
| --- | --- | --- |
| Go unit tests | Pure parsing, validators, specific helper functions | Long command flows that need real files and git |
| `testscript` | Real CLI workflows with readable setup, assertions, and filesystem state | Pixel-perfect terminal demos |
| VHS tapes | Optional visual demos for docs and release smoke checks | Required CI validation or source of truth |
| Manual sandbox | Real GitHub/device-key testing after the MVP stabilizes | Routine local validation |

Current pilot scenarios live in `cmd/doss/testdata/script/`:

- `connect_custom.txt`: unknown-agent wiring, refresh, and remove.
- `view_requester_fallback.txt`: policy plus local access, with missing rough
  conservatively omitted from requester views.
