param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

# Composed by the existing oracle gate, in a fresh process on both shell versions.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility, Microsoft.PowerShell.Management
. (Join-Path $PSScriptRoot 'powershell-oracle-helpers.ps1')
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
$lab = Join-Path ([IO.Path]::GetTempPath()) ('whichwhy-literal-' + [guid]::NewGuid().ToString('N'))
$oldPath = $env:PATH
$oldPathExt = $env:PATHEXT
$oldEncoding = [Console]::OutputEncoding

function Assert-LiteralReport([string]$name, [object[]]$expected) {
    $before = Get-OracleSessionSnapshot
    $preference = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
    $value = $null
    $options = $null
    if ($null -ne $preference) { $value = $preference.Value; $options = $preference.Options }
    $texts = @()
    foreach ($machine in @($true, $false)) {
        if ($machine) { $text = @(whichwhy inspect $name --json) }
        else { $text = @(whichwhy inspect $name) }
        if ($LASTEXITCODE -ne [int]($expected.Count -eq 0)) { throw "Wrong literal exit code for <$name>: $LASTEXITCODE" }
        Assert-OracleText (Get-OracleSessionSnapshot) $before 'Literal session snapshot'
        $after = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
        if (-not [object]::ReferenceEquals($preference, $after)) { throw 'Literal preference identity/absence changed' }
        if ($null -ne $after -and (-not [object]::ReferenceEquals($value, $after.Value) -or $options -ne $after.Options)) { throw 'Literal preference value/options changed' }
        if (Test-OraclePathExists $marker) { throw 'Literal inspection executed a fixture' }
        $texts += ($text -join "`n")
    }
    $doc = $texts[0] | ConvertFrom-Json
    Assert-OracleJSONContract $doc
    Assert-OracleText $doc.command $name 'Literal command identity'
    Assert-OracleText ($texts[1].Split([char]10)[0]) ('WhichWhy - ' + $name) 'Literal human heading'
    if ($doc.resolution_scope -ne 'powershell-loaded-session' -or $doc.policy -ne 'powershell-loaded-session-order-v1' -or $doc.claim_strength -ne 'shell-observed') { throw 'Literal shell claim changed' }
    foreach ($claim in @($doc.resolution_scope, $doc.policy, $doc.claim_strength)) { if (-not $texts[1].Contains($claim)) { throw 'Human shell claim changed' } }
    if ($doc.candidates.Count -ne $expected.Count) { throw "Literal candidate count for <$name>: $($doc.candidates.Count), expected $($expected.Count)" }
    $position = 0
    for ($i = 0; $i -lt $expected.Count; $i++) {
        Assert-OracleCandidate $doc.candidates[$i] $expected[$i] "Literal <$name> candidate $i"
        $label = [string]$expected[$i].CommandType + ' ' + [string]$expected[$i].Name
        if ($expected[$i].CommandType -eq 'Alias') { $label += ' -> ' + $expected[$i].Definition }
        $position = $texts[1].IndexOf($label, $position, [StringComparison]::Ordinal)
        if ($position -lt 0) { throw "Human candidate missing/out of order for <$name>" }
        $position += $label.Length
        if ($expected[$i].CommandType -eq 'Application') {
            $position = $texts[1].IndexOf([string]$expected[$i].Path, $position, [StringComparison]::Ordinal)
            if ($position -lt 0) { throw 'Human external path missing/out of order' }
            $position += ([string]$expected[$i].Path).Length
        }
    }
    if ($expected.Count) { Assert-OracleCandidate $doc.selected $expected[0] 'Literal selected' }
    elseif ($null -ne $doc.selected -or -not $texts[1].Contains($doc.no_candidate_reason)) { throw 'Literal miss fabricated selection or lost reason' }
    [Console]::WriteLine("PASS [literal] <$name>, candidates=$($expected.Count)")
}

