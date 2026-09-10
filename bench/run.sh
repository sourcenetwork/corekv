#!/usr/bin/env bash
# Runs the corekv bench suite reproducibly. Output is tee'd to bench/results-<stamp>.txt.
#
#   ./run.sh                  # badger + memory lanes
#   ./run.sh -tags regolith   # also the regolith lane (needs regolith/ and `make ffi`)
#
# Any extra arguments are passed straight through to `go test`.
set -euo pipefail

cd "$(dirname "$0")"

out="results-$(date +%Y%m%d-%H%M%S).txt"

{
  echo "# $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "# go:   $(go version)"
  echo "# args: $*"
  uname -srm
} | tee "$out"

go test -run '^$' -bench . -benchtime 3s -count 5 -timeout 0 "$@" 2>&1 | tee -a "$out"

echo "results: $(pwd)/$out"
