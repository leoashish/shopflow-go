# Copies the reference solution for one step over your stub files.
# Usage:  .\scripts\apply-solution.ps1 03
# Tip: commit your own work to git first, so you can compare with `git diff` or undo with `git checkout .`
param([Parameter(Mandatory = $true)][string]$Step)
$Step = '{0:D2}' -f [int]$Step
$root = Resolve-Path (Join-Path $PSScriptRoot '..')
$src = Join-Path $root "_solutions\step$Step"
if (-not (Test-Path $src)) { Write-Error "No solution folder: $src"; exit 1 }
Copy-Item -Path (Join-Path $src '*') -Destination $root -Recurse -Force -Verbose
Write-Host "Applied the reference solution for Step $Step."
