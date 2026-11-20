#!/usr/bin/env bash
set -Eeuo pipefail

# CockroachDB and all archive processing run in the existing Docker container.
export MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*'
export LC_ALL=C
umask 077

usage() {
  cat <<'EOF'
Usage:
  bash scripts/db_snapshot.sh backup --database NAME --output FILE [--compose FILE | --container NAME]
  bash scripts/db_snapshot.sh restore --archive FILE --target NAME [--grant-user USER] [--compose FILE | --container NAME]

Existing archives and destination databases are never overwritten.
EOF
}

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
valid_identifier() { [[ $1 =~ ^[a-z][a-z0-9_]{0,62}$ ]]; }

# Docker Desktop on Windows needs native paths, not Git Bash /c/... paths.
docker_host_path() {
  if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s\n' "$1"; fi
}

sql() {
  docker exec "$container" /cockroach/cockroach sql --insecure --host=localhost:26257 --format=tsv --execute "$1"
}

command=${1:-}
[[ $command == backup || $command == restore ]] || { usage; exit 1; }
shift
compose=compose.yaml container= database= output= archive= target= grant_user=
while (($#)); do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --compose|--container|--database|--output|--archive|--target|--grant-user)
      (($# >= 2)) || die "Missing value for $1"
      case "$1" in
        --compose) compose=$2 ;; --container) container=$2 ;; --database) database=$2 ;;
        --output) output=$2 ;; --archive) archive=$2 ;; --target) target=$2 ;; --grant-user) grant_user=$2 ;;
      esac
      shift 2 ;;
    *) die "Unknown argument: $1" ;;
  esac
done

if [[ $command == backup ]]; then
  [[ -n $database && -n $output && -z $archive && -z $target && -z $grant_user ]] || die 'Specify --database and --output for backup.'
  valid_identifier "$database" || die 'Invalid database identifier.'
  output_dir=$(cd -- "$(dirname -- "$output")" && pwd -P) || die 'Output directory does not exist.'
  output="$output_dir/$(basename -- "$output")"
  [[ ! -e $output && ! -L $output ]] || die "Output already exists: $output"
else
  [[ -n $archive && -n $target && -z $database && -z $output ]] || die 'Specify --archive and --target for restore.'
  valid_identifier "$target" || die 'Invalid target identifier.'
  [[ -z $grant_user ]] || valid_identifier "$grant_user" || die 'Invalid user identifier.'
  [[ -f $archive ]] || die 'Backup archive does not exist.'
  archive="$(cd -- "$(dirname -- "$archive")" && pwd -P)/$(basename -- "$archive")"
fi

