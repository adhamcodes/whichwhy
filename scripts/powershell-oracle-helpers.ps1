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
    # Oracle discovery must not execute defaults or inspected dynamic parameters.
    # A new function-local table preserves caller identity/content/options; an
    # inherited AllScope object is shared and must be refused before assignment.
    if ($null -ne $ExecutionContext.SessionState.PSVariable.Get('local:PSDefaultParameterValues') -and
        ($ExecutionContext.SessionState.PSVariable.Get('local:PSDefaultParameterValues').Options -band [System.Management.Automation.ScopedItemOptions]::AllScope)) {
        throw 'Oracle parameter defaults cannot be isolated from inherited AllScope state'
    }
    $PSDefaultParameterValues = @{}
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

# Independent public wire-shape assertions on real bridge reports. Metadata and
# semantic candidate order are checked separately against the shell oracle.
function Assert-OracleJSONContract($doc) {
    $keys = @('schema_version', 'command', 'resolution_scope', 'policy', 'claim_strength', 'shell', 'selected', 'candidates', 'limitations')
    if ($null -eq $doc.selected) { $keys += 'no_candidate_reason' } else { $keys += 'selection_reason' }
    $actualKeys = @($doc.PSObject.Properties.Name)
    if ($actualKeys.Count -ne $keys.Count) { throw 'Command JSON key count changed' }
    foreach ($key in $keys) {
        if ($actualKeys -cnotcontains $key) { throw "Command JSON missing key: $key" }
    }
    if ($doc.schema_version -ne 2 -or $doc.resolution_scope -cne 'powershell-loaded-session' -or $doc.policy -cne 'powershell-loaded-session-order-v1' -or $doc.claim_strength -cne 'shell-observed') { throw 'Command JSON identifiers changed' }
    if ($doc.command -isnot [string] -or $doc.candidates -isnot [array] -or $doc.limitations -isnot [array]) { throw 'Command JSON string/array shape changed' }
    foreach ($limit in $doc.limitations) { if ($limit -isnot [string] -or -not $limit) { throw 'Invalid JSON limitation' } }
    $reason = if ($null -eq $doc.selected) { $doc.no_candidate_reason } else { $doc.selection_reason }
    if ($reason -isnot [string] -or -not $reason) { throw 'Invalid JSON selection reason' }
    $shellKeys = @($doc.shell.PSObject.Properties.Name)
    if ($shellKeys.Count -ne 3 -or $shellKeys -cnotcontains 'name' -or $shellKeys -cnotcontains 'version' -or $shellKeys -cnotcontains 'edition') { throw 'Shell JSON shape changed' }
    Assert-OracleText $doc.shell.name 'PowerShell' 'Shell name'
    Assert-OracleText $doc.shell.version ([string]$PSVersionTable.PSVersion) 'Shell version'
    Assert-OracleText $doc.shell.edition ([string]$PSVersionTable.PSEdition) 'Shell edition'
    $allCandidates = @($doc.candidates)
    if ($null -ne $doc.selected) { $allCandidates += $doc.selected }
    foreach ($candidate in $allCandidates) {
        if ($null -eq $candidate -or $null -eq $candidate.PSObject.Properties['kind']) { throw 'Missing JSON candidate kind' }
        foreach ($property in $candidate.PSObject.Properties) {
            if (@('kind', 'name', 'path', 'source', 'alias_target') -cnotcontains $property.Name) { throw "Unexpected shell candidate key: $($property.Name)" }
            if ($property.Value -isnot [string] -or -not $property.Value) { throw "Optional JSON candidate field must be omitted when empty: $($property.Name)" }
        }
    }
    if (($null -eq $doc.selected) -ne ($doc.candidates.Count -eq 0)) { throw 'JSON selection/miss shape changed' }
}
