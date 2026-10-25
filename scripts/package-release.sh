#!/usr/bin/env bash
# Build the two release archives and their SHA-256 manifest; never publish.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo 'Usage: bash scripts/package-release.sh TAG OUTPUT_DIRECTORY' >&2
  exit 1
fi
version=$(python3 scripts/calver.py validate "$1")
output=$2
if [[ -e "$output" ]]; then
  echo 'Output directory must not already exist.' >&2
  exit 1
fi
mkdir -p "$output"
output=$(cd "$output" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export CGO_ENABLED=0

for target in darwin_arm64 linux_amd64; do
  export GOOS=${target%_*}
  export GOARCH=${target#*_}
  directory="repo-policy_${version}_${target}"
  mkdir -p "$work/$directory"
  go build -trimpath -ldflags "-s -w -X main.version=$version" \
    -o "$work/$directory/repo-policy" ./cmd/repo-policy
  cp LICENSE README.md "$work/$directory/"
  COPYFILE_DISABLE=1 tar -czf "$output/$directory.tar.gz" -C "$work" "$directory"
done

python3 - "$output" <<'PY'
import hashlib
import pathlib
import sys

output = pathlib.Path(sys.argv[1])
with (output / 'checksums.txt').open('w') as manifest:
    for archive in sorted(output.glob('*.tar.gz')):
        manifest.write(f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n')
PY