if [[ -z $container ]]; then
  mapfile -t containers < <(docker compose -f "$(docker_host_path "$compose")" ps --status running -q db)
  ((${#containers[@]} == 1)) && [[ -n ${containers[0]} ]] || die 'Expected exactly one running db container.'
  container=${containers[0]}
fi

extern=/cockroach/cockroach-data/extern
stage="wishix-transfer/$(date -u +%Y%m%d%H%M%S)-$$-$RANDOM-$RANDOM"
base="$extern/$stage"
uri="nodelocal://1/$stage/collection"
stage_created=0 complete=0 part=
cleanup() {
  local code=$?
  trap - EXIT
  if [[ -n $part && -f $part ]]; then rm -f -- "$part"; fi
  if ((stage_created)); then
    if ((complete)); then
      if ! docker exec "$container" rm -rf -- "$base"; then
        printf 'Warning: temporary files remain in %s:%s\n' "$container" "$base" >&2
      fi
    else
      printf 'Operation failed. Inspect temporary files in %s:%s and any CockroachDB job before cleanup.\n' "$container" "$base" >&2
    fi
  fi
  exit "$code"
}
trap cleanup EXIT

if [[ $command == restore ]]; then
  result=$(sql "SELECT count(*) FROM [SHOW DATABASES] WHERE database_name = '$target';")
  [[ ${result##*$'\n'} == 0 ]] || die "Target database already exists: $target; nothing was overwritten."
  if [[ -n $grant_user ]]; then
    result=$(sql "SELECT count(*) FROM [SHOW USERS] WHERE username = '$grant_user';")
    [[ ${result##*$'\n'} == 1 ]] || die "Database user does not exist: $grant_user; no restore was started."
  fi
fi

docker exec "$container" mkdir -p -- "$extern/wishix-transfer"
docker exec "$container" mkdir -m 700 -- "$base"
stage_created=1

if [[ $command == backup ]]; then
  sql "BACKUP DATABASE \"$database\" INTO '$uri';"
  sql "SHOW BACKUP LATEST IN '$uri';" >/dev/null
  printf '{"format":"wishix-cockroach-backup-v1","database":"%s"}\n' "$database" |
    docker exec -i "$container" sh -c 'cat > "$1"' _ "$base/manifest.json"
  docker exec "$container" bash -c 'umask 077; tar -czf "$1/snapshot.tar.gz" -C "$1" manifest.json collection' _ "$base"
  # A sibling temporary file and a hard link publish atomically without replacing an existing archive.
  part=$(mktemp "${output}.part.XXXXXXXX")
  docker cp "$container:$base/snapshot.tar.gz" "$(docker_host_path "$part")"
  chmod 600 -- "$part"
  ln -- "$part" "$output" || die "Could not publish archive without overwriting: $output"
  complete=1
  printf 'Backup created: %s\n' "$output"
else
  docker cp "$(docker_host_path "$archive")" "$container:$base/snapshot.tar.gz"
  # Validate all entry names and types before extraction. Never extract links or paths outside this directory.
  docker exec -i "$container" bash -s -- "$base" <<'VALIDATE'
set -euo pipefail
export LC_ALL=C
base=$1
tar -tzf "$base/snapshot.tar.gz" > "$base/names"
tar -tvzf "$base/snapshot.tar.gz" > "$base/types"
manifest=0 collection=0
while IFS= read -r name; do
  [[ $name =~ ^[a-zA-Z0-9._/=-]+$ && $name != /* && $name != *..* && $name != *//* ]] || exit 1
  case "$name" in
    manifest.json) manifest=$((manifest + 1)) ;;
    collection|collection/|collection/*) collection=1 ;;
    *) exit 1 ;;
  esac
done < "$base/names"
[[ $manifest == 1 && $collection == 1 ]] || exit 1
while IFS= read -r item; do
  [[ $item == -* || $item == d* ]] || exit 1
done < "$base/types"
[[ -z $(sort "$base/names" | uniq -d) ]] || exit 1
tar -xzf "$base/snapshot.tar.gz" -C "$base" --no-same-owner --no-same-permissions
[[ -f $base/manifest.json && $(wc -c < "$base/manifest.json") -le 4096 ]] || exit 1
grep -Eq '"format"[[:space:]]*:[[:space:]]*"wishix-cockroach-backup-v1"' "$base/manifest.json"
VALIDATE
  database=$(docker exec "$container" sed -n 's/.*"database"[[:space:]]*:[[:space:]]*"\([a-z][a-z0-9_]*\)".*/\1/p' "$base/manifest.json")
  valid_identifier "$database" || die 'Invalid database name in backup manifest.'
  sql "SHOW BACKUP LATEST IN '$uri';" >/dev/null
  if ! sql "RESTORE DATABASE \"$database\" FROM LATEST IN '$uri' WITH new_db_name = '$target';"; then
    die "Restore failed. Inspect database $target; no database was deleted automatically."
  fi
  if [[ -n $grant_user ]]; then
    if ! sql "GRANT ALL ON DATABASE \"$target\" TO \"$grant_user\"; GRANT ALL ON SCHEMA \"$target\".public TO \"$grant_user\"; GRANT ALL ON ALL TABLES IN SCHEMA \"$target\".public TO \"$grant_user\";"; then
      die "Database $target was restored, but granting permissions failed."
    fi
  fi
  complete=1
  printf 'Restored into new database: %s\n' "$target"
fi
