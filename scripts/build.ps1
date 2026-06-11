param(
    [switch]$RunTests,
    [switch]$DebugEditor
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$Bin = Join-Path $Root "bin"

if (-not (Test-Path -LiteralPath $Bin)) {
    New-Item -ItemType Directory -Path $Bin | Out-Null
}

Push-Location $Root
try {
    if ($RunTests) {
        go test ./...
    }

    go build -o (Join-Path $Bin "aiengine.exe") .

    if ($DebugEditor) {
        go build -o (Join-Path $Bin "AIEngine-Editor.exe") ./cmd/editor
    } else {
        go build -ldflags "-H=windowsgui" -o (Join-Path $Bin "AIEngine-Editor.exe") ./cmd/editor
    }

    Get-Item -LiteralPath (Join-Path $Bin "aiengine.exe"), (Join-Path $Bin "AIEngine-Editor.exe") |
        Select-Object Name, Length, LastWriteTime
} finally {
    Pop-Location
}
