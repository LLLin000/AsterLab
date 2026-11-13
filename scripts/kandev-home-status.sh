#!/usr/bin/env bash
set -eu

ENGINEERING_HOME=${ENGINEERING_HOME:-/srv/projects/home}
REPOS_ROOT=${REPOS_ROOT:-/srv/projects}
if [[ -z ${KANDEV_API_TOKEN:-} ]]; then
    printf '%s\n' 'KANDEV_API_TOKEN is required to query Engineering Home status.' >&2
    exit 2
fi

export ENGINEERING_HOME REPOS_ROOT
export PYTHONPATH="$ENGINEERING_HOME/src"
exec python3 -m engctl status --project asterlab
