# Runs the API locally on Windows with the settings from .env (which git ignores).
#
#   .\scripts\dev.ps1 migrate      create or upgrade the database tables
#   .\scripts\dev.ps1 serve        start the API on TD_ADDR (default 127.0.0.1:8090)
#   .\scripts\dev.ps1 job node-regrow
#   .\scripts\dev.ps1 seed all      pre-seed the test areas' nodes (after scripts\import-osm.ps1)
#
# The variables are set for this run only; nothing is written to your Windows environment.
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Command)

$root = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $root ".env"
if (-not (Test-Path $envFile)) {
    Write-Error "No .env found. Copy .env.example to .env and fill it in."
    exit 1
}

foreach ($line in Get-Content $envFile) {
    $line = $line.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { continue }
    $eq = $line.IndexOf("=")
    if ($eq -lt 1) { continue }
    [Environment]::SetEnvironmentVariable($line.Substring(0, $eq).Trim(), $line.Substring($eq + 1).Trim(), "Process")
}

if (-not $Command -or $Command.Count -eq 0) { $Command = @("serve") }

Push-Location $root
try {
    go run ./cmd/tdapi @Command
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
