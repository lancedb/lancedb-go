#!/usr/bin/env bash
# Generate reproducible companion-module candidates and the static Go module
# origin tree from one exact lancedb-c release.
#
# The identity file pins the upstream tag, commit, archive checksums,
# header/license identity. This command downloads only those exact assets
# (bounded), verifies them, builds one data-only module per target, assembles
# the origin tree, and records the candidate identity. It never publishes and
# never reads credentials.
#
# Usage:
#   IDENTITY=scripts/go-module-origin/<version>.identity.json \
#     scripts/go-module-origin/generate-candidates.sh
#
# Optional environment:
#   WORK_DIR        working directory (default: a fresh temporary directory)
#   CANDIDATES_DIR  candidate modules output (default: $WORK_DIR/candidates)
#   DOWNLOADS_DIR   archive download cache (default: $WORK_DIR/downloads)
#   ORIGIN_DIR      origin staging output (default: $WORK_DIR/origin)
#   IDENTITY_OUT    recorded candidate identity (default: $WORK_DIR/candidate-identity.json)
#   RUN_ID          candidate workflow run id recorded in the identity
#   GO              Go command (default: go)
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: IDENTITY=<release identity.json> scripts/go-module-origin/generate-candidates.sh

Environment:
  IDENTITY        release identity to verify and package (required)
  WORK_DIR        working directory (default: a fresh temporary directory)
  CANDIDATES_DIR  candidate modules output (default: $WORK_DIR/candidates)
  DOWNLOADS_DIR   archive download cache (default: $WORK_DIR/downloads)
  ORIGIN_DIR      origin staging output (default: $WORK_DIR/origin)
  IDENTITY_OUT    recorded candidate identity (default: $WORK_DIR/candidate-identity.json)
  RUN_ID          candidate workflow run id recorded in the identity
  GO              Go command (default: go)
EOF
}

if [[ ${1:-} == "-h" || ${1:-} == "--help" ]]; then
  usage
  exit 0
fi

identity=${IDENTITY:?IDENTITY must point at the release identity JSON}
go_cmd=${GO:-go}
command -v "$go_cmd" >/dev/null 2>&1 || { echo "generate-candidates: required command not found: $go_cmd" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "generate-candidates: required command not found: python3" >&2; exit 1; }

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
generator="$repo_root/scripts/go-module-origin/generator"
identity=$(cd "$(dirname "$identity")" && printf '%s/%s\n' "$(pwd -P)" "$(basename "$identity")")
[[ -f $identity ]] || { echo "generate-candidates: missing identity file: $identity" >&2; exit 1; }

work=${WORK_DIR:-}
if [[ -z $work ]]; then
  work=$(mktemp -d)
fi
mkdir -p "$work"
candidates=${CANDIDATES_DIR:-$work/candidates}
downloads=${DOWNLOADS_DIR:-$work/downloads}
origin_dir=${ORIGIN_DIR:-$work/origin}
identity_out=${IDENTITY_OUT:-$work/candidate-identity.json}

export GOWORK=off
export GOTOOLCHAIN=${GOTOOLCHAIN:-local}
export GOFLAGS=${GOFLAGS:-}

field() {
  python3 -c 'import json,sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
value = data.get(sys.argv[2])
if value is None:
    sys.exit(1)
print(value)' "$identity" "$1"
}

python3 - "$identity" <<'PY'
import json
import pathlib
import sys

identity = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
for field in ("version", "time", "source_repo", "source_release", "source_commit", "header_sha256", "targets"):
    if field not in identity:
        raise SystemExit(f"identity is missing {field}")
expected = {"linux-amd64", "linux-arm64", "darwin-arm64", "windows-amd64", "windows-arm64"}
targets = set(identity["targets"])
if targets != expected:
    raise SystemExit(f"identity targets are {sorted(targets)}, want {sorted(expected)}")
for target, value in identity["targets"].items():
    for field in ("archive", "archive_sha256", "library", "library_sha256"):
        if field not in value:
            raise SystemExit(f"identity target {target} is missing {field}")
print("identity targets: ok")
PY

version=$(field version)
release_time=$(field time)
module_prefix=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("module_prefix", "go.lancedb.io/lancedb-go/artifacts"))' "$identity")
base_url=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("base_url", "https://go.lancedb.io"))' "$identity")
go_version=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("go_version", "1.21.0"))' "$identity")
source_repo=$(field source_repo)
source_release=$(field source_release)
source_commit=$(field source_commit)
header_sha256=$(field header_sha256)

echo "release input: $source_repo $source_release $source_commit"
python3 "$repo_root/scripts/go-module-origin/fetch-release.py" \
  --identity "$identity" \
  --downloads "$downloads" \
  --out "$work/release" > "$work/targets.tsv"

while IFS=$'\t' read -r target library license; do
  [[ -n $target ]] || continue
  goos=${target%-*}
  goarch=${target##*-}
  echo "candidate: $target ($goos/$goarch)"
  (
    cd "$generator"
    "$go_cmd" run ./cmd/gencandidate \
      -library "$library" \
      -license "$license" \
      -target "$goos/$goarch" \
      -module-path "$module_prefix/$target" \
      -out "$candidates/$target/module" \
      -go-version "$go_version" \
      -upstream-repo "$source_repo" \
      -upstream-commit "$source_commit" \
      -header-sha256 "$header_sha256" >/dev/null
  )
done < "$work/targets.tsv"

echo "origin: $origin_dir"
(
  cd "$generator"
  "$go_cmd" run ./cmd/genorigin \
    -candidates "$candidates" \
    -out "$origin_dir" \
    -version "$version" \
    -time "$release_time" \
    -module-prefix "$module_prefix" \
    -base-url "$base_url" \
    -go-version "$go_version" \
    -source-repo "$source_repo" \
    -source-commit "$source_commit" \
    -source-release "$source_release" > "$work/origin-result.json"
)

record_args=(--identity "$identity" --candidates "$candidates" --origin "$origin_dir" --out "$identity_out")
if [[ -n ${RUN_ID:-} ]]; then
  record_args+=(--run-id "$RUN_ID")
fi
python3 "$repo_root/scripts/go-module-origin/record-candidate-identity.py" "${record_args[@]}"

echo "origin manifest: $origin_dir/manifest.json"
cat "$work/origin-result.json"
echo "recorded identity: $identity_out"
