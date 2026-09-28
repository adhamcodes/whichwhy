param([Parameter(Mandatory = $true)][string]$ExecutablePath)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility, Microsoft.PowerShell.Management
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
$initialConsoleOutput = [Console]::OutputEncoding
$initialOutputVariable = Get-Variable OutputEncoding
$initialOutputValue = $initialOutputVariable.Value
$stderrFile = Join-Path ([IO.Path]::GetTempPath()) ('whichwhy-response-' + [guid]::NewGuid().ToString('N'))
$fixtureDir = $stderrFile + '-' + [char]0x754c
$oldPath = $env:PATH
Invoke-Expression ((& $exe init powershell) -join "`n")

# ASCII source works in 5.1 without a script BOM. Includes combining and
# supplementary identities, and Unicode candidate alias-target metadata.
$name = 'unicode-' + [char]0x03bb + '-' + [char]0x00e9 + '-' + [char]0x754c
$target = 'target-e' + [char]0x0301 + '-' + [char]::ConvertFromUtf32(0x1f642)
Set-Item -LiteralPath ('Function:' + $target) -Value { throw 'Inspected body executed' }
Set-Alias -Name $name -Value $target
# A separator in an observed alias target makes the existing evidence decoder
# reject the request. This exercises real native exit 2 and stderr delivery.
Set-Alias -Name 'ww-response-invalid-record' -Value ('unused' + [char]31 + 'target')
# AUD-002: metadata can be malformed even when the requested identity is valid.
$invalidTarget = 'target-' + [char]0xd800
$global:WhichWhyResponseDynamicMarker = 0
$global:WhichWhyResponseBodyMarker = 0
Set-Item -LiteralPath ('Function:' + $invalidTarget) -Value {
    [CmdletBinding()] param()
    dynamicparam {
        $global:WhichWhyResponseDynamicMarker++
        [System.Management.Automation.RuntimeDefinedParameterDictionary]::new()
    }
    end { $global:WhichWhyResponseBodyMarker++ }
}
Set-Alias -Name 'ww-response-invalid-metadata' -Value $invalidTarget
$externalName = 'external-' + [char]0x754c + '.cmd'
$externalPath = Join-Path $fixtureDir $externalName
[void][IO.Directory]::CreateDirectory($fixtureDir)
[IO.File]::WriteAllText($externalPath, '@echo inspected-body-executed')
$env:PATH = $fixtureDir + ';' + $env:PATH
$fixturePath = $env:PATH
$fixtureLocation = $ExecutionContext.SessionState.Path.CurrentLocation.Path
$fixtureModules = @(Microsoft.PowerShell.Core\Get-Module | ForEach-Object { $_.Name + '|' + $_.Path })
$preferences = @(foreach ($preferenceName in @('PSDefaultParameterValues', 'global:PSDefaultParameterValues', 'global:PSModuleAutoLoadingPreference')) {
    $variable = $ExecutionContext.SessionState.PSVariable.Get($preferenceName)
    $value = $null; $options = $null; $content = ''
    if ($null -ne $variable) {
        $value = $variable.Value; $options = $variable.Options
        $content = [System.Management.Automation.PSSerializer]::Serialize($value)
    }
    [pscustomobject]@{ Name = $preferenceName; Variable = $variable; Value = $value; Options = $options; Content = $content }
})

function Assert-ResponseEncoding {
    if (-not [object]::ReferenceEquals($consoleOutput, [Console]::OutputEncoding) -or
        -not [object]::ReferenceEquals($consoleInput, [Console]::InputEncoding)) { throw 'Caller console encoding object changed' }
    $after = Get-Variable OutputEncoding
    if (-not [object]::ReferenceEquals($outputVariable, $after) -or
        -not [object]::ReferenceEquals($outputValue, $after.Value) -or
        $outputOptions -ne $after.Options) { throw 'Caller OutputEncoding variable changed' }
}

