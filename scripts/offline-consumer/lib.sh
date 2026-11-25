#!/usr/bin/env bash
# Shared helpers for the offline consumer lanes. The caller sets COMMANDS,
# SUMMARY and SLUG, and may override fail(). Network isolation is real on every
# supported host: a network namespace on Linux, a deny-network sandbox on
# macOS, and a per-executable outbound Windows Firewall rule on Windows.

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

NET_CLEANUP_RULE=${NET_CLEANUP_RULE:-}

# cleanup_network_off removes the ephemeral Windows rule if one was created.
# It is idempotent and safe to call from an exit trap.
cleanup_network_off() {
  local rule=${NET_CLEANUP_RULE:-}
  [[ -n $rule ]] || return 0
  local ps
  ps=$(command -v powershell || command -v pwsh || true)
  if [[ -n $ps ]]; then
    "$ps" -NoProfile -NonInteractive -Command \
      "Remove-NetFirewallRule -DisplayName '$rule' -ErrorAction SilentlyContinue" >/dev/null 2>&1 || true
  fi
  printf 'network_cleanup=removed\n' | tee -a "${SUMMARY:-/dev/null}"
  NET_CLEANUP_RULE=
}

# run_network_probe runs the same consumer executable in probe mode until the
# expected result appears. want is "ok" (direct egress works) or "blocked"
# (direct egress fails).
run_network_probe() {
  local binary=$1 attempts=$2 want=$3 label=$4
  local i
  for ((i = 1; i <= attempts; i++)); do
    if "$binary" -probe-network >/dev/null 2>&1; then
      if [[ $want == ok ]]; then
        printf 'network_probe=%s\n' "$label" | tee -a "${SUMMARY:-/dev/null}"
        return 0
      fi
    elif [[ $want == blocked ]]; then
      printf 'network_probe=%s\n' "$label" | tee -a "${SUMMARY:-/dev/null}"
      return 0
    fi
    sleep 1
  done
  return 1
}

# prepare_windows_network_off blocks outbound traffic for exactly the consumer
# executable path. It first proves direct egress from that executable, then
# installs the rule and proves the same executable is blocked. The rule is
# removed by cleanup_network_off on exit, including error paths.
prepare_windows_network_off() {
  local binary=$1
  command -v cygpath >/dev/null 2>&1 || return 1
  local ps
  ps=$(command -v powershell || command -v pwsh || true)
  [[ -n $ps ]] || return 1
  local winbin rule
  winbin=$(cygpath -w "$binary")
  rule="lancedb-go-offline-${GITHUB_RUN_ID:-$$}-${GITHUB_JOB:-local}-${SLUG:-target}"
  rule=$(printf '%s' "$rule" | tr -c 'A-Za-z0-9._-' '-')

  # Positive control: the executable can reach the network before the rule.
  if ! run_network_probe "$binary" 3 ok egress-ok; then
    return 1
  fi
  if ! "$ps" -NoProfile -NonInteractive -Command \
    "\$p = (Resolve-Path -LiteralPath '$winbin').Path; New-NetFirewallRule -DisplayName '$rule' -Direction Outbound -Program \$p -Action Block -Profile Any -ErrorAction Stop | Out-Null"; then
    return 1
  fi
  NET_CLEANUP_RULE=$rule
  # Negative control: the same executable must now be blocked.
  if ! run_network_probe "$binary" 10 blocked egress-blocked; then
    cleanup_network_off
    return 1
  fi
  return 0
}

# detect_network_off selects a real isolation mechanism for the current host
# and sets NET_OFF/NET_KIND. It returns non-zero when the host has no usable
# mechanism, so a lane never claims an offline run it did not perform.
detect_network_off() {
  local binary=${1:-}
  case "$(uname -s)" in
    Linux)
      if unshare -rn true >/dev/null 2>&1; then
        NET_OFF=(unshare -rn)
        NET_KIND=netns
      else
        return 1
      fi
      ;;
    Darwin)
      if command -v sandbox-exec >/dev/null 2>&1; then
        NET_OFF=(sandbox-exec -p '(version 1)(allow default)(deny network*)')
        NET_KIND=sandbox-exec
      else
        return 1
      fi
      ;;
    MINGW* | MSYS* | CYGWIN*)
      [[ -n $binary ]] || return 1
      if ! prepare_windows_network_off "$binary"; then
        return 1
      fi
      NET_OFF=()
      NET_KIND=windows-firewall
      ;;
    *)
      return 1
      ;;
  esac
  return 0
}

run_consumer() {
  local binary=$1 cache=$2 db=$3 out=$4 label=$5
  mkdir -p "$cache" "$db"
  : > "$out"
  printf '+ run=%s LANCEDB_C_CACHE_DIR=%s %s %s -uri %s\n' \
    "$label" "$cache" "${NET_OFF[*]}" "$binary" "$db" >> "$COMMANDS"
  set +e
  env -u LANCEDB_C_LIBRARY_PATH \
    LANCEDB_C_CACHE_DIR="$cache" \
    GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
    HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 ALL_PROXY=http://127.0.0.1:9 NO_PROXY= \
    "${NET_OFF[@]}" "$binary" -uri "$db" > "$out" 2>&1
  local rc=$?
  set -e
  cat "$out" | tee -a "${SUMMARY:-/dev/null}"
  return "$rc"
}

expect_field() {
  local out=$1 key=$2 want=$3
  local got
  got=$(awk -F= -v k="$key" '$1 == k { print substr($0, length(k) + 2) }' "$out" | head -1)
  [[ $got == "$want" ]] || fail "$key is '$got', want '$want'"
}
