# Builds the API for the server (linux/arm64) and deploys it to the live service (td.demonreeper.xyz,
# database terran_dominion). Deploy to dev first: live only gets a build whose migrations dev has run.
#
#   .\scripts\deploy-live.ps1
#
# Same steps as deploy-dev.ps1 (copy as tdapi.new, rename over the running binary, restart).
param(
    [string]$Server = "ubuntu@sshmvepoc.monocity.xyz"
)

& (Join-Path $PSScriptRoot "deploy-dev.ps1") -Server $Server -Folder "/home/ubuntu/terrandominion-api" -Service "terrandominion-api"
exit $LASTEXITCODE