function Assert-MetadataState {
    Assert-ResponseEncoding
    if ($global:WhichWhyResponseDynamicMarker -ne 0 -or $global:WhichWhyResponseBodyMarker -ne 0) { throw 'Malformed-metadata fixture executed' }
    if ($env:PATH -cne $fixturePath -or $ExecutionContext.SessionState.Path.CurrentLocation.Path -cne $fixtureLocation) { throw 'Caller PATH/location changed' }
    $modules = @(Microsoft.PowerShell.Core\Get-Module | ForEach-Object { $_.Name + '|' + $_.Path })
    if ([string]::Join([char]10, $modules) -cne [string]::Join([char]10, $fixtureModules)) { throw 'Loaded modules changed' }
    foreach ($snapshot in $preferences) {
        $after = $ExecutionContext.SessionState.PSVariable.Get($snapshot.Name)
        if (-not [object]::ReferenceEquals($snapshot.Variable, $after)) { throw 'Caller preference identity/absence changed' }
        if ($null -ne $after -and
            (-not [object]::ReferenceEquals($snapshot.Value, $after.Value) -or $snapshot.Options -ne $after.Options -or
             $snapshot.Content -cne [System.Management.Automation.PSSerializer]::Serialize($after.Value))) { throw 'Caller preference value/options/content changed' }
    }
    if ((Get-Alias 'ww-response-invalid-metadata').Definition -cne $invalidTarget) { throw 'Alias metadata changed' }
}

# AUD-003 control: capture only the known CLI with explicit UTF-8, independently
# of PowerShell's native-command decoder. The arguments are fixed product routes.
function Get-PublicUTF8Response([string]$route) {
    $process = [Diagnostics.Process]::new()
    try {
        $process.StartInfo.FileName = $exe
        $process.StartInfo.Arguments = $route + ' --json'
        $process.StartInfo.WorkingDirectory = $ExecutionContext.SessionState.Path.CurrentFileSystemLocation.ProviderPath
        $process.StartInfo.UseShellExecute = $false
        $process.StartInfo.CreateNoWindow = $true
        $process.StartInfo.RedirectStandardOutput = $true
        $process.StartInfo.RedirectStandardError = $true
        $process.StartInfo.StandardOutputEncoding = [Text.UTF8Encoding]::new($false, $true)
        $process.StartInfo.StandardErrorEncoding = [Text.UTF8Encoding]::new($false, $true)
        if (-not $process.Start()) { throw 'UTF-8 control did not start' }
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        $process.WaitForExit()
        [pscustomobject]@{ Stdout = $stdout.GetAwaiter().GetResult(); Stderr = $stderr.GetAwaiter().GetResult(); Code = $process.ExitCode }
    } finally { $process.Dispose() }
}

