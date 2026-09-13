param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath,
    [int]$ExpectedMajor = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ($ExpectedMajor -and $PSVersionTable.PSVersion.Major -ne $ExpectedMajor) { throw 'Wrong PowerShell version for required gate' }
if ($ExpectedMajor -eq 5 -and $PSVersionTable.PSVersion.Minor -ne 1) { throw 'Required Windows PowerShell 5.1 gate' }
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility, Microsoft.PowerShell.Management
. (Join-Path $PSScriptRoot 'powershell-oracle-helpers.ps1')
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
$probeName = 'wworacleprobe'
$lab = Join-Path ([IO.Path]::GetTempPath()) ('whichwhy-powershell-oracle-' + [guid]::NewGuid().ToString('N'))
$oldPath = $env:PATH
$oldModulePath = $env:PSModulePath
$marker = Join-Path $lab 'body.marker'
$moduleName = 'WhichWhyOracleFixture'
$moduleDir = Join-Path $lab $moduleName
$initMarker = Join-Path $moduleDir 'init.marker'

function Assert-WhichWhyMatchesPowerShell([string]$phase, [string]$name, [string[]]$kinds, [bool]$loaded = $false) {
    $before = Get-OracleSessionSnapshot
    $preference = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    $value = $null
    if ($null -ne $preference) { $value = $preference.Value }
    $texts = @()
    foreach ($json in @($true, $false)) {
        if ($json) { $text = @(whichwhy $name --json) } else { $text = @(whichwhy $name) }
        $code = $LASTEXITCODE
        if ($code -ne [int]($kinds.Count -eq 0)) { throw "[$phase] unexpected inspection exit $code" }
        if ((Test-OraclePathExists $marker)) { throw "[$phase] inspection executed fixture" }
        if ((Test-OraclePathExists $initMarker) -ne $loaded) { throw "[$phase] module initialization changed" }
        Assert-OracleText (Get-OracleSessionSnapshot) $before "[$phase] session state"
        $afterPreference = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
        if (-not [object]::ReferenceEquals($preference, $afterPreference)) { throw 'Preference identity/absence changed' }
        if ($null -ne $preference -and -not [object]::ReferenceEquals($value, $preference.Value)) { throw 'Preference value changed' }
        $texts += ($text -join "`n")
    }
    # No subject discovery before WhichWhy; guard even qualified unloaded names.
    $oracle = @(Get-PassiveOracleMatches $name)
    Assert-OracleText (Get-OracleSessionSnapshot) $before "[$phase] oracle session state"
    if ((Test-OraclePathExists $marker) -or ((Test-OraclePathExists $initMarker) -ne $loaded)) { throw 'Oracle mutated fixture state' }
    $doc = $texts[0] | ConvertFrom-Json
    if ($doc.schema_version -ne 2 -or $doc.resolution_scope -ne 'powershell-loaded-session' -or $doc.policy -ne 'powershell-loaded-session-order-v1' -or $doc.claim_strength -ne 'shell-observed') { throw 'Shell claim changed' }
    Assert-OracleText $doc.command $name 'Command identity'
    Assert-OracleText $doc.shell.version ([string]$PSVersionTable.PSVersion) 'Version'
    Assert-OracleText $doc.shell.edition ([string]$PSVersionTable.PSEdition) 'Edition'
    if ($doc.candidates.Count -ne $oracle.Count -or $oracle.Count -ne $kinds.Count) { throw "[$phase] candidate/fixture count mismatch" }
    for ($i = 0; $i -lt $oracle.Count; $i++) {
        Assert-OracleText ([string]$oracle[$i].CommandType) $kinds[$i] "[$phase] fixture order $i"
        Assert-OracleCandidate $doc.candidates[$i] $oracle[$i] "[$phase] candidate $i"
    }
    if ($oracle.Count) { Assert-OracleCandidate $doc.selected $oracle[0] "[$phase] selected" }
    elseif ($null -ne $doc.selected -or -not $doc.no_candidate_reason -or -not $texts[1].Contains($doc.no_candidate_reason)) { throw 'Missing result overclaims' }
    if ($doc.limitations -notcontains 'Unloaded module auto-loading is not modeled yet.') { throw 'Missing autoload limitation' }
    [Console]::WriteLine("PASS [$phase] candidates=$($oracle.Count); PowerShell $($PSVersionTable.PSVersion)")
}

