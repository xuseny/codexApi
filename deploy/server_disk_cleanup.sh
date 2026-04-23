#!/usr/bin/env bash

set -euo pipefail

MODE="dry-run"
ASSUME_YES=0

PGSQL_LOG_DIR="${PGSQL_LOG_DIR:-/www/server/pgsql/logs}"
PGSQL_LOG_KEEP_COUNT="${PGSQL_LOG_KEEP_COUNT:-2}"

APT_ARCHIVES_DIR="${APT_ARCHIVES_DIR:-/var/cache/apt/archives}"

SYSLOG_PATH="${SYSLOG_PATH:-/var/log/syslog}"
SYSLOG_TRUNCATE_THRESHOLD="${SYSLOG_TRUNCATE_THRESHOLD:-100M}"

JOURNAL_DIR="${JOURNAL_DIR:-/var/log/journal}"
JOURNAL_MAX_SIZE="${JOURNAL_MAX_SIZE:-200M}"

DOCKER_LOG_DIR="${DOCKER_LOG_DIR:-/var/lib/docker/containers}"
DOCKER_LOG_THRESHOLD="${DOCKER_LOG_THRESHOLD:-100M}"

GO_BUILD_CACHE_DIR="${GO_BUILD_CACHE_DIR:-}"
GO_DOWNLOAD_CACHE_DIR="${GO_DOWNLOAD_CACHE_DIR:-}"

declare -a PG_LOGS_TO_DELETE=()
declare -a PG_LOGS_TO_KEEP=()
declare -a SYSLOG_ROTATED_TO_DELETE=()
declare -a DOCKER_LOGS_TO_TRUNCATE=()

PG_LOGS_DELETE_BYTES=0
SYSLOG_ROTATED_BYTES=0
DOCKER_LOG_BYTES=0
SYSLOG_CURRENT_BYTES=0
SYSLOG_TRUNCATE_BYTES=0
APT_CACHE_BYTES=0
JOURNAL_CURRENT_BYTES=0
JOURNAL_RECLAIM_ESTIMATE=0
GO_BUILD_CACHE_BYTES=0
GO_DOWNLOAD_CACHE_BYTES=0
SYSLOG_SHOULD_TRUNCATE=0

usage() {
  cat <<'EOF'
Usage:
  bash deploy/server_disk_cleanup.sh
  sudo bash deploy/server_disk_cleanup.sh --apply
  sudo bash deploy/server_disk_cleanup.sh --apply --yes

Options:
  --apply                     Execute cleanup actions.
  --yes                       Skip confirmation when used with --apply.
  --keep-pg-logs N            Keep the newest N PostgreSQL daily log files.
  --journal-size SIZE         Vacuum systemd journal to SIZE (default: 200M).
  --docker-log-threshold SIZE Truncate Docker JSON logs larger than SIZE.
  --syslog-threshold SIZE     Truncate /var/log/syslog only when larger than SIZE.
  --help                      Show this help message.

Environment overrides:
  PGSQL_LOG_DIR
  PGSQL_LOG_KEEP_COUNT
  APT_ARCHIVES_DIR
  SYSLOG_PATH
  SYSLOG_TRUNCATE_THRESHOLD
  JOURNAL_DIR
  JOURNAL_MAX_SIZE
  DOCKER_LOG_DIR
  DOCKER_LOG_THRESHOLD
  GO_BUILD_CACHE_DIR
  GO_DOWNLOAD_CACHE_DIR
EOF
}

info() {
  printf '[INFO] %s\n' "$*"
}

warn() {
  printf '[WARN] %s\n' "$*" >&2
}

fail() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

human_size() {
  local bytes="${1:-0}"
  if command_exists numfmt; then
    numfmt --to=iec --suffix=B "$bytes"
  else
    printf '%sB' "$bytes"
  fi
}

to_bytes() {
  local value="${1:-0}"
  if [[ "$value" =~ ^[0-9]+$ ]]; then
    printf '%s\n' "$value"
    return 0
  fi

  if command_exists numfmt; then
    numfmt --from=auto "$value"
    return 0
  fi

  return 1
}

file_size() {
  local path="$1"
  if [[ -f "$path" ]]; then
    stat -c '%s' "$path"
  else
    printf '0\n'
  fi
}

dir_size() {
  local path="$1"
  if [[ -d "$path" ]]; then
    du -sb "$path" 2>/dev/null | awk '{print $1}'
  else
    printf '0\n'
  fi
}

disk_line() {
  local path="${1:-/}"
  df -h "$path" | awk 'NR==2 {print $0}'
}

