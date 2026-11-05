# Builds bin/tdapi for the API server (Linux arm64) from Windows, with the git version stamped in.
#
#   .\scripts\build-linux.ps1
#   scp bin/tdapi ubuntu@<server>:/home/ubuntu/terrandominion-api-dev/
#   (on the server) sudo systemctl restart terrandominion-api-dev
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $version = (git describe --tags --always --dirty 2>$null)
    if (-not $version) { $version = "dev" }
    $env:GOOS = "linux"; $env:GOARCH = "arm64"; $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o bin/tdapi ./cmd/tdapi
    $code = $LASTEXITCODE
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
if ($code -eq 0) { Write-Host "Built bin/tdapi (linux/arm64, $version)" }
exit $code