try {
    [void][IO.Directory]::CreateDirectory($moduleDir)
    $secondDir = Join-Path $lab 'second path'
    [void][IO.Directory]::CreateDirectory($secondDir)
    foreach ($dir in @($lab, $secondDir)) {
        [IO.File]::WriteAllText((Join-Path $dir ($probeName + '.cmd')), "@echo off`r`necho executed>`"$marker`"`r`n")
    }
    [IO.File]::WriteAllText((Join-Path $moduleDir ($moduleName + '.psd1')), "@{ RootModule = '$moduleName.psm1'; ModuleVersion = '1.0'; FunctionsToExport = @('Get-WhichWhyOracleFixture') }")
    $moduleBody = "[IO.File]::AppendAllText((Join-Path `$PSScriptRoot 'init.marker'), 'init')`nfunction Get-WhichWhyOracleFixture { [IO.File]::WriteAllText((Join-Path (Split-Path `$PSScriptRoot) 'body.marker'), 'executed') }`nExport-ModuleMember -Function Get-WhichWhyOracleFixture"
    [IO.File]::WriteAllText((Join-Path $moduleDir ($moduleName + '.psm1')), $moduleBody)
    $env:PATH = "$lab;$secondDir"
    $env:PSModulePath = "$lab;$oldModulePath"
    $initScript = (& $exe init powershell) -join "`n"
    if ($LASTEXITCODE -ne 0 -or -not $initScript) { throw 'Bridge generation failed' }
    Invoke-Expression $initScript
    function global:wworacleprobe { [IO.File]::WriteAllText($marker, 'executed') }
    function global:Write-WhichWhyOracleMarker { [IO.File]::WriteAllText($marker, 'executed') }
    Set-Alias -Name $probeName -Value Write-WhichWhyOracleMarker -Scope Global
    Assert-WhichWhyMatchesPowerShell 'alias-function-application' $probeName @('Alias', 'Function', 'Application', 'Application')
    Remove-Item -LiteralPath ("Alias:" + $probeName) -Force
    Assert-WhichWhyMatchesPowerShell 'function-application' $probeName @('Function', 'Application', 'Application')
    Remove-Item -LiteralPath ("Function:" + $probeName) -Force
    Assert-WhichWhyMatchesPowerShell 'application-only' $probeName @('Application', 'Application')
    Assert-WhichWhyMatchesPowerShell 'missing' 'wworaclemissing' @()
    Assert-WhichWhyMatchesPowerShell 'unloaded-module' 'Get-WhichWhyOracleFixture' @()
    Assert-WhichWhyMatchesPowerShell 'unloaded-qualified' "$moduleName\Get-WhichWhyOracleFixture" @()
    Microsoft.PowerShell.Core\Import-Module (Join-Path $moduleDir ($moduleName + '.psd1')) -Global
    Assert-WhichWhyMatchesPowerShell 'loaded-module' 'Get-WhichWhyOracleFixture' @('Function') $true
    if ([IO.File]::ReadAllText($initMarker) -cne 'init') { throw 'Module initialized more than once' }
} finally {
    $env:PATH = $oldPath
    $env:PSModulePath = $oldModulePath
    Microsoft.PowerShell.Core\Remove-Module $moduleName -Force -ErrorAction SilentlyContinue
    foreach ($path in @("Alias:$probeName", "Function:$probeName", 'Function:whichwhy', 'Function:Write-WhichWhyOracleMarker')) {
        Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
    }
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
    if ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($lab)) -ne $tempRoot) { throw 'Unsafe fixture cleanup path' }
    if ([IO.Directory]::Exists($lab)) { [IO.Directory]::Delete($lab, $true) }
}

# Isolate suites from the parent harness; each native launch must succeed.
$shellExe = [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
foreach ($suite in @('passive', 'transport')) {
    & $shellExe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "test-powershell-$suite.ps1") -ExecutablePath $exe
    if ($LASTEXITCODE -ne 0) { throw "$suite suite failed" }
}
