param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath,
    [string]$PreferenceCase
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path

if (-not $PreferenceCase) {
    # Every preference case gets a new process, including the truly absent case.
    $shellExe = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
    foreach ($case in @('Absent', 'All', 'ModuleQualified', 'None', 'Shadowed', 'Null', 'Other', 'Array', 'Enum', 'TypedEnum', 'AllScope', 'ReadOnlyNone', 'ConstantNone', 'ReadOnlyAll', 'ConstantAll', 'TypedArray')) {
        & $shellExe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $PSCommandPath -ExecutablePath $exe -PreferenceCase $case
        if ($LASTEXITCODE -ne 0) { throw "Passive inspection failed for $case" }
    }
    return
}

$lab = Join-Path ([IO.Path]::GetTempPath()) ('whichwhy-passive-' + [guid]::NewGuid().ToString('N'))
$moduleName = 'WhichWhyPassiveFixture'
$probe = 'Get-WhichWhyPassiveFixture'
$moduleDir = Join-Path $lab $moduleName
$initMarker = Join-Path $moduleDir 'init.marker'
$bodyMarker = Join-Path $moduleDir 'body.marker'
$oldModulePath = $env:PSModulePath

function Assert-ModuleState([bool]$loaded) {
    if ((@(Microsoft.PowerShell.Core\Get-Module -Name $moduleName).Count -gt 0) -ne $loaded) { throw 'Module loaded state changed' }
    if ([IO.File]::Exists($initMarker) -ne $loaded) { throw 'Unexpected module initialization' }
    if ([IO.File]::Exists($bodyMarker)) { throw 'Inspected command body executed' }
}

function Get-ModuleSnapshot {
    # No formatting/utility commands run between this snapshot and its comparison.
    $names = @(foreach ($module in @(Microsoft.PowerShell.Core\Get-Module)) { "$($module.Name):$($module.Path)" })
    [Array]::Sort($names)
    return $names -join "`n"
}

function Assert-PreferenceUnchanged {
    $after = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    if (-not [object]::ReferenceEquals($originalVariable, $after)) { throw 'Preference variable identity or absence changed' }
    if ($null -ne $after) {
        if (-not [object]::ReferenceEquals($originalValue, $after.Value)) { throw 'Preference value changed' }
        if ($originalOptions -ne $after.Options) { throw 'Preference options changed' }
    }
    $callerAfter = $ExecutionContext.SessionState.PSVariable.Get('PSModuleAutoLoadingPreference')
    if (-not [object]::ReferenceEquals($callerVariable, $callerAfter)) { throw 'Caller preference identity or absence changed' }
    if ($null -ne $callerAfter -and -not [object]::ReferenceEquals($callerValue, $callerAfter.Value)) { throw 'Caller preference value changed' }
}

function Get-GlobalVariableNames {
    $names = @(foreach ($variable in @(Microsoft.PowerShell.Utility\Get-Variable -Scope Global)) { $variable.Name })
    [Array]::Sort($names)
    return $names -join "`n"
}

function Assert-Inspection([string]$name, [bool]$loaded, [string]$kind = '') {
    Assert-ModuleState $loaded
    $modulesBefore = Get-ModuleSnapshot
    $variablesBefore = Get-GlobalVariableNames
    $text = @(whichwhy $name --json)
    $code = $LASTEXITCODE
    Assert-PreferenceUnchanged
    Assert-ModuleState $loaded
    if ($modulesBefore -cne (Get-ModuleSnapshot)) { throw 'Collector changed the loaded module set' }
    if ($variablesBefore -cne (Get-GlobalVariableNames)) { throw 'Collector leaked global variables' }
    $doc = ($text -join "`n") | ConvertFrom-Json
    if ($doc.resolution_scope -ne 'powershell-loaded-session') { throw 'Unexpected resolution scope' }
    if ($doc.policy -ne 'powershell-loaded-session-order-v1' -or $doc.claim_strength -ne 'shell-observed') { throw 'Unexpected shell claim' }
    if ($doc.limitations -notcontains 'Unloaded module auto-loading is not modeled yet.') { throw 'Missing autoload limitation' }
    if ($kind) {
        if ($code -ne 0 -or $doc.selected.kind -ne $kind) { throw "Wrong loaded winner for $name" }
        if ($name -eq $probe -and $doc.selected.source -ne $moduleName) { throw 'Wrong module source' }
    } elseif ($code -ne 1 -or $null -ne $doc.selected -or $doc.candidates.Count -ne 0) {
        throw "Unloaded command fabricated a winner for $name"
    }
    $human = @(whichwhy $name) -join "`n"
    if ($LASTEXITCODE -ne $code) { throw 'Human/JSON exit status divergence' }
    Assert-PreferenceUnchanged
    Assert-ModuleState $loaded
    if ($modulesBefore -cne (Get-ModuleSnapshot)) { throw 'Human collector changed the loaded module set' }
    if ($variablesBefore -cne (Get-GlobalVariableNames)) { throw 'Human collector leaked global variables' }
    if ($kind) {
        if ($human -notmatch 'POWERSHELL WINNER') { throw 'Human output lost winner' }
    } elseif ($human -notmatch 'No command match was found in the current loaded PowerShell session') {
        throw 'Human output lost loaded-session limitation'
    }
}

