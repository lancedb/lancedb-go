#!/usr/bin/env bash
# Shared helpers for the offline consumer lanes. The caller sets COMMANDS,
# NET_OFF and NET_KIND (via detect_network_off) and may override fail().

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

# detect_network_off selects a real isolation mechanism for the current host
# and sets NET_OFF/NET_KIND. It returns non-zero when the host has no usable
# mechanism, so a lane never claims an offline run it did not perform.
detect_network_off() {
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
    *)
      # Windows runners have no network namespace or sandbox in the shell. The
      # run still disables Go module fetch and routes HTTP through a dead
      # proxy, so an accidental runtime download fails instead of succeeding.
      NET_OFF=()
      NET_KIND=env-blocked
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
