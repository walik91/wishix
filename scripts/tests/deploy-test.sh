#!/usr/bin/env bash
set -Eeuo pipefail

# Isolated deployment exercise: Git and Docker are replaced with local fakes.
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
test_dir=$(mktemp -d)
trap 'rm -rf -- "$test_dir"' EXIT
mkdir -p "$test_dir/bin"

cat > "$test_dir/bin/git" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  'symbolic-ref --quiet --short HEAD') echo main ;;
  'status --porcelain --untracked-files=normal') ;;
  'remote get-url origin') echo origin ;;
  'fetch --no-tags origin main'|'merge-base --is-ancestor '*|'merge --ff-only --no-overwrite-ignore '*) ;;
  'rev-parse --verify '*|'rev-parse HEAD') echo testcommit ;;
  *) echo "Unexpected git command: $*" >&2; exit 1 ;;
esac
EOF

cat > "$test_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$TEST_LOG"
server_image="sha256:$(printf 'a%.0s' {1..64})"
front_image="sha256:$(printf 'b%.0s' {1..64})"
case "$*" in
  'info')
    [[ $TEST_SCENARIO != missing_daemon || ! -e ${TEST_LOG}.missing ]] || exit 1 ;;
  'compose version') ;;
  'image inspect '*)
    if [[ $* == *"$server_image" && ( $TEST_SCENARIO == missing || $TEST_SCENARIO == missing_daemon ) ]]; then
      : > "${TEST_LOG}.missing"
      echo "Error: No such image: $server_image" >&2; exit 1
    elif [[ $* == *"$front_image" && $TEST_SCENARIO == missing_front ]]; then
      echo "Error: No such image: $front_image" >&2; exit 1
    elif [[ $* == *"$server_image" && $TEST_SCENARIO == inspect_error ]]; then
      echo 'Docker inspection failed' >&2; exit 1
    fi ;;
  'image tag '*)
    [[ $TEST_SCENARIO != tag_error || $* != *rollback-server* ]] || exit 1 ;;
  'inspect --format '*)
    case "$*" in
      *com.docker.compose.config-hash*) echo dbhash ;;
      *Config.Env*) echo DB_NAME=wishix ;;
      *'{{.Image}}'*server-container*) echo "$server_image" ;;
      *'{{.Image}}'*front-container*) echo "$front_image" ;;
      *) echo "Unexpected inspect: $*" >&2; exit 1 ;;
    esac ;;
  'compose -f compose.prod.yaml '*)
    shift 3
    case "$*" in
      'config --quiet'|'build server front'|'up -d --no-recreate --wait --wait-timeout 300 db'|'run --rm --no-deps db-init'|'up -d --no-deps --no-build --wait --wait-timeout 300 server front') ;;
      'config') echo 'name: wishix' ;;
      'config --hash db') echo 'db dbhash' ;;
      'ps -a -q db') echo db-container ;;
      'ps -a -q server'|'ps -q server') echo server-container ;;
      'ps -a -q front'|'ps -q front') echo front-container ;;
      'run --rm --no-deps --entrypoint /bin/bash db-init '*) printf wishix ;;
      'exec -T server ./migrate version') echo 16 ;;
      *) echo "Unexpected compose command: $*" >&2; exit 1 ;;
    esac ;;
  *) echo "Unexpected docker command: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$test_dir/bin/git" "$test_dir/bin/docker"
export PATH="$test_dir/bin:$PATH"

for scenario in available missing missing_front missing_daemon inspect_error tag_error; do
  work="$test_dir/$scenario"
  mkdir -p "$work/.wishix-deploy"
  : > "$work/.env"
  server_image="sha256:$(printf 'a%.0s' {1..64})"
  front_image="sha256:$(printf 'b%.0s' {1..64})"
  printf 'v1\noldcommit\n16\n%s\n%s\nwishix\n' "$server_image" "$front_image" > "$work/.wishix-deploy/current"
  export TEST_SCENARIO=$scenario TEST_LOG="$work/docker.log"
  if bash "$root/scripts/deploy.sh" --internal-copy "$work" > "$work/output" 2>&1; then
    [[ $scenario == available || $scenario == missing || $scenario == missing_front ]]
    [[ $(sed -n '2p' "$work/.wishix-deploy/current") == testcommit ]]
    [[ $(sed -n '2p' "$work/.wishix-deploy/previous") == oldcommit ]]
    [[ ! -e $work/.wishix-deploy/incomplete ]]
    grep -Fq 'build server front' "$TEST_LOG"
    if [[ $scenario == missing ]]; then
      grep -Fq 'previous server image' "$work/output"
      ! grep -Fq "image tag $server_image wishix-rollback-server:latest" "$TEST_LOG"
      grep -Fq "image tag $front_image wishix-rollback-front:latest" "$TEST_LOG"
    elif [[ $scenario == missing_front ]]; then
      grep -Fq 'previous front image' "$work/output"
      ! grep -Fq "image tag $front_image wishix-rollback-front:latest" "$TEST_LOG"
      grep -Fq "image tag $server_image wishix-rollback-server:latest" "$TEST_LOG"
    else
      grep -Fq "image tag $server_image wishix-rollback-server:latest" "$TEST_LOG"
    fi
  else
    [[ $scenario == missing_daemon || $scenario == inspect_error || $scenario == tag_error ]]
    [[ $(sed -n '2p' "$work/.wishix-deploy/current") == oldcommit ]]
    ! grep -Fq 'build server front' "$TEST_LOG"
    case $scenario in
      missing_daemon) grep -Fq 'Docker is unavailable while checking the previous image' "$work/output" ;;
      inspect_error) grep -Fq 'Could not inspect the previous server image' "$work/output" ;;
      tag_error) grep -Fq 'Could not retain the previous server image' "$work/output" ;;
    esac
  fi
  printf 'PASS %s\n' "$scenario"
done
