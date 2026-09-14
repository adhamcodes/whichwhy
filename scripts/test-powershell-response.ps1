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
$externalName = 'external-' + [char]0x754c + '.cmd'
$externalPath = Join-Path $fixtureDir $externalName
[void][IO.Directory]::CreateDirectory($fixtureDir)
[IO.File]::WriteAllText($externalPath, '@echo inspected-body-executed')
$env:PATH = $fixtureDir + ';' + $env:PATH

function Assert-ResponseEncoding {
    if (-not [object]::ReferenceEquals($consoleOutput, [Console]::OutputEncoding) -or
        -not [object]::ReferenceEquals($consoleInput, [Console]::InputEncoding)) { throw 'Caller console encoding object changed' }
    $after = Get-Variable OutputEncoding
    if (-not [object]::ReferenceEquals($outputVariable, $after) -or
        -not [object]::ReferenceEquals($outputValue, $after.Value) -or
        $outputOptions -ne $after.Options) { throw 'Caller OutputEncoding variable changed' }
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
                if ($LASTEXITCODE -ne $code -or -not [string]::Equals($human[0], ('WhichWhy ' + [char]0x2014 + ' ' + $identity), [StringComparison]::Ordinal)) { throw 'Human response changed identity/status' }
                Assert-ResponseEncoding
            }
            foreach ($json in @($false, $true)) {
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
        [Console]::WriteLine("PASS [response] caller code page $($consoleOutput.CodePage); both grammars/formats; hit/miss/native error/encoding failure; exact encoding objects retained")
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
