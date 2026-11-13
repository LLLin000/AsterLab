# Engineering Home status integration

AsterLab can query the shared Engineering Home status view. Home owns shared policy and navigation; AsterLab remains authoritative for project-domain contracts, while Kandev owns tasks and sessions and GitHub owns PR/check/merge facts. The Home `engctl status` path is read-only today. The wrapper does not mutate Home state or write either application's SQLite database.

## Environment

The wrapper `scripts/kandev-home-status.sh` accepts:

- `ENGINEERING_HOME`: Home checkout; defaults to `/srv/projects/home`.
- `REPOS_ROOT`: repository root; defaults to `/srv/projects`.
- `KANDEV_API_TOKEN`: required by `engctl` to query Kandev. Supply it through the existing protected environment configuration; do not put it in shell history or print it.

The script sets `PYTHONPATH` to `$ENGINEERING_HOME/src` and invokes Python 3. The Home checkout must be installed/available at `ENGINEERING_HOME` with its source tree present.

## Install and operate

From the AsterLab checkout, make sure the script is executable (it is tracked as an executable script) and load the protected Kandev environment, then run:

```sh
set -a
. /etc/kandev-ready-feeder.env
set +a
scripts/kandev-home-status.sh
```

Equivalent direct invocation:

```sh
ENGINEERING_HOME=/srv/projects/home REPOS_ROOT=/srv/projects \
  KANDEV_API_TOKEN="$KANDEV_API_TOKEN" scripts/kandev-home-status.sh
```

The command is equivalent to `PYTHONPATH="$ENGINEERING_HOME/src" python3 -m engctl status --project asterlab`. If the token is missing, the wrapper exits with an explanatory error. It performs no SQLite writes; do not add direct database updates to this integration.

## Ownership and migration

Keep exactly one owner for each fact and action. Home provides shared policy/navigation, not a second AsterLab project contract or task ledger. Project-specific rules stay in AsterLab; task/session state stays in Kandev; PR, checks, and merge facts stay in GitHub. Before migrating an automation or state field, identify its current authoritative owner and move its readers/writers together; do not run dual writers or maintain parallel copies. Any future write-capable integration requires an explicit contract and tested enforcement rather than treating this status entry point as permission to write.
