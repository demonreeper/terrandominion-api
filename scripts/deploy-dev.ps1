# Builds the API for the server (linux/arm64) and deploys it to the dev service in one step.
#
#   .\scripts\deploy-dev.ps1
#   .\scripts\deploy-dev.ps1 -Server ubuntu@other-host
#
# The running binary is held open by the service, so it cannot be overwritten in place: the new
# build is copied as tdapi.new and renamed over the old one (a rename works while it runs), then
# the service restarts. Migrations run as it starts. Needs passwordless sudo for systemctl.
param(
    [string]$Server = "ubuntu@sshmvepoc.monocity.xyz",
    [string]$Folder = "/home/ubuntu/terrandominion-api-dev",
    [string]$Service = "terrandominion-api-dev"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

& (Join-Path $PSScriptRoot "build-linux.ps1")
if ($LASTEXITCODE -ne 0) { Write-Error "Build failed."; exit 1 }

Push-Location $root
try {
    scp bin/tdapi "${Server}:${Folder}/tdapi.new"
    if ($LASTEXITCODE -ne 0) { Write-Error "Copy failed."; exit 1 }

    $remote = "cd $Folder && chmod 700 tdapi.new && mv tdapi.new tdapi && " +
              "sudo systemctl restart $Service && sleep 2 && systemctl is-active $Service && " +
              "./tdapi version && journalctl -u $Service -n 3 --no-pager"
    ssh $Server $remote
    if ($LASTEXITCODE -ne 0) { Write-Error "Restart failed. Check: journalctl -u $Service -n 30"; exit 1 }
    Write-Host "Deployed to $Server ($Service)."
}
finally {
    Pop-Location
}
