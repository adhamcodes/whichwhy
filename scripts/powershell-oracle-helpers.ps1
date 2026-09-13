# Test-only helpers. Load test dependencies before snapshots; discover subjects last.
function Test-OraclePathExists([string]$path) {
    # File.Exists returns false for access/IO errors as well as absence. An
    # unobservable marker must fail the oracle, not prove passive inspection.
    try {
        [void][IO.File]::GetAttributes($path)
        return $true
    } catch [IO.FileNotFoundException] { return $false }
      catch [IO.DirectoryNotFoundException] { return $false }
}

function Get-OracleSessionSnapshot {
    $state = @(
        foreach ($m in @(Microsoft.PowerShell.Core\Get-Module)) { "module:$($m.Name):$($m.Path)" }
        foreach ($v in @(Microsoft.PowerShell.Utility\Get-Variable -Scope Global)) { "variable:$($v.Name)" }
        foreach ($f in @(Microsoft.PowerShell.Management\Get-ChildItem Function:)) { "function:$($f.Name):$($f.Definition)" }
        foreach ($a in @(Microsoft.PowerShell.Management\Get-ChildItem Alias:)) { "alias:$($a.Name):$($a.Definition):$($a.Options)" }
        "PATH:$env:PATH"
        "PATHEXT:$env:PATHEXT"
        "PSModulePath:$env:PSModulePath"
        "location:$($ExecutionContext.SessionState.Path.CurrentLocation)"
    )
    [Array]::Sort($state, [StringComparer]::Ordinal)
    return $state -join "`n"
}

function Get-PassiveOracleMatches([string]$name) {
    $preference = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    $value = $null
    if ($null -ne $preference) { $value = $preference.Value }
    $changed = $false
    try {
        # An immutable None preference already provides the safe oracle state.
        if (-not (($value -is [string] -or $value -is [System.Management.Automation.PSModuleAutoLoadingPreference]) -and $value -eq 'None')) {
            $ExecutionContext.SessionState.PSVariable.Set('global:PSModuleAutoLoadingPreference', 'None')
            $changed = $true
            $guard = $ExecutionContext.SessionState.PSVariable.GetValue('global:PSModuleAutoLoadingPreference')
            if (-not (($guard -is [string] -or $guard -is [System.Management.Automation.PSModuleAutoLoadingPreference]) -and $guard -eq 'None')) { throw 'Oracle autoload guard unavailable' }
        }
        $discoveryErrors = @()
        $found = @(Microsoft.PowerShell.Core\Get-Command -Name $name -All -ListImported -ErrorAction SilentlyContinue -ErrorVariable discoveryErrors)
        foreach ($failure in $discoveryErrors) {
            if ($failure.Exception -isnot [System.Management.Automation.CommandNotFoundException]) { throw $failure }
        }
        return $found
    } finally {
        if ($changed) {
            if ($null -eq $preference) { $ExecutionContext.SessionState.PSVariable.Remove('global:PSModuleAutoLoadingPreference') }
            else { $preference.Value = $value }
        }
    }
}

function Assert-OracleText([string]$actual, [string]$expected, [string]$context) {
    if (-not [string]::Equals($actual, $expected, [StringComparison]::Ordinal)) {
        # Snapshot failures should identify the first changed line, not dump the
        # full caller environment and every function definition into CI logs.
        $wantLines = $expected.Split([char]10)
        $gotLines = $actual.Split([char]10)
        $i = 0
        while ($i -lt [Math]::Min($wantLines.Length, $gotLines.Length) -and [string]::Equals($wantLines[$i], $gotLines[$i], [StringComparison]::Ordinal)) { $i++ }
        $want = '<end>'
        $got = '<end>'
        if ($i -lt $wantLines.Length) { $want = $wantLines[$i].Substring(0, [Math]::Min(180, $wantLines[$i].Length)) }
        if ($i -lt $gotLines.Length) { $got = $gotLines[$i].Substring(0, [Math]::Min(180, $gotLines[$i].Length)) }
        throw "$context mismatch at line ${i}: expected <$want>, actual <$got>"
    }
}

function Assert-OracleCandidate($actual, $expected, [string]$context) {
    $kind = ([string]$expected.CommandType).ToLowerInvariant()
    if ($kind -eq 'externalscript') { $kind = 'external-script' }
    $path = ''
    if ($null -ne $expected.PSObject.Properties['Path']) { $path = [string]$expected.Path }
    $target = ''
    if ($kind -eq 'alias') { $target = [string]$expected.Definition }
    # Optional empty JSON fields are omitted by the renderer.
    foreach ($pair in @(@('kind', $kind), @('name', [string]$expected.Name), @('source', [string]$expected.Source), @('path', $path), @('alias_target', $target))) {
        $property = $actual.PSObject.Properties[$pair[0]]
        $value = ''
        if ($null -ne $property) { $value = [string]$property.Value }
        Assert-OracleText $value $pair[1] "$context $($pair[0])"
    }
}
