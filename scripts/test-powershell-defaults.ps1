param(
    [Parameter(Mandatory = $true)][string]$ExecutablePath,
    [string]$DefaultsCase
)

# Each case starts in a fresh process, including immutable/AllScope variables.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility, Microsoft.PowerShell.Management
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
. (Join-Path $PSScriptRoot 'powershell-oracle-helpers.ps1')
if (-not $DefaultsCase) {
    $shellExe = [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
    foreach ($case in @('Absent', 'Empty', 'Null', 'ArgumentList', 'Wildcard', 'CommandType', 'ScriptBlock', 'Unrelated', 'ReadOnly', 'Constant', 'AllScope', 'AllScopeReadOnly', 'AllScopeConstant', 'Inherited', 'Nested', 'Shadowed')) {
        & $shellExe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $PSCommandPath -ExecutablePath $exe -DefaultsCase $case
        if ($LASTEXITCODE -ne 0) { throw "Parameter-default isolation failed for $case" }
    }
    return
}

Invoke-Expression ((& $exe init powershell) -join "`n")
$global:AuditDynamicMarker = 0
$global:AuditBodyMarker = 0
$global:AuditDefaultMarker = 0
function global:wwauditdynamic {
    [CmdletBinding()]
    param([string]$Value)
    dynamicparam {
        $global:AuditDynamicMarker++
        [System.Management.Automation.RuntimeDefinedParameterDictionary]::new()
    }
    end { $global:AuditBodyMarker++ }
}
# Independent fixture identity; do not obtain expected metadata with Get-Command.
Set-Item -LiteralPath 'Function:global:wwaudit[defaults]' -Value ${function:wwauditdynamic}
$ExecutionContext.SessionState.PSVariable.Remove('global:PSDefaultParameterValues')
$ExecutionContext.SessionState.PSVariable.Remove('script:PSDefaultParameterValues')

function Assert-DefaultsState {
    if ($global:AuditDynamicMarker -ne 0 -or $global:AuditBodyMarker -ne 0 -or $global:AuditDefaultMarker -ne 0) {
        throw "Inspected/default code executed: dynamic=$global:AuditDynamicMarker body=$global:AuditBodyMarker default=$global:AuditDefaultMarker"
    }
    $after = $ExecutionContext.SessionState.PSVariable.Get('PSDefaultParameterValues')
    if (-not [object]::ReferenceEquals($original, $after)) { throw 'Defaults variable identity/absence changed' }
    if ($null -ne $after) {
        if (-not [object]::ReferenceEquals($originalValue, $after.Value) -or $originalOptions -ne $after.Options) { throw 'Defaults value/options changed' }
        if ([System.Management.Automation.PSSerializer]::Serialize($after.Value) -cne $originalContent) { throw 'Defaults content changed' }
        if ($null -ne $originalValue) {
            if ($originalValue.Count -ne $originalEntries.Count) { throw 'Defaults entry count changed' }
            foreach ($key in $originalEntries.Keys) {
                if (-not $originalValue.ContainsKey($key) -or -not [object]::ReferenceEquals($originalEntries[$key], $originalValue[$key])) { throw 'Defaults entry changed' }
            }
        }
    }
    $afterGlobal = $ExecutionContext.SessionState.PSVariable.Get('global:PSDefaultParameterValues')
    if (-not [object]::ReferenceEquals($globalOriginal, $afterGlobal)) { throw 'Global defaults identity changed' }
    if ($null -ne $afterGlobal -and (-not [object]::ReferenceEquals($globalValue, $afterGlobal.Value) -or $globalOptions -ne $afterGlobal.Options)) { throw 'Global defaults value/options changed' }
    if ($null -ne $afterGlobal -and [System.Management.Automation.PSSerializer]::Serialize($afterGlobal.Value) -cne $globalContent) { throw 'Global defaults content changed' }
    $afterAutoload = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    if (-not [object]::ReferenceEquals($autoloadOriginal, $afterAutoload)) { throw 'Autoload preference identity/absence changed' }
    if ($null -ne $afterAutoload -and (-not [object]::ReferenceEquals($autoloadValue, $afterAutoload.Value) -or $autoloadOptions -ne $afterAutoload.Options)) { throw 'Autoload preference value/options changed' }
}

function Test-DefaultsCase {
    $defaults = @{'Get-Command:ArgumentList' = @('probe')}
    $options = 'None'
    switch ($DefaultsCase) {
        'Empty' { $defaults = @{} }
        'Null' { $defaults = $null }
        'Wildcard' { $defaults = @{'*-Command:ArgumentList' = @('probe')} }
        'CommandType' { $defaults = @{'Get-Command:CommandType' = 'Cmdlet'} }
        'ScriptBlock' { $defaults = @{'Get-Command:ArgumentList' = { $global:AuditDefaultMarker++; @('probe') }} }
        'Unrelated' { $defaults = @{'Get-Date:Format' = 'yyyy'} }
        'ReadOnly' { $options = 'ReadOnly' }
        'Constant' { $options = 'Constant' }
        'AllScope' { $options = 'AllScope' }
        'AllScopeReadOnly' { $options = 'AllScope,ReadOnly' }
        'AllScopeConstant' { $options = 'AllScope,Constant' }
    }
    if ($DefaultsCase -in @('Inherited', 'Shadowed')) {
        New-Variable PSDefaultParameterValues -Scope Global -Value $defaults
    }
    if ($DefaultsCase -eq 'Shadowed') { $defaults = @{'*-Command:ArgumentList' = @('shadowed-probe')} }
    if ($DefaultsCase -notin @('Absent', 'Inherited')) {
        New-Variable PSDefaultParameterValues -Scope Local -Value $defaults -Option $options
    }
    $original = $ExecutionContext.SessionState.PSVariable.Get('PSDefaultParameterValues')
    $originalValue = $null; $originalOptions = $null; $originalEntries = @{}; $originalContent = ''
    if ($null -ne $original) {
        $originalValue = $original.Value; $originalOptions = $original.Options
        $originalContent = [System.Management.Automation.PSSerializer]::Serialize($originalValue)
        if ($null -ne $originalValue) { foreach ($key in $originalValue.Keys) { $originalEntries[$key] = $originalValue[$key] } }
    }
    $globalOriginal = $ExecutionContext.SessionState.PSVariable.Get('global:PSDefaultParameterValues')
    $globalValue = $null; $globalOptions = $null; $globalContent = ''
    if ($null -ne $globalOriginal) {
        $globalValue = $globalOriginal.Value; $globalOptions = $globalOriginal.Options
        $globalContent = [System.Management.Automation.PSSerializer]::Serialize($globalValue)
    }
    $autoloadOriginal = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    $autoloadValue = $null; $autoloadOptions = $null
    if ($null -ne $autoloadOriginal) { $autoloadValue = $autoloadOriginal.Value; $autoloadOptions = $autoloadOriginal.Options }
    $mustRefuse = $DefaultsCase.StartsWith('AllScope', [StringComparison]::Ordinal)
    # Refusal must terminate safely even with the normal nonterminating-error preference.
    if ($mustRefuse) { $ErrorActionPreference = 'Continue' }
    foreach ($literal in @($false, $true)) {
        $names = @('wwauditdynamic', 'wwaudit-defaults-missing')
        if ($literal) { $names += 'wwaudit[defaults]' }
        foreach ($name in $names) {
            foreach ($json in @($false, $true)) {
                $arguments = @($name)
                if ($literal) { $arguments = @('inspect') + $arguments }
                if ($json) { $arguments += '--json' }
                $output = @(); $refused = $false
                try {
                    if ($DefaultsCase -eq 'Nested') { $output = @(& { & { whichwhy @arguments } }) }
                    else { $output = @(whichwhy @arguments) }
                } catch {
                    if (-not $mustRefuse -or [string]$_ -notlike '*whichwhy: cannot isolate PowerShell scratch variable PSDefaultParameterValues*') { throw }
                    $refused = $true
                }
                $code = $LASTEXITCODE
                # Check markers BEFORE any oracle call or report parsing.
                Assert-DefaultsState
                if ($mustRefuse) {
                    if (-not $refused -or $code -ne 2 -or $output.Count -ne 0) { throw 'AllScope defaults were not safely refused' }
                } else {
                    $missing = $name -eq 'wwaudit-defaults-missing'
                    if ($refused -or $code -ne [int]$missing -or $output.Count -eq 0) { throw 'Defaults changed result/exit behavior' }
                    if ($json) {
                        $doc = ($output -join "`n") | ConvertFrom-Json
                        Assert-OracleJSONContract $doc
                        if ($doc.command -cne $name) { throw 'Requested identity changed' }
                        if ($missing) {
                            if ($null -ne $doc.selected -or $doc.candidates.Count -ne 0) { throw 'Missing fixture fabricated a candidate' }
                        } elseif ($doc.candidates.Count -ne 1 -or $doc.selected.kind -cne 'function' -or $doc.selected.name -cne $name) { throw 'Known function fixture lost or reordered' }
                    } elseif ($output[0] -cne ('WhichWhy - ' + $name) -or (-not $missing -and ($output -join "`n") -notlike '*SELECTED*')) { throw 'Human result changed' }
                }
            }
        }
    }
    if ($mustRefuse) {
        # The guard belongs to inspection, not unrelated public forwarding.
        $version = @(whichwhy --version)
        if ($LASTEXITCODE -ne 0 -or $version.Count -ne 1 -or $version[0] -notlike 'whichwhy *') { throw 'AllScope defaults blocked version forwarding' }
        Assert-DefaultsState
    } else {
        # A failure after isolation/autoload guard installation must not leak state.
        $failed = $false
        try { whichwhy '' --json | Out-Null } catch { $failed = $true }
        if (-not $failed) { throw 'Expected discovery validation failure' }
        Assert-DefaultsState
    }
    # Independently assert the test oracle is passive too, not merely in agreement.
    $oracle = @(); $refused = $false
    try { $oracle = @(Get-PassiveOracleMatches 'wwauditdynamic') } catch {
        if (-not $mustRefuse -or [string]$_ -notlike '*Oracle parameter defaults cannot be isolated*') { throw }
        $refused = $true
    }
    Assert-DefaultsState
    if ($mustRefuse) {
        if (-not $refused -or $oracle.Count -ne 0) { throw 'Oracle did not refuse AllScope defaults' }
    } elseif ($oracle.Count -ne 1 -or $oracle[0].Name -cne 'wwauditdynamic' -or [string]$oracle[0].CommandType -cne 'Function') { throw 'Oracle lost the fixed function fixture' }
    [Console]::WriteLine("PASS [defaults/$DefaultsCase] both grammars/formats; dynamic/body/default markers zero; caller state preserved; PowerShell $($PSVersionTable.PSVersion)")
}
Test-DefaultsCase
