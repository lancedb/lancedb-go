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
    LDB_OFFLINE_RULE="$rule" "$ps" -NoProfile -NonInteractive -Command '
      $ruleName = $env:LDB_OFFLINE_RULE
      Remove-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
      & netsh advfirewall firewall delete rule name="$ruleName" | Out-Null
      try { (New-Object -ComObject HNetCfg.FwPolicy2).Rules.Remove($ruleName) } catch {}
    ' >/dev/null 2>&1 || true
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
  winbin=$(cygpath -w -a "$binary")
  if [[ $winbin != *.exe ]]; then
    printf 'ERROR: Windows network isolation requires an .exe consumer path, got: %s\n' "$winbin" >&2
    return 1
  fi
  rule="lancedb-go-offline-${GITHUB_RUN_ID:-$$}-${GITHUB_JOB:-local}-${SLUG:-target}"
  rule=$(printf '%s' "$rule" | tr -c 'A-Za-z0-9._-' '-')

  # Positive control: the executable can reach the network before the rule.
  if ! run_network_probe "$binary" 3 ok egress-ok; then
    return 1
  fi

  # Create a rule for exactly this executable. Inputs go through the
  # environment so no shell quoting is interpolated. The script prints the
  # resolved path, the rule name and which mechanism worked, then falls back
  # to netsh when the cmdlet rejects the parameters.
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
      $created = $null
      try {
        New-NetFirewallRule -DisplayName $ruleName -Direction Outbound -Program $program -Action Block -Profile Any -ErrorAction Stop | Out-Null
        $created = "New-NetFirewallRule"
      } catch {
        Write-Host "network_rule_cmdlet_error=$($_.Exception.Message) $($_.FullyQualifiedErrorId)"
        & netsh advfirewall firewall add rule name="$ruleName" dir=out action=block program="$program" enable=yes profile=any | Out-Host
        if ($LASTEXITCODE -eq 0) { $created = "netsh" }
        else { Write-Host "network_rule_netsh_error=$LASTEXITCODE" }
      }
      if ($null -eq $created) {
        try {
          $policy = New-Object -ComObject HNetCfg.FwPolicy2
          $fwRule = New-Object -ComObject HNetCfg.FWRule
          $fwRule.Name = $ruleName
          $fwRule.Description = "lancedb-go offline consumer test rule"
          $fwRule.Direction = 2
          $fwRule.Action = 0
          $fwRule.Program = $program
          $fwRule.Enabled = $true
          $fwRule.Profiles = 0x7FFFFFFF
          $policy.Rules.Add($fwRule)
          $created = "HNetCfg.FwPolicy2"
        } catch {
          Write-Host "network_rule_com_error=$($_.Exception.Message)"
        }
      }
      if ($null -eq $created) {
        $control = Join-Path $env:SystemRoot "System32\curl.exe"
        try {
          New-NetFirewallRule -DisplayName "$ruleName-control" -Direction Outbound -Program $control -Action Block -Profile Any -ErrorAction Stop | Out-Null
          Write-Host "network_rule_control=New-NetFirewallRule"
          Remove-NetFirewallRule -DisplayName "$ruleName-control" -ErrorAction SilentlyContinue
        } catch {
          Write-Host "network_rule_control_error=$($_.Exception.Message) $($_.FullyQualifiedErrorId)"
        }
        exit 1
      }
      $readback = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction Stop
      Write-Host "network_rule_created=$created readback=$($readback.DisplayName) enabled=$($readback.Enabled) direction=$($readback.Direction) action=$($readback.Action)"
    ' 2>&1
  )
  rc=$?
  printf '%s\n' "$output" | tee -a "${SUMMARY:-/dev/null}"
  [[ $rc == 0 ]] || return 1

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