try { & {
    $second = Join-Path $lab 'second path'
    [void][IO.Directory]::CreateDirectory($second)
    $env:PATH = "$lab;$second"
    $env:PATHEXT = '.EXE;.CMD;.BAT'
    [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
    $marker = Join-Path $lab 'body.marker'
    $init = (& $exe init powershell) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Bridge generation failed' }
    Invoke-Expression $init
    function Write-F9LiteralMarker { [IO.File]::WriteAllText($marker, 'executed') }

    # Independently specified fixture identities. Provider lookup, not product
    # discovery or an escaped query, supplies the expected shell-local metadata.
    $names = @(
        'inspect', 'path', 'doctor', 'init', 'help', 'version', '-h', '--help', '-v', '--version', '--json', '--',
        '__powershell', '__powershell-json', '*', '[', ']',
        'ww*f9', 'ww?f9', 'ww[f9]', 'ww]f9', 'ww[abc]f9', 'ww[z-a]f9', 'ww[f9',
        'ww`f9', 'ww``f9', 'ww`*?[]f9', 'ww`[f9]', 'ww*f9`',
        'ww f9', "ww'f9", ('ww' + [char]0x754c + 'f9'), 'WwCaseF9',
        'ww[f9].cmd', 'ww]f9.cmd', 'ww`f9.cmd', 'ww`[f9].cmd'
    )
    # Competitors make accidental pattern expansion observable for every class.
    foreach ($name in @('wwABCf9', 'wwXYZf9', 'wwaf9', 'wwbf9', 'wwcf9', 'wwf', 'ww9', 'wwf9', 'ww[f9].cmd.cmd')) {
        Set-Alias -Name $name -Value Write-F9LiteralMarker -Scope Local -Force
        Set-Item -LiteralPath ('Function:' + $name) -Value { [IO.File]::WriteAllText($marker, 'executed') }
        [IO.File]::WriteAllText((Join-Path $lab ($name + '.cmd')), "@echo off`r`necho executed>`"$marker`"`r`n")
    }
    foreach ($name in $names) {
        Set-Alias -Name $name -Value Write-F9LiteralMarker -Scope Local -Force
        Set-Item -LiteralPath ('Function:' + $name) -Value { [IO.File]::WriteAllText($marker, 'executed') }
    }
    foreach ($name in $names) {
        $expected = @((Get-Item -LiteralPath ('Alias:' + $name)), (Get-Item -LiteralPath ('Function:' + $name)))
        if ($name.EndsWith('.cmd', [StringComparison]::Ordinal)) {
            foreach ($dir in @($lab, $second)) {
                $path = Join-Path $dir $name
                [IO.File]::WriteAllText($path, "@echo off`r`necho executed>`"$marker`"`r`n")
                $expected += [pscustomobject]@{ CommandType = 'Application'; Name = $name; Source = $path; Path = $path; Definition = $path }
            }
        }
        $requested = $name
        if ($name -eq 'WwCaseF9') { $requested = 'wwcasef9' }
        Assert-LiteralReport $requested $expected
        if ($name.EndsWith('.cmd', [StringComparison]::Ordinal)) {
            # The same filename without shell-local competitors: two PATH entries.
            Remove-Item -LiteralPath ('Alias:' + $name), ('Function:' + $name) -Force
            Assert-LiteralReport $requested @($expected[2], $expected[3])
        }
    }
    Assert-LiteralReport 'wwMissing*F9' @()
    Assert-LiteralReport 'wwMissing`F9' @()
    # Exercise the canonical unquoted option-looking operand at the actual
    # PowerShell parser boundary as well as the variable-based identity matrix.
    $optionJSON = @(whichwhy inspect --json --json)
    if ($LASTEXITCODE -ne 0) { throw 'Unquoted --json identity was not inspected' }
    Assert-OracleText (($optionJSON -join "`n" | ConvertFrom-Json).command) '--json' 'Unquoted JSON identity'
    $optionHuman = @(whichwhy inspect --json)
    if ($LASTEXITCODE -ne 0) { throw 'Unquoted human --json identity was not inspected' }
    Assert-OracleText $optionHuman[0] ('WhichWhy - --json') 'Unquoted human identity'
    # An application's Source is its file path, not a module qualifier.
    Assert-LiteralReport ((Join-Path $lab 'ww[f9].cmd') + '\ww[f9].cmd') @()

    # Two exports of the same literal and ordinary names prove source/order and
    # qualified selection, including an export shadowed by a later import.
    foreach ($moduleName in @('F9FirstModule', 'F9SecondModule')) {
        $module = New-Module -Name $moduleName -ScriptBlock {
            function Get-F9Module { throw 'Module fixture executed' }
            Set-Item -LiteralPath 'Function:script:wwModule[f9]' -Value { throw 'Module fixture executed' }
            Export-ModuleMember -Function *
        }
        Import-Module $module
    }
    $expected = @(Get-PassiveOracleMatches 'Get-F9Module')
    if ($expected.Count -ne 2 -or $expected[0].Source -ne 'F9SecondModule' -or $expected[1].Source -ne 'F9FirstModule') { throw 'Module order control invalid' }
    Assert-LiteralReport 'Get-F9Module' $expected
    foreach ($moduleName in @('F9FirstModule', 'F9SecondModule')) {
        $module = Get-Module -Name $moduleName
        Assert-LiteralReport ($moduleName + '\wwModule[f9]') @($module.ExportedFunctions['wwModule[f9]'])
    }

    # Exercise the generated function, not just Go's parser. No malformed public
    # shape can forward arbitrary payloads into the private evidence endpoint.
    foreach ($arguments in @(
        @('inspect'), @('inspect', ''), @('inspect', 'a', 'b'), @('inspect', 'a', 'b', '--json'),
        @('inspect', 'a', '--json', 'extra'), @('inspect', '__powershell', 'YQ==', '5.1', 'Desktop'),
        @('__powershell', 'YQ==', '5.1', 'Desktop'), @('__powershell-json', 'YQ==', '5.1', 'Desktop')
    )) {
        $text = @(whichwhy @arguments)
        if ($LASTEXITCODE -ne 2 -or $text.Count -ne 0) { throw 'Malformed public request reached inspection/evidence' }
    }
    if (Test-OraclePathExists $marker) { throw 'A fixture executed' }
    [Console]::WriteLine("PASS [literal] all identities, protocol isolation and passive snapshots; PowerShell $($PSVersionTable.PSVersion)")
} } finally {
    $env:PATH = $oldPath
    $env:PATHEXT = $oldPathExt
    [Console]::OutputEncoding = $oldEncoding
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
    if ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($lab)) -ne $tempRoot) { throw 'Unsafe literal fixture cleanup path' }
    if ([IO.Directory]::Exists($lab)) { [IO.Directory]::Delete($lab, $true) }
}
