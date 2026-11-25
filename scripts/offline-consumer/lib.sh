#!/usr/bin/env bash
# Shared helpers for the offline consumer lanes. The caller sets COMMANDS,
# SUMMARY and SLUG, and may override fail(). Network isolation is real on every
# supported host: a network namespace on Linux, a deny-network sandbox on
# macOS, and a per-executable outbound Windows Firewall rule on Windows.

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

# NET_CLEANUP_RULE is registered before the Windows rule is created so the
# exit trap can remove a rule whose creation readback fails.
NET_CLEANUP_RULE=${NET_CLEANUP_RULE:-}

# cleanup_network_off removes the ephemeral Windows rule and verifies it is
# gone. It is safe to call from an exit trap and idempotent when no rule was
# registered. A failure is reported and returned, never hidden.
cleanup_network_off() {
  local rule=${NET_CLEANUP_RULE:-}
  [[ -n $rule ]] || return 0
  local ps output rc
  ps=$(command -v powershell || command -v pwsh || true)
  if [[ -z $ps ]]; then
    printf 'network_cleanup=FAILED reason=powershell-missing rule=%s\n' "$rule" | tee -a "${SUMMARY:-/dev/null}" >&2
    return 1
  fi
  output=$(
    LDB_OFFLINE_RULE="$rule" "$ps" -NoProfile -NonInteractive -Command '
      $ErrorActionPreference = "Stop"
      $ruleName = $env:LDB_OFFLINE_RULE
      $existing = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
      if ($null -ne $existing) {
        $existing | Remove-NetFirewallRule -Confirm:$false -ErrorAction Stop
      }
      $remaining = Get-NetFirewallRule -Name $ruleName -ErrorAction SilentlyContinue
      if ($null -ne $remaining) {
        throw "firewall rule $ruleName still exists after removal"
      }
      Write-Host "network_cleanup=verified rule=$ruleName"
    ' 2>&1
  )
  rc=$?
  printf '%s\n' "$output" | tee -a "${SUMMARY:-/dev/null}"
  if [[ $rc -ne 0 ]]; then
    printf 'network_cleanup=FAILED rule=%s\n' "$rule" | tee -a "${SUMMARY:-/dev/null}" >&2
    return 1
  fi
  NET_CLEANUP_RULE=
  return 0
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
# executable. It is authorized only on GitHub Actions hosted runners, uses the
# native runner temp path, and proves direct egress before and after the rule.
# The exact rule name is registered before creation so cleanup can remove a
# rule whose readback fails; removal is verified by the exit trap.
prepare_windows_network_off() {
  local binary=$1
  if [[ ${GITHUB_ACTIONS:-} != true || ${RUNNER_ENVIRONMENT:-} != github-hosted || -z ${GITHUB_RUN_ID:-} ]]; then
    printf 'ERROR: Windows network isolation is authorized only on GitHub Actions hosted runners (GITHUB_ACTIONS=true, RUNNER_ENVIRONMENT=github-hosted, GITHUB_RUN_ID set)\n' >&2
    return 1
  fi
  command -v cygpath >/dev/null 2>&1 || return 1
  local ps
  ps=$(command -v powershell || command -v pwsh || true)
  [[ -n $ps ]] || return 1
  local winbin rule
  winbin=$(cygpath -w -a "$binary")
  if [[ $winbin != *.exe ]]; then
    printf 'ERROR: Windows network isolation requires an .exe consumer path, got: %s\n' "$winbin" >&2
    return 1
  fi
  rule="lancedb-go-offline-${GITHUB_RUN_ID}-${GITHUB_JOB:-job}-${SLUG:-target}-$(date +%s)-$$"
  rule=$(printf '%s' "$rule" | tr -c 'A-Za-z0-9._-' '-')
  # Register before create: if creation succeeds but the readback below fails,
  # the exit trap must still know which exact rule to remove.
  NET_CLEANUP_RULE=$rule

  # Positive control: the executable can reach the network before the rule.
  if ! run_network_probe "$binary" 3 ok egress-ok; then
    return 1
  fi

  local output rc
  output=$(
    LDB_OFFLINE_PROGRAM="$winbin" LDB_OFFLINE_RULE="$rule" "$ps" -NoProfile -NonInteractive -Command '
      $ErrorActionPreference = "Stop"
      $program = $env:LDB_OFFLINE_PROGRAM
      $ruleName = $env:LDB_OFFLINE_RULE
      Write-Host "network_rule_program=$program"
      Write-Host "network_rule_name=$ruleName"
      $item = Get-Item -LiteralPath $program
      $program = $item.FullName
      Write-Host "network_rule_program_full=$program"
      Write-Host "network_rule_program_type=$($item.Attributes) length=$($item.Length)"
      # Omit -Enabled: the documented default is True and the readback below
      # verifies it. The typed Enabled enum does not accept a Boolean value.
      $created = New-NetFirewallRule -Name $ruleName -DisplayName $ruleName -Direction Outbound -Program $program -Action Block -Profile Any -ErrorAction Stop -PassThru
      if ($null -eq $created) { throw "New-NetFirewallRule returned no rule" }
      Write-Host "network_rule_id=$($created.InstanceID)"
      $readback = Get-NetFirewallRule -Name $ruleName -ErrorAction Stop
      if ($null -eq $readback) { throw "rule readback is empty" }
      if ($readback.InstanceID -ne $created.InstanceID) {
        throw "rule InstanceID changed: $($readback.InstanceID) != $($created.InstanceID)"
      }
      Write-Host "network_rule_readback=name=$($readback.Name) id=$($readback.InstanceID) enabled=$([string]$readback.Enabled) direction=$([string]$readback.Direction) action=$([string]$readback.Action)"
      if ([string]$readback.Enabled -ne "True") { throw "rule Enabled is $([string]$readback.Enabled)" }
      if ([string]$readback.Direction -ne "Outbound") { throw "rule Direction is $([string]$readback.Direction)" }
      if ([string]$readback.Action -ne "Block") { throw "rule Action is $([string]$readback.Action)" }
      $filter = @(Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $readback -ErrorAction Stop)
      if ($filter.Count -ne 1) { throw "expected one application filter, got $($filter.Count)" }
      $filterProgram = $filter[0].Program
      Write-Host "network_rule_filter_program=$filterProgram"
      if ([string]::IsNullOrEmpty($filterProgram)) { throw "application filter has no program" }
      $want = [System.IO.Path]::GetFullPath($program)
      $got = [System.IO.Path]::GetFullPath($filterProgram)
      if (-not [string]::Equals($want, $got, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "application filter program $filterProgram does not match $program"
      }
      Write-Host "network_rule_filter=$filterProgram"
      Write-Host "network_rule_created=New-NetFirewallRule id=$($readback.InstanceID) enabled=$($readback.Enabled) direction=$($readback.Direction) action=$($readback.Action)"
    ' 2>&1
  )
  rc=$?
  printf '%s\n' "$output" | tee -a "${SUMMARY:-/dev/null}"
  [[ $rc == 0 ]] || return 1

  # Negative control: the same executable must now be blocked.
  if ! run_network_probe "$binary" 10 blocked egress-blocked; then
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
