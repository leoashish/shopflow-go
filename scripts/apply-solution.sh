#!/usr/bin/env bash
# Copies the reference solution for one step over your stub files.
# Usage:  bash scripts/apply-solution.sh 03
# Tip: commit your own work to git first, so you can compare with `git diff` or undo with `git checkout .`
set -euo pipefail
step="${1:?usage: apply-solution.sh <step number, e.g. 03>}"
step=$(printf '%02d' "$((10#$step))")
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$root/_solutions/step$step"
[ -d "$src" ] || { echo "No solution folder: $src" >&2; exit 1; }
cp -rv "$src"/. "$root"/
echo "Applied the reference solution for Step $step."