try {
    # First case is the untouched fresh-session encoding; never force UTF-8
    # before the regression. Later cases explicitly challenge both decoders.
    foreach ($codePage in @(0, 437, 1252, 65001)) {
        if ($codePage -ne 0) { [Console]::OutputEncoding = [Text.Encoding]::GetEncoding($codePage, [Text.EncoderExceptionFallback]::new(), [Text.DecoderExceptionFallback]::new()) }
        $OutputEncoding = [Text.UnicodeEncoding]::new($false, $false, $true)
        $consoleOutput = [Console]::OutputEncoding
        $consoleInput = [Console]::InputEncoding
        $outputVariable = Get-Variable OutputEncoding
        $outputValue = $outputVariable.Value
        $outputOptions = $outputVariable.Options
        foreach ($literal in @($false, $true)) {
            foreach ($identity in @($name, $target, $externalName, ('ww-response-missing-' + [char]0x754c))) {
                $arguments = @($identity)
                if ($literal) { $arguments = @('inspect') + $arguments }
                # Real bridge -> JSON -> ConvertFrom-Json: no recoding or
                # string repair between the public bridge and consumer.
                $doc = whichwhy @arguments --json 2> $stderrFile | ConvertFrom-Json
                $code = $LASTEXITCODE
                Assert-ResponseEncoding
                if (-not [string]::Equals($doc.command, $identity, [StringComparison]::Ordinal)) { throw "Corrupt request identity (code page $codePage)" }
                if ((Get-Item -LiteralPath $stderrFile).Length -ne 0) { throw 'Report wrote stderr' }
                $expectedCode = [int]$identity.StartsWith('ww-response-missing-')
                if ($code -ne $expectedCode) { throw 'Wrong JSON exit code' }
                if ($expectedCode -eq 0) {
                    if (-not [string]::Equals($doc.selected.name, $identity, [StringComparison]::Ordinal)) { throw 'Corrupt selected identity' }
                    if ($identity -ceq $name -and -not [string]::Equals($doc.selected.alias_target, $target, [StringComparison]::Ordinal)) { throw 'Corrupt alias target' }
                    if ($identity -ceq $externalName -and
                        (-not [string]::Equals($doc.selected.path, $externalPath, [StringComparison]::Ordinal) -or
                         -not [string]::Equals($doc.selected.source, $externalPath, [StringComparison]::Ordinal))) { throw 'Corrupt external path/source' }
                    foreach ($candidate in $doc.candidates) {
                        if (-not [string]::Equals($candidate.name, $identity, [StringComparison]::Ordinal)) { throw 'Corrupt candidate identity' }
                    }
                } elseif ($null -ne $doc.selected -or $doc.candidates.Count -ne 0) { throw 'Fabricated missing selection' }
                $human = @(whichwhy @arguments 2> $stderrFile)
                if ($LASTEXITCODE -ne $code -or -not [string]::Equals($human[0], ('WhichWhy - ' + $identity), [StringComparison]::Ordinal)) { throw 'Human response changed identity/status' }
                Assert-ResponseEncoding
            }
            foreach ($json in @($false, $true)) {
                $arguments = @('ww-response-invalid-metadata')
                if ($literal) { $arguments = @('inspect') + $arguments }
                if ($json) { $arguments += '--json' }
                foreach ($errorPreference in @('Continue', 'Stop')) {
                    $ErrorActionPreference = $errorPreference
                    $global:LASTEXITCODE = 73
                    $failure = $null
                    # Collect each success object as it arrives, so a later
                    # exception cannot hide a partial report from the assertion.
                    $success = [Collections.Generic.List[object]]::new()
                    try { whichwhy @arguments 2> $stderrFile | ForEach-Object { $success.Add($_) } } catch { $failure = $_ }
                    if ($ErrorActionPreference -cne $errorPreference) { throw 'Caller error preference changed' }
                    Assert-MetadataState
                    if ($LASTEXITCODE -ne 2 -or $success.Count -ne 0 -or $failure -isnot [System.Management.Automation.ErrorRecord]) { throw 'Malformed metadata was not rejected with exit 2 and no stdout' }
                    $cause = $failure.Exception
                    while ($null -ne $cause -and $cause -isnot [Text.EncoderFallbackException]) { $cause = $cause.InnerException }
                    if ($null -eq $cause) { throw 'Malformed metadata failed for an unrelated reason' }
                    # A caught pre-native encoding exception is a PowerShell
                    # error, not a CLI JSON envelope or a second stderr message.
                    if ((Get-Item -LiteralPath $stderrFile).Length -ne 0) { throw 'Caught metadata failure also wrote stderr' }
                }
                $ErrorActionPreference = 'Stop'
                $arguments = @('ww-response-invalid-record')
                if ($literal) { $arguments = @('inspect') + $arguments }
                if ($json) { $arguments += '--json' }
                $ErrorActionPreference = 'Continue'
                try { $output = @(whichwhy @arguments 2> $stderrFile) } finally { $ErrorActionPreference = 'Stop' }
                if ($LASTEXITCODE -ne 2 -or $output.Count -ne 0 -or (Get-Content -Raw -LiteralPath $stderrFile) -notlike '*whichwhy: malformed PowerShell evidence record*') { throw 'Native failure lost stdout/stderr/exit semantics' }
                Assert-ResponseEncoding
                $ErrorActionPreference = 'Continue'
                try { $merged = @(whichwhy @arguments 2>&1) } finally { $ErrorActionPreference = 'Stop' }
                if ($LASTEXITCODE -ne 2 -or $merged.Count -ne 1 -or $merged[0] -isnot [System.Management.Automation.ErrorRecord]) { throw 'Native stderr is not redirectable as stream 2' }
                Assert-ResponseEncoding
                $arguments = @(('invalid-' + [char]0xd800))
                if ($literal) { $arguments = @('inspect') + $arguments }
                if ($json) { $arguments += '--json' }
                $rejected = $false
                try { whichwhy @arguments | Out-Null } catch { $rejected = $true }
                if (-not $rejected) { throw 'Invalid request encoding was accepted' }
                Assert-ResponseEncoding
            }
        }
        foreach ($route in @('path', 'doctor')) {
            $native = (& $exe $route --json 2> $stderrFile) | ConvertFrom-Json
            $nativeCode = $LASTEXITCODE
            if ((Get-Item -LiteralPath $stderrFile).Length -ne 0) { throw 'Public native report wrote stderr' }
            $forwarded = (whichwhy $route --json 2> $stderrFile) | ConvertFrom-Json
            if ($LASTEXITCODE -ne $nativeCode -or (Get-Item -LiteralPath $stderrFile).Length -ne 0) { throw 'Public forwarding changed exit/stderr behavior' }
            $control = Get-PublicUTF8Response $route
            if ($control.Code -ne $nativeCode -or $control.Stderr -ne '') { throw 'UTF-8 native control changed exit/stderr behavior' }
            $decoded = $control.Stdout | ConvertFrom-Json
            if ($route -eq 'path') {
                $nativeRaw = $native.raw_value; $forwardedRaw = $forwarded.raw_value; $controlRaw = $decoded.raw_value
            } else {
                $nativeRaw = $native.command_discovery.process_path.raw_value
                $forwardedRaw = $forwarded.command_discovery.process_path.raw_value
                $controlRaw = $decoded.command_discovery.process_path.raw_value
            }
            if (-not [string]::Equals($nativeRaw, $forwardedRaw, [StringComparison]::Ordinal)) { throw 'Public route no longer uses native PowerShell decoding' }
            if (-not [string]::Equals($controlRaw, $fixturePath, [StringComparison]::Ordinal)) { throw 'Underlying CLI UTF-8 bytes changed PATH evidence' }
            $exact = [string]::Equals($forwardedRaw, $fixturePath, [StringComparison]::Ordinal)
            if (($consoleOutput.CodePage -eq 65001 -and -not $exact) -or ($codePage -in @(437, 1252) -and $exact)) { throw 'Public decoding distinction was not exercised' }
            Assert-MetadataState
            [Console]::WriteLine("PASS [public-response] $route; requested code page $codePage, actual $($consoleOutput.CodePage); native forwarding exact=$exact; UTF-8 capture exact=True")
        }
        [Console]::WriteLine("PASS [response] caller code page $($consoleOutput.CodePage); both grammars/formats; hit/miss/native error/command and metadata encoding failure; caller state retained")
    }
} finally {
    $env:PATH = $oldPath
    [Console]::OutputEncoding = $initialConsoleOutput
    $initialOutputVariable.Value = $initialOutputValue
    if ([IO.File]::Exists($stderrFile)) { [IO.File]::Delete($stderrFile) }
    if ([IO.File]::Exists($externalPath)) { [IO.File]::Delete($externalPath) }
    if ([IO.Directory]::Exists($fixtureDir)) { [IO.Directory]::Delete($fixtureDir) }
}
[Console]::WriteLine("PASS [response] PowerShell $($PSVersionTable.PSVersion)")