add_candidate() {
  local current="$1"
  local delta="$2"
  printf '%s\n' "$((current + delta))"
}

join_by_comma() {
  local first=1
  for item in "$@"; do
    if (( first )); then
      printf '%s' "$item"
      first=0
    else
      printf ', %s' "$item"
    fi
  done
}

require_non_negative_integer() {
  local value="$1"
  [[ "$value" =~ ^[0-9]+$ ]]
}

parse_args() {
  while (($# > 0)); do
    case "$1" in
      --apply)
        MODE="apply"
        shift
        ;;
      --yes)
        ASSUME_YES=1
        shift
        ;;
      --keep-pg-logs)
        (($# >= 2)) || fail "--keep-pg-logs requires a value"
        require_non_negative_integer "$2" || fail "--keep-pg-logs requires a non-negative integer"
        PGSQL_LOG_KEEP_COUNT="$2"
        shift 2
        ;;
      --journal-size)
        (($# >= 2)) || fail "--journal-size requires a value"
        JOURNAL_MAX_SIZE="$2"
        shift 2
        ;;
      --docker-log-threshold)
        (($# >= 2)) || fail "--docker-log-threshold requires a value"
        DOCKER_LOG_THRESHOLD="$2"
        shift 2
        ;;
      --syslog-threshold)
        (($# >= 2)) || fail "--syslog-threshold requires a value"
        SYSLOG_TRUNCATE_THRESHOLD="$2"
        shift 2
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        fail "Unknown argument: $1"
        ;;
    esac
  done
}

ensure_apply_prerequisites() {
  if [[ "$MODE" == "apply" && "${EUID:-$(id -u)}" -ne 0 ]]; then
    fail "Run with sudo when using --apply"
  fi
}

collect_pg_logs() {
  [[ -d "$PGSQL_LOG_DIR" ]] || return 0

  local -a all_logs=()
  mapfile -t all_logs < <(
    find "$PGSQL_LOG_DIR" -maxdepth 1 -type f -name 'postgresql-*.log' -printf '%f\t%p\t%s\n' 2>/dev/null | sort -r
  )

  local index=0
  local line size
  for line in "${all_logs[@]}"; do
    if (( index < PGSQL_LOG_KEEP_COUNT )); then
      PG_LOGS_TO_KEEP+=("$line")
    else
      PG_LOGS_TO_DELETE+=("$line")
      size="${line##*$'\t'}"
      PG_LOGS_DELETE_BYTES=$(add_candidate "$PG_LOGS_DELETE_BYTES" "$size")
    fi
    index=$((index + 1))
  done
}

collect_syslog_candidates() {
  local syslog_dir
  syslog_dir="$(dirname "$SYSLOG_PATH")"
  [[ -d "$syslog_dir" ]] || return 0

  if [[ -f "$SYSLOG_PATH" ]]; then
    SYSLOG_CURRENT_BYTES="$(file_size "$SYSLOG_PATH")"
    local threshold_bytes
    threshold_bytes="$(to_bytes "$SYSLOG_TRUNCATE_THRESHOLD" 2>/dev/null || printf '0')"
    if [[ "$threshold_bytes" =~ ^[0-9]+$ ]] && (( SYSLOG_CURRENT_BYTES > threshold_bytes )); then
      SYSLOG_SHOULD_TRUNCATE=1
      SYSLOG_TRUNCATE_BYTES="$SYSLOG_CURRENT_BYTES"
    fi
  fi

  local -a rotated=()
  mapfile -t rotated < <(
    find "$syslog_dir" -maxdepth 1 -type f \( -name 'syslog.[0-9]*' -o -name 'syslog.*.gz' \) -printf '%p\t%s\n' 2>/dev/null | sort
  )

  local line size
  for line in "${rotated[@]}"; do
    SYSLOG_ROTATED_TO_DELETE+=("$line")
    size="${line##*$'\t'}"
    SYSLOG_ROTATED_BYTES=$(add_candidate "$SYSLOG_ROTATED_BYTES" "$size")
  done
}

collect_apt_candidates() {
  if [[ -d "$APT_ARCHIVES_DIR" ]] && (command_exists apt || command_exists apt-get); then
    APT_CACHE_BYTES="$(dir_size "$APT_ARCHIVES_DIR")"
  fi
}

collect_journal_candidates() {
  [[ -d "$JOURNAL_DIR" ]] || return 0

  JOURNAL_CURRENT_BYTES="$(dir_size "$JOURNAL_DIR")"
  local target_bytes
  target_bytes="$(to_bytes "$JOURNAL_MAX_SIZE" 2>/dev/null || printf '0')"
  if [[ "$target_bytes" =~ ^[0-9]+$ ]] && (( JOURNAL_CURRENT_BYTES > target_bytes )); then
    JOURNAL_RECLAIM_ESTIMATE="$((JOURNAL_CURRENT_BYTES - target_bytes))"
  fi
}

collect_docker_candidates() {
  [[ -d "$DOCKER_LOG_DIR" ]] || return 0

  local -a logs=()
  mapfile -t logs < <(
    find "$DOCKER_LOG_DIR" -type f -name '*-json.log' -size +"$DOCKER_LOG_THRESHOLD" -printf '%p\t%s\n' 2>/dev/null | sort
  )

  local line size
  for line in "${logs[@]}"; do
    DOCKER_LOGS_TO_TRUNCATE+=("$line")
    size="${line##*$'\t'}"
    DOCKER_LOG_BYTES=$(add_candidate "$DOCKER_LOG_BYTES" "$size")
  done
}

detect_go_cache_dirs() {
  if [[ -z "$GO_BUILD_CACHE_DIR" && -z "$GO_DOWNLOAD_CACHE_DIR" ]] && command_exists go; then
    GO_BUILD_CACHE_DIR="$(go env GOCACHE 2>/dev/null || true)"
    local gopath
    gopath="$(go env GOPATH 2>/dev/null || true)"
    if [[ -n "$gopath" ]]; then
      GO_DOWNLOAD_CACHE_DIR="$gopath/pkg/mod/cache/download"
    fi
  fi

  if [[ -z "$GO_BUILD_CACHE_DIR" && -d /root/.cache/go-build ]]; then
    GO_BUILD_CACHE_DIR="/root/.cache/go-build"
  fi

  if [[ -z "$GO_DOWNLOAD_CACHE_DIR" && -d /root/go/pkg/mod/cache/download ]]; then
    GO_DOWNLOAD_CACHE_DIR="/root/go/pkg/mod/cache/download"
  fi
}

collect_go_candidates() {
  detect_go_cache_dirs

  if [[ -n "$GO_BUILD_CACHE_DIR" && -d "$GO_BUILD_CACHE_DIR" ]]; then
    GO_BUILD_CACHE_BYTES="$(dir_size "$GO_BUILD_CACHE_DIR")"
  fi

  if [[ -n "$GO_DOWNLOAD_CACHE_DIR" && -d "$GO_DOWNLOAD_CACHE_DIR" ]]; then
    GO_DOWNLOAD_CACHE_BYTES="$(dir_size "$GO_DOWNLOAD_CACHE_DIR")"
  fi
}

print_path_with_size() {
  local path="$1"
  local bytes="$2"
  printf '  - %s (%s)\n' "$path" "$(human_size "$bytes")"
}

print_candidates() {
  local approx_reclaim=0
  approx_reclaim=$((approx_reclaim + PG_LOGS_DELETE_BYTES))
  approx_reclaim=$((approx_reclaim + SYSLOG_ROTATED_BYTES))
  approx_reclaim=$((approx_reclaim + SYSLOG_TRUNCATE_BYTES))
  approx_reclaim=$((approx_reclaim + APT_CACHE_BYTES))
  approx_reclaim=$((approx_reclaim + JOURNAL_RECLAIM_ESTIMATE))
  approx_reclaim=$((approx_reclaim + DOCKER_LOG_BYTES))
  approx_reclaim=$((approx_reclaim + GO_BUILD_CACHE_BYTES))
  approx_reclaim=$((approx_reclaim + GO_DOWNLOAD_CACHE_BYTES))

  printf 'Mode: %s\n' "$MODE"
  printf 'Root filesystem before: %s\n' "$(disk_line /)"
  printf 'Approx reclaimable space: %s\n' "$(human_size "$approx_reclaim")"
  printf '\n'

  printf 'PostgreSQL daily logs in %s\n' "$PGSQL_LOG_DIR"
  if ((${#PG_LOGS_TO_DELETE[@]} == 0)); then
    printf '  - No PostgreSQL daily logs selected for deletion.\n'
  else
    printf '  - Keep newest %s file(s), delete %s older file(s), approx %s\n' \
      "$PGSQL_LOG_KEEP_COUNT" "${#PG_LOGS_TO_DELETE[@]}" "$(human_size "$PG_LOGS_DELETE_BYTES")"
    local line name path bytes
    for line in "${PG_LOGS_TO_DELETE[@]}"; do
      name="${line%%$'\t'*}"
      path="$(printf '%s' "$line" | cut -f2)"
      bytes="${line##*$'\t'}"
      print_path_with_size "$path" "$bytes"
      printf '    file: %s\n' "$name"
    done
  fi
  if ((${#PG_LOGS_TO_KEEP[@]} > 0)); then
    local kept_names=()
    local kept_line kept_name
    for kept_line in "${PG_LOGS_TO_KEEP[@]}"; do
      kept_name="${kept_line%%$'\t'*}"
      kept_names+=("$kept_name")
    done
    printf '  - Keep list: %s\n' "$(join_by_comma "${kept_names[@]}")"
  fi
  printf '\n'

  printf 'APT cache in %s\n' "$APT_ARCHIVES_DIR"
  if (( APT_CACHE_BYTES > 0 )); then
    printf '  - Will run apt clean, approx %s currently cached.\n' "$(human_size "$APT_CACHE_BYTES")"
  else
    printf '  - No apt cache detected or apt is unavailable.\n'
  fi
  printf '\n'

  printf 'System logs\n'
  if ((${#SYSLOG_ROTATED_TO_DELETE[@]} > 0)); then
    printf '  - Delete %s rotated syslog file(s), approx %s\n' \
      "${#SYSLOG_ROTATED_TO_DELETE[@]}" "$(human_size "$SYSLOG_ROTATED_BYTES")"
    local syslog_line syslog_path syslog_bytes
    for syslog_line in "${SYSLOG_ROTATED_TO_DELETE[@]}"; do
      syslog_path="$(printf '%s' "$syslog_line" | cut -f1)"
      syslog_bytes="${syslog_line##*$'\t'}"
      print_path_with_size "$syslog_path" "$syslog_bytes"
    done
  else
    printf '  - No rotated syslog files selected.\n'
  fi

  if (( SYSLOG_SHOULD_TRUNCATE == 1 )); then
    printf '  - Truncate %s because it is %s (> %s).\n' \
      "$SYSLOG_PATH" "$(human_size "$SYSLOG_CURRENT_BYTES")" "$SYSLOG_TRUNCATE_THRESHOLD"
  else
    if [[ -f "$SYSLOG_PATH" ]]; then
      printf '  - Keep %s as-is (%s <= %s).\n' \
        "$SYSLOG_PATH" "$(human_size "$SYSLOG_CURRENT_BYTES")" "$SYSLOG_TRUNCATE_THRESHOLD"
    else
      printf '  - %s not found.\n' "$SYSLOG_PATH"
    fi
  fi

  if (( JOURNAL_CURRENT_BYTES > 0 )); then
    printf '  - Vacuum systemd journal from %s down toward %s, estimate %s reclaimable.\n' \
      "$(human_size "$JOURNAL_CURRENT_BYTES")" "$JOURNAL_MAX_SIZE" "$(human_size "$JOURNAL_RECLAIM_ESTIMATE")"
  else
    printf '  - No journal directory detected at %s.\n' "$JOURNAL_DIR"
  fi
  printf '\n'

  printf 'Docker JSON logs in %s\n' "$DOCKER_LOG_DIR"
  if ((${#DOCKER_LOGS_TO_TRUNCATE[@]} > 0)); then
    printf '  - Truncate %s file(s) larger than %s, approx %s\n' \
      "${#DOCKER_LOGS_TO_TRUNCATE[@]}" "$DOCKER_LOG_THRESHOLD" "$(human_size "$DOCKER_LOG_BYTES")"
    local docker_line docker_path docker_bytes
    for docker_line in "${DOCKER_LOGS_TO_TRUNCATE[@]}"; do
      docker_path="$(printf '%s' "$docker_line" | cut -f1)"
      docker_bytes="${docker_line##*$'\t'}"
      print_path_with_size "$docker_path" "$docker_bytes"
    done
  else
    printf '  - No Docker JSON logs larger than %s.\n' "$DOCKER_LOG_THRESHOLD"
  fi
  printf '\n'

  printf 'Go caches\n'
  if [[ -n "$GO_BUILD_CACHE_DIR" && -d "$GO_BUILD_CACHE_DIR" ]]; then
    printf '  - Build cache: %s at %s\n' "$(human_size "$GO_BUILD_CACHE_BYTES")" "$GO_BUILD_CACHE_DIR"
  else
    printf '  - Build cache: not detected.\n'
  fi

  if [[ -n "$GO_DOWNLOAD_CACHE_DIR" && -d "$GO_DOWNLOAD_CACHE_DIR" ]]; then
    printf '  - Module download cache: %s at %s\n' "$(human_size "$GO_DOWNLOAD_CACHE_BYTES")" "$GO_DOWNLOAD_CACHE_DIR"
  else
    printf '  - Module download cache: not detected.\n'
  fi
  printf '\n'

  printf 'Explicitly excluded from cleanup\n'
  printf '  - /swap.img\n'
  printf '  - /www/server/pgsql/data\n'
  printf '  - /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots\n'
  printf '  - /root/*.dump\n'
  printf '  - /root/sub2api.from-A\n'
  printf '  - /opt/sub2api/sub2api\n'
  printf '\n'

  if command_exists lsof; then
    local deleted_count
    deleted_count="$(lsof +L1 2>/dev/null | awk 'NR>1 {count++} END {print count+0}')"
    printf 'Open deleted files still held by processes: %s\n' "$deleted_count"
    if (( deleted_count > 0 )); then
      printf '  - Run: sudo lsof +L1\n'
    fi
  fi
}

confirm_apply() {
  if (( ASSUME_YES == 1 )); then
    return 0
  fi

  printf 'Proceed with cleanup? [y/N] '
  local answer
  read -r answer
  case "$answer" in
    y|Y|yes|YES)
      return 0
      ;;
    *)
      info "Cleanup cancelled."
      return 1
      ;;
  esac
}

delete_file() {
  local path="$1"
  [[ -f "$path" ]] || return 0
  rm -f -- "$path"
}

truncate_file() {
  local path="$1"
  [[ -f "$path" ]] || return 0
  : > "$path"
}

clear_dir_contents() {
  local dir="$1"
  [[ -n "$dir" && -d "$dir" ]] || return 0
  [[ "$dir" != "/" ]] || fail "Refusing to clear /"
  find "$dir" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
}

apply_cleanup() {
  local before_line after_line
  before_line="$(disk_line /)"
  info "Root filesystem before cleanup: $before_line"

  local line path

  for line in "${PG_LOGS_TO_DELETE[@]}"; do
    path="$(printf '%s' "$line" | cut -f2)"
    info "Deleting PostgreSQL log: $path"
    delete_file "$path"
  done

  for line in "${SYSLOG_ROTATED_TO_DELETE[@]}"; do
    path="$(printf '%s' "$line" | cut -f1)"
    info "Deleting rotated syslog: $path"
    delete_file "$path"
  done

  if (( SYSLOG_SHOULD_TRUNCATE == 1 )); then
    info "Truncating syslog: $SYSLOG_PATH"
    truncate_file "$SYSLOG_PATH"
  fi

  if (( APT_CACHE_BYTES > 0 )); then
    if command_exists apt; then
      info "Cleaning apt cache"
      apt clean
    elif command_exists apt-get; then
      info "Cleaning apt cache with apt-get clean"
      apt-get clean
    fi
  fi

  if (( JOURNAL_CURRENT_BYTES > 0 )) && command_exists journalctl; then
    info "Vacuuming systemd journal to $JOURNAL_MAX_SIZE"
    journalctl --vacuum-size="$JOURNAL_MAX_SIZE"
  fi

  for line in "${DOCKER_LOGS_TO_TRUNCATE[@]}"; do
    path="$(printf '%s' "$line" | cut -f1)"
    info "Truncating Docker JSON log: $path"
    truncate_file "$path"
  done

  if [[ -n "$GO_BUILD_CACHE_DIR" && -d "$GO_BUILD_CACHE_DIR" ]]; then
    info "Clearing Go build cache: $GO_BUILD_CACHE_DIR"
    clear_dir_contents "$GO_BUILD_CACHE_DIR"
  fi

  if [[ -n "$GO_DOWNLOAD_CACHE_DIR" && -d "$GO_DOWNLOAD_CACHE_DIR" ]]; then
    info "Clearing Go module download cache: $GO_DOWNLOAD_CACHE_DIR"
    clear_dir_contents "$GO_DOWNLOAD_CACHE_DIR"
  fi

  after_line="$(disk_line /)"
  info "Root filesystem after cleanup: $after_line"

  if command_exists lsof; then
    local deleted_count
    deleted_count="$(lsof +L1 2>/dev/null | awk 'NR>1 {count++} END {print count+0}')"
    if (( deleted_count > 0 )); then
      warn "There are still $deleted_count deleted files held by processes. Run: sudo lsof +L1"
    fi
  fi
}

main() {
  parse_args "$@"
  ensure_apply_prerequisites

  collect_pg_logs
  collect_apt_candidates
  collect_syslog_candidates
  collect_journal_candidates
  collect_docker_candidates
  collect_go_candidates

  print_candidates

  if [[ "$MODE" == "apply" ]]; then
    confirm_apply || exit 0
    apply_cleanup
  else
    printf 'Dry run only. Re-run with: sudo bash deploy/server_disk_cleanup.sh --apply\n'
  fi
}

main "$@"
