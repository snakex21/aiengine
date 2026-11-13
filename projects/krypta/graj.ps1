# graj.ps1 - buduje silnik i uruchamia "Krysztalowa Krypte".
# Uzycie (z katalogu glownego repo):
#   powershell -ExecutionPolicy Bypass -File projects\krypta\graj.ps1
# Dodatkowe argumenty trafiaja do silnika, np.:  ... graj.ps1 --exec "fps"
# go build korzysta z cache, wiec po pierwszym razie trwa chwile.
$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$exe  = Join-Path $root 'bin\aiengine.exe'
Push-Location $root
try {
    go build -o $exe .
    if ($LASTEXITCODE -ne 0) { throw 'go build nie powiodl sie' }
    & $exe --project krypta @args
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
