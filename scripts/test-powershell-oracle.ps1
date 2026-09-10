param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
$probeName = 'wworacleprobe'
$lab = Join-Path ([System.IO.Path]::GetTempPath()) ('whichwhy-powershell-oracle-' + [guid]::NewGuid().ToString('N'))
$oldPath = $env:PATH

function Convert-CommandTypeToKind([string]$commandType) {
    if ($commandType -eq 'ExternalScript') {
        return 'external-script'
    }
    return $commandType.ToLowerInvariant()
}

function Assert-WhichWhyMatchesPowerShell([string]$phase) {
    $oracle = @(Microsoft.PowerShell.Core\Get-Command -Name $probeName -All -ListImported -ErrorAction SilentlyContinue)
    if ($oracle.Count -eq 0) {
        throw "[$phase] PowerShell oracle returned no matches"
    }

    $jsonText = (whichwhy $probeName --json | Out-String)
    if ($LASTEXITCODE -ne 0) {
        throw "[$phase] WhichWhy exited with $LASTEXITCODE"
    }
    $doc = $jsonText | ConvertFrom-Json

    if ($null -eq $doc.winner) {
        throw "[$phase] WhichWhy returned no winner"
    }
    if ([int]$doc.candidates.Count -ne [int]$oracle.Count) {
        throw "[$phase] candidate count mismatch: WhichWhy=$($doc.candidates.Count) PowerShell=$($oracle.Count)"
    }

    for ($i = 0; $i -lt $oracle.Count; $i++) {
        $expected = $oracle[$i]
        $actual = $doc.candidates[$i]
        $expectedKind = Convert-CommandTypeToKind ([string]$expected.CommandType)

        if ([string]$actual.kind -ne $expectedKind) {
            throw "[$phase] candidate $i kind mismatch: WhichWhy=$($actual.kind) PowerShell=$expectedKind"
        }
        if ([string]$actual.name -ne [string]$expected.Name) {
            throw "[$phase] candidate $i name mismatch: WhichWhy=$($actual.name) PowerShell=$($expected.Name)"
        }

        $expectedPath = ''
        if ($null -ne $expected.PSObject.Properties['Path']) {
            $expectedPath = [string]$expected.Path
        }
        if ($expectedPath -ne '' -and -not [string]::Equals([string]$actual.path, $expectedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "[$phase] candidate $i path mismatch: WhichWhy=$($actual.path) PowerShell=$expectedPath"
        }

        if ($expectedKind -eq 'alias') {
            if ([string]$actual.alias_target -ne [string]$expected.Definition) {
                throw "[$phase] alias target mismatch: WhichWhy=$($actual.alias_target) PowerShell=$($expected.Definition)"
            }
        }
    }

    $expectedWinnerKind = Convert-CommandTypeToKind ([string]$oracle[0].CommandType)
    if ([string]$doc.winner.kind -ne $expectedWinnerKind) {
        throw "[$phase] winner mismatch: WhichWhy=$($doc.winner.kind) PowerShell=$expectedWinnerKind"
    }

    Write-Host "PASS [$phase] winner=$expectedWinnerKind candidates=$($oracle.Count)"
}

New-Item -ItemType Directory -Path $lab -Force | Out-Null
Set-Content -LiteralPath (Join-Path $lab ($probeName + '.cmd')) -Value '@echo off' -Encoding ASCII

try {
    $env:PATH = "$lab;$oldPath"

    $initScript = (& $exe init powershell | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "whichwhy init powershell exited with $LASTEXITCODE"
    }
    Invoke-Expression $initScript

    function global:wworacleprobe { 'function' }
    Set-Alias -Name $probeName -Value Get-Date -Scope Global

    Assert-WhichWhyMatchesPowerShell 'alias-function-application'

    Remove-Item -LiteralPath ("Alias:" + $probeName) -Force
    Assert-WhichWhyMatchesPowerShell 'function-application'

    Remove-Item -LiteralPath ("Function:" + $probeName) -Force
    Assert-WhichWhyMatchesPowerShell 'application-only'
}
finally {
    $env:PATH = $oldPath
    Remove-Item -LiteralPath ("Alias:" + $probeName) -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath ("Function:" + $probeName) -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath 'Function:whichwhy' -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $lab -Recurse -Force -ErrorAction SilentlyContinue
}
