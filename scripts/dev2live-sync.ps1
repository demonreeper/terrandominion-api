# Dev2LiveSync (pre-launch only): resets live's game data and carries the test players over from dev, so
# their accounts, sign-ins and callsigns are reserved for launch, with the tester badge. See cmd/dev2live.
#
#   .\scripts\dev2live-sync.ps1           dry run: prints what would change, live untouched
#   .\scripts\dev2live-sync.ps1 -Apply    backs live up on the server, then does it
#
# Runs on the API server, with each service's own .env (dev: terrandominion-api-dev, live: terrandominion-api),
# so it works when the database only takes connections from there. Backups: /home/ubuntu/td-backups/.
# Restore one: pg_restore --clean --if-exists --no-owner -d "<live url>" <file> (on the server).
param(
    [switch]$Apply,
    [string]$Skip = "44",           # dev ids not to carry (44: the owner's second test account)
    [string]$NoBadge = "1",         # carried ids without the badge (1: the owner)
    [string]$Badge = "first signal",
    [switch]$KeepTestNodes,
    [switch]$AllowLiveOnly,
    [string]$Server = "ubuntu@sshmvepoc.monocity.xyz"
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

# 1. Build the job for the server (linux/arm64).
Push-Location $root
try {
    $env:GOOS = "linux"; $env:GOARCH = "arm64"; $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-s -w" -o bin/dev2live ./cmd/dev2live
    if ($LASTEXITCODE -ne 0) { throw "build failed" }
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

# 2. Copy it to the server.
ssh $Server "mkdir -p /home/ubuntu/td-tools /home/ubuntu/td-backups"
scp -q (Join-Path $root "bin/dev2live") "${Server}:/home/ubuntu/td-tools/dev2live"
if ($LASTEXITCODE -ne 0) { throw "copy failed" }

$urls = 'DEV=$(grep "^TD_DATABASE_URL=" /home/ubuntu/terrandominion-api-dev/.env | cut -d= -f2-); ' +
        'LIVE=$(grep "^TD_DATABASE_URL=" /home/ubuntu/terrandominion-api/.env | cut -d= -f2-); '

# 3. Back live up first (only when applying).
if ($Apply) {
    $stamp = (Get-Date).ToUniversalTime().ToString("yyyyMMdd-HHmmss")
    $backup = "/home/ubuntu/td-backups/live-$stamp.dump"
    ssh $Server ($urls + "pg_dump -Fc -f $backup `"`$LIVE`" && ls -lh $backup")
    if ($LASTEXITCODE -ne 0) { throw "backup failed: nothing changed" }
    Write-Host "Live backed up to $backup"
}

# 4. Run it.
$flags = "-skip '$Skip' -no-badge '$NoBadge' -badge '$Badge'"
if ($KeepTestNodes) { $flags += " -keep-test-nodes" }
if ($AllowLiveOnly) { $flags += " -allow-live-only" }
if ($Apply) { $flags += " -apply" }
ssh $Server ($urls + "chmod +x /home/ubuntu/td-tools/dev2live && TD_DEV_DATABASE_URL=`"`$DEV`" TD_LIVE_DATABASE_URL=`"`$LIVE`" /home/ubuntu/td-tools/dev2live $flags")
exit $LASTEXITCODE
