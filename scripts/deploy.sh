#!/usr/bin/env bash
set -Eeuo pipefail
export MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*'
umask 077

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }

# The checkout may replace this script during a fast-forward. Run a temporary copy
# so that the current process never reads a file that Git is updating.
if [[ ${1:-} != --internal-copy ]]; then
  (($# == 0)) || die 'Run without arguments: bash scripts/deploy.sh'
  root=$(git rev-parse --show-toplevel) || die 'Run inside the Wishix Git checkout.'
  runner=$(mktemp "${TMPDIR:-/tmp}/wishix-deploy.XXXXXXXX")
  trap 'rm -f -- "$runner"' EXIT
  cp -- "$0" "$runner"
  bash "$runner" --internal-copy "$root"
  exit $?
fi

(($# == 2)) || die 'Invalid internal arguments.'
cd -- "$2"
state=.wishix-deploy
lock="$state/lock"
mkdir -p -- "$state"
if ! mkdir -- "$lock" 2>/dev/null; then
  die "Another deployment may be running (lock: $lock). Check it before removing a stale lock."
fi
printf '%s\n' "$$" > "$lock/pid"
phase=preflight
target_commit=
cleanup() {
  code=$?
  trap - EXIT
  if ((code != 0)); then
    printf 'Deployment failed during %s. ' "$phase" >&2
    if [[ $phase == start || $phase == verify ]]; then
      printf 'The database schema may have changed; no automatic rollback was run.\n' >&2
      failed_server=$("${compose[@]}" ps -a -q server 2>/dev/null || true)
      if [[ -n $failed_server ]]; then
        health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$failed_server" 2>/dev/null || true)
        if [[ $health != healthy ]]; then
          printf 'Stopping the unhealthy API to prevent repeated migration attempts.\n' >&2
          "${compose[@]}" stop server || printf 'Warning: could not stop the API.\n' >&2
        fi
      fi
      printf 'Review .wishix-deploy/incomplete before running another deployment.\n' >&2
    else
      printf 'The running application was not intentionally restarted.\n' >&2
    fi
    [[ -z $target_commit ]] || printf 'Requested commit: %s\n' "$target_commit" >&2
    [[ ! -f $state/current ]] || printf 'Last successful deployment: %s\n' "$(sed -n '2p' "$state/current")" >&2
  fi
  rm -f -- "$lock/pid"
  rmdir -- "$lock" || printf 'Warning: could not remove deployment lock %s\n' "$lock" >&2
  exit "$code"
}
trap cleanup EXIT

[[ -f .env ]] || die 'Production .env is missing.'
[[ ! -e $state/incomplete ]] || die 'A previous deployment may have changed the schema. Review it before removing .wishix-deploy/incomplete.'
[[ $(git symbolic-ref --quiet --short HEAD) == main ]] || die 'Checkout must be on branch main.'
[[ -z $(git status --porcelain --untracked-files=normal) ]] || die 'Git checkout has local changes or untracked files; nothing was overwritten.'
git remote get-url origin >/dev/null || die 'Git remote origin is not configured.'
docker info >/dev/null || die 'Docker is unavailable.'
docker compose version >/dev/null || die 'Docker Compose is unavailable.'

compose=(docker compose -f compose.prod.yaml)
phase=fetch
git fetch --no-tags origin main
target_commit=$(git rev-parse --verify 'FETCH_HEAD^{commit}')
checkout_commit=$(git rev-parse --verify 'HEAD^{commit}')
git merge-base --is-ancestor "$checkout_commit" "$target_commit" || die 'Local main cannot be fast-forwarded to origin/main.'
# Tracked application files must remain readable by non-root container users;
# retain the restrictive umask for deployment state and temporary files.
(umask 022; git merge --ff-only --no-overwrite-ignore "$target_commit")

phase=configuration
"${compose[@]}" config --quiet
project=$("${compose[@]}" config | sed -n 's/^name: //p')
[[ $project =~ ^[a-z0-9][a-z0-9_-]*$ ]] || die 'Could not determine the Compose project name.'
db_id=$("${compose[@]}" ps -a -q db)
server_id=$("${compose[@]}" ps -a -q server)
front_id=$("${compose[@]}" ps -a -q front)

# A changed database service must be handled separately; --no-recreate below
# guarantees that routine application deployments do not replace it.
if [[ -n $db_id ]]; then
  desired_hash=$("${compose[@]}" config --hash db | awk '$1 == "db" {print $2}')
  actual_hash=$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.config-hash" }}' "$db_id")
  [[ -n $desired_hash && $actual_hash == "$desired_hash" ]] || die 'Database service configuration changed. Review it separately before deployment.'
elif [[ -f $state/current ]]; then
  docker volume inspect "${project}_db-data" >/dev/null 2>&1 || die 'Previous deployment exists but the database container and data volume are missing. Refusing to create an empty database.'
fi

# db-init's environment is evaluated by Compose; do not print .env or secrets.
configured_db=$("${compose[@]}" run --rm --no-deps --entrypoint /bin/bash db-init -c 'printf "%s" "$DB_NAME"')
[[ $configured_db =~ ^[a-z][a-z0-9_]{0,62}$ ]] || die 'DB_NAME must be a lowercase SQL identifier.'
if [[ -n $server_id ]]; then
  previous_db=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$server_id" | sed -n 's/^DB_NAME=//p')
  [[ -z $previous_db || $previous_db == "$configured_db" ]] || die 'DB_NAME differs from the existing application container. Review the database change separately.'
elif [[ -f $state/current ]]; then
  previous_db=$(sed -n '6p' "$state/current")
  [[ -z $previous_db || $previous_db == "$configured_db" ]] || die 'DB_NAME differs from the last successful deployment.'
fi

old_server= old_front=
if [[ -f $state/current ]]; then
  old_server=$(sed -n '4p' "$state/current")
  old_front=$(sed -n '5p' "$state/current")
else
  [[ -z $server_id ]] || old_server=$(docker inspect --format '{{.Image}}' "$server_id")
  [[ -z $front_id ]] || old_front=$(docker inspect --format '{{.Image}}' "$front_id")
fi
# Retain the last known-good images, even when rebuilding moves the usual tags.
if [[ -n $old_server ]]; then
  docker image tag "$old_server" "${project}-rollback-server:latest" || die 'Could not retain the previous server image.'
fi
if [[ -n $old_front ]]; then
  docker image tag "$old_front" "${project}-rollback-front:latest" || die 'Could not retain the previous frontend image.'
fi

phase=build
"${compose[@]}" build server front

phase=database
"${compose[@]}" up -d --no-recreate --wait --wait-timeout 300 db
"${compose[@]}" run --rm --no-deps db-init

phase=start
# Dependencies were checked explicitly; do not recreate the database here.
printf '%s\n' "$target_commit" > "$state/incomplete"
"${compose[@]}" up -d --no-deps --no-build --wait --wait-timeout 300 server front

phase=verify
version=$("${compose[@]}" exec -T server ./migrate version | tr -d '\r\n')
[[ $version =~ ^[0-9]+$ ]] || die 'Could not determine the current migration version.'
server_id=$("${compose[@]}" ps -q server)
front_id=$("${compose[@]}" ps -q front)
[[ -n $server_id && -n $front_id ]] || die 'Application containers are not running.'
server_image=$(docker inspect --format '{{.Image}}' "$server_id")
front_image=$(docker inspect --format '{{.Image}}' "$front_id")
[[ -z $(git status --porcelain --untracked-files=normal) ]] || die 'Checkout changed during deployment; deployment state was not recorded.'
[[ $(git rev-parse HEAD) == "$target_commit" ]] || die 'Checkout commit changed during deployment.'

[[ ! -f $state/current ]] || cp -- "$state/current" "$state/previous"
record=$(mktemp "$state/current.XXXXXXXX")
printf 'v1\n%s\n%s\n%s\n%s\n%s\n' "$target_commit" "$version" "$server_image" "$front_image" "$configured_db" > "$record"
mv -f -- "$record" "$state/current"
rm -f -- "$state/incomplete"
phase=complete
printf 'Deployment succeeded. Commit: %s; database migration: %s\n' "$target_commit" "$version"
printf 'Previous application images, when available: %s-rollback-server:latest and %s-rollback-front:latest\n' "$project" "$project"