[void][IO.Directory]::CreateDirectory($moduleDir)
[IO.File]::WriteAllText((Join-Path $moduleDir ($moduleName + '.psd1')), "@{ RootModule = '$moduleName.psm1'; ModuleVersion = '1.0'; FunctionsToExport = @('$probe') }")
[IO.File]::WriteAllText((Join-Path $moduleDir ($moduleName + '.psm1')), @'
[IO.File]::AppendAllText((Join-Path $PSScriptRoot 'init.marker'), 'init')
function Get-WhichWhyPassiveFixture {
    [IO.File]::WriteAllText((Join-Path $PSScriptRoot 'body.marker'), 'body')
}
Export-ModuleMember -Function Get-WhichWhyPassiveFixture
'@)

try { & {
    $env:PSModulePath = "$lab;$oldModulePath"
    # Load the fixture's test helpers before measuring the collector's module set.
    Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility
    $init = (& $exe init powershell) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Bridge generation failed' }
    Invoke-Expression $init

    switch ($PreferenceCase) {
        'Absent' { $ExecutionContext.SessionState.PSVariable.Remove('global:PSModuleAutoLoadingPreference') }
        'Shadowed' { $global:PSModuleAutoLoadingPreference = 'All' }
        'Null' { $global:PSModuleAutoLoadingPreference = $null }
        'Other' { $global:PSModuleAutoLoadingPreference = [object]::new() }
        'Array' { $global:PSModuleAutoLoadingPreference = @('None') }
        'Enum' { $global:PSModuleAutoLoadingPreference = [System.Management.Automation.PSModuleAutoLoadingPreference]::ModuleQualified }
        'TypedEnum' { [System.Management.Automation.PSModuleAutoLoadingPreference]$global:PSModuleAutoLoadingPreference = 'All' }
        'TypedArray' { [string[]]$global:PSModuleAutoLoadingPreference = @('All') }
        'AllScope' { New-Variable PSModuleAutoLoadingPreference -Value All -Scope Global -Option AllScope }
        'ReadOnlyNone' { New-Variable PSModuleAutoLoadingPreference -Value None -Scope Global -Option ReadOnly }
        'ConstantNone' { New-Variable PSModuleAutoLoadingPreference -Value None -Scope Global -Option Constant }
        'ReadOnlyAll' { New-Variable PSModuleAutoLoadingPreference -Value All -Scope Global -Option ReadOnly }
        'ConstantAll' { New-Variable PSModuleAutoLoadingPreference -Value All -Scope Global -Option Constant }
        default { $global:PSModuleAutoLoadingPreference = $PreferenceCase }
    }
    $originalVariable = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    $originalValue = $null
    $originalOptions = $null
    if ($null -ne $originalVariable) { $originalValue = $originalVariable.Value; $originalOptions = $originalVariable.Options }
    # Test a conflicting caller-local preference separately so Absent really
    # has no preference variable anywhere in the caller's scope chain.
    if ($PreferenceCase -eq 'Shadowed') { $local:PSModuleAutoLoadingPreference = 'ModuleQualified' }
    $callerVariable = $ExecutionContext.SessionState.PSVariable.Get('PSModuleAutoLoadingPreference')
    $callerValue = $null
    if ($null -ne $callerVariable) { $callerValue = $callerVariable.Value }

    Assert-ModuleState $false
    if ($PreferenceCase -in @('ReadOnlyAll', 'ConstantAll', 'TypedArray')) {
        # Immutable or coercing preferences must fail closed, even under the
        # usual nonterminating-error preference. Never force caller options.
        $ErrorActionPreference = 'Continue'
        $failed = $false
        try { whichwhy $probe --json } catch { $failed = $true }
        if (-not $failed) { throw 'Constrained autoload preference did not fail closed' }
        Assert-PreferenceUnchanged
        Assert-ModuleState $false
        [Console]::WriteLine("PASS [passive/$PreferenceCase] safely refused inspection; PowerShell $($PSVersionTable.PSVersion)")
        return
    }
    $available = @(Microsoft.PowerShell.Core\Get-Module -Name $moduleName -ListAvailable)
    if ($available.Count -ne 1) { throw 'Disposable module is not discoverable' }
    Assert-ModuleState $false
    Assert-Inspection $probe $false
    Assert-Inspection "$moduleName\$probe" $false
    # Preserve the existing routing for wildcard names:
    # these are explicitly process-external, never claimed as shell evidence.
    foreach ($externalName in @('wwpassive[fixture]', 'wwpassive*fixture', 'wwpassive?fixture')) {
        $externalText = @(whichwhy $externalName --json)
        Assert-PreferenceUnchanged
        Assert-ModuleState $false
        $external = ($externalText -join "`n") | ConvertFrom-Json
        if ($external.resolution_scope -ne 'process-external') { throw 'Special-name routing changed' }
    }
    Set-Alias -Name wwpassivealias -Value $probe -Scope Global
    Assert-Inspection 'wwpassivealias' $false 'alias'
    Set-Alias -Name wwpassivealias -Value "$moduleName\$probe" -Scope Global
    Assert-Inspection 'wwpassivealias' $false 'alias'

    # Literal punctuation stays a single exact-name argument, never evaluated.
    foreach ($literal in @('wwpassive;literal', 'wwpassive`literal', 'wwpassive(literal)', 'wwpassive literal', "wwpassive'literal")) {
        Set-Item -LiteralPath ("Function:global:" + $literal) -Value { throw 'Literal command executed' }
        $oracle = @(Microsoft.PowerShell.Core\Get-Command -Name $literal -All -ListImported -ErrorAction SilentlyContinue)
        $expectedKind = ''
        if ($oracle.Count -gt 0) { $expectedKind = ([string]$oracle[0].CommandType).ToLowerInvariant() }
        Assert-Inspection $literal $false $expectedKind
    }
    function global:Get-Command { throw 'Unqualified Get-Command executed' }
    Assert-Inspection $probe $false

    # An empty exact name fails Get-Command's parameter validation after the
    # guard is installed. No production fault-injection hook is needed.
    $failureModules = Get-ModuleSnapshot
    $failureVariables = Get-GlobalVariableNames
    $failed = $false
    try { whichwhy '' --json } catch {
        if ($_.FullyQualifiedErrorId -notlike 'ParameterArgumentValidationError*') { throw }
        $failed = $true
    }
    if (-not $failed) { throw 'Controlled discovery failure did not occur' }
    Assert-PreferenceUnchanged
    Assert-ModuleState $false
    if ($failureModules -cne (Get-ModuleSnapshot) -or $failureVariables -cne (Get-GlobalVariableNames)) { throw 'Failed discovery leaked session state' }

    Microsoft.PowerShell.Core\Import-Module (Join-Path $moduleDir ($moduleName + '.psd1')) -Global
    $initText = [IO.File]::ReadAllText($initMarker)
    Assert-Inspection $probe $true 'function'
    Assert-Inspection "$moduleName\$probe" $true 'function'
    Assert-Inspection 'Get-Command' $true 'function'
    Assert-Inspection 'Microsoft.PowerShell.Core\Get-Command' $true 'cmdlet'
    if ($initText -cne [IO.File]::ReadAllText($initMarker)) { throw 'Loaded module was initialized again' }
    [Console]::WriteLine("PASS [passive/$PreferenceCase] PowerShell $($PSVersionTable.PSVersion)")
} } finally {
    $env:PSModulePath = $oldModulePath
    # Verify this unique disposable directory stays under the system temp root.
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
    if ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($lab)) -ne $tempRoot) { throw 'Unsafe fixture cleanup path' }
    [IO.Directory]::Delete($lab, $true)
}
