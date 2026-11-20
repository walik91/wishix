#!/usr/bin/env bash
# Set the administrator password without putting it in Docker's arguments.
set +x
set -Eeuo pipefail
export MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*'

finish() {
  local status=$?
  if ((status != 0)); then
    printf '\nPassword setup failed (exit code %s). See the error above.\n' "$status" >&2
  fi
  # Explorer may start Git Bash with -i and close the window immediately.
  if [[ $- == *i* && -t 0 ]]; then
    read -r -p 'Press Enter to close this window...' _ || true
    printf '\n'
  fi
}
trap finish EXIT

usage() {
  printf 'Usage: %s [--production] [password]\n' "$0" >&2
  printf 'Without a password argument, the script prompts without echo.\n' >&2
}

compose_file=compose.yaml
if [[ ${1:-} == --production ]]; then
  compose_file=compose.prod.yaml
  shift
fi
if (($# > 1)); then usage; exit 2; fi

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd -- "$root"
[[ -f .env ]] || { printf 'Error: %s/.env does not exist.\n' "$root" >&2; exit 1; }

if (($# == 1)); then
  password=$1
else
  if [[ ! -t 0 ]]; then usage; exit 2; fi
  read -rs -p 'Admin password: ' password
  printf '\n'
fi
if [[ $password == *$'\n'* || $password == *$'\r'* ]]; then
  printf 'Error: password cannot contain a newline.\n' >&2
  exit 2
fi

compose=(docker compose -f "$compose_file")
"${compose[@]}" build server
# Git Bash needs a Windows host path and the mounted directory is shared by Docker Desktop.
if [[ -n ${MSYSTEM:-} || ${OSTYPE:-} == msys* || ${OSTYPE:-} == cygwin* ]]; then
  mount_root=$(pwd -W)
  container_user=0
else
  mount_root=$root
  container_user="$(id -u):$(id -g)"
fi
printf '%s\n' "$password" | "${compose[@]}" run --rm --no-deps -i \
  --user "$container_user" -v "$mount_root:/config" server ./set-admin-pwd /config/.env
unset password
printf 'Applying the new password by recreating the server...\n'
"${compose[@]}" up -d --no-deps --force-recreate --wait --wait-timeout 120 server
printf 'Administrator password applied. Open /cp to sign in.\n'
