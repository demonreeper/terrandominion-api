# Loads the node seeder's map layers (land, walkable ways, blocked areas) into the database named
# in .env, from the same OSM extract the map tiles are built from. Re-run it whenever the map is
# rebuilt; it replaces the old layers and makes every cell be looked at again.
#
#   .\scripts\import-osm.ps1                      uses the extract in References\maps or deploy\maps
#   .\scripts\import-osm.ps1 -Pbf D:\path\maldives.osm.pbf
#
# Needs Python 3 (no extra packages). Then pre-seed the test area with:
#   .\scripts\dev.ps1 seed all
param([string]$Pbf = "")

$root = Split-Path -Parent $PSScriptRoot
if ($Pbf -eq "") {
    $candidates = @(
        (Join-Path $root "deploy\maps\data\sources\maldives.osm.pbf"),
        (Join-Path $root "..\References\maps\data\sources\maldives.osm.pbf")
    )
    $Pbf = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $Pbf) {
        Write-Error "No maldives.osm.pbf found. Looked in: $($candidates -join ', '). Pass -Pbf."
        exit 1
    }
}

$python = Get-Command python3 -ErrorAction SilentlyContinue
if (-not $python) { $python = Get-Command python -ErrorAction SilentlyContinue }
if (-not $python) {
    Write-Error "Python 3 is needed for deploy\maps\osm_extract.py."
    exit 1
}

$out = Join-Path ([IO.Path]::GetTempPath()) "td-osm-extract.json"
Write-Host "Extracting from $Pbf"
& $python.Source (Join-Path $root "deploy\maps\osm_extract.py") $Pbf $out
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Importing into the database in .env"
& (Join-Path $PSScriptRoot "dev.ps1") import-osm $out
$code = $LASTEXITCODE
Remove-Item $out -ErrorAction SilentlyContinue
exit $code
