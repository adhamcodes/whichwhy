param(
    [Parameter(Mandatory = $true)]
    [string]$ExecutablePath
)

# Run in a fresh -NoProfile process. Wildcards (* ? [ ]) and reserved CLI words
# belong to F9 and are deliberately excluded from these real bridge fixtures.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility, Microsoft.PowerShell.Management
. (Join-Path $PSScriptRoot 'powershell-oracle-helpers.ps1')
$exe = (Resolve-Path -LiteralPath $ExecutablePath).Path
$marker = Join-Path ([IO.Path]::GetTempPath()) ('whichwhy-transport-' + [guid]::NewGuid().ToString('N') + '.marker')
$oldEncoding = [Console]::OutputEncoding
$oldAutoLoading = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference')
$oldAutoLoadingValue = $null
if ($null -ne $oldAutoLoading) { $oldAutoLoadingValue = $oldAutoLoading.Value }
$failures = @()

# ASCII source also works with Windows PowerShell's legacy script-file encoding.
$unicode = 'unicode-' + [char]0x03bb + '-' + [char]0x00e9 + '-' + [char]0x754c
$names = @(
    'normal-name',
    'name with space',
    "apostrophe'name",
    'name"with"quote',
    'backslash\name',
    'trailing-backslash\',
    'multiple\\backslashes',
    $unicode,
    'punctuation;,@#$(){}!',
    'mixed space "quote" \end\',
    'mixed \"quote" space\',
    ' leading and trailing ',
    ('decomposed-e' + [char]0x0301),
    ('supplementary-' + [char]::ConvertFromUtf32(0x1f642)),
    'bm9ybWFsLW5hbWU='
)

function Assert-Ordinal([string]$actual, [string]$expected, [string]$context) {
    if (-not [string]::Equals($actual, $expected, [StringComparison]::Ordinal)) {
        # Base64 diagnostics survive the terminal's own encoding/quoting rules.
        $want = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($expected))
        $got = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($actual))
        throw "$context identity mismatch: expected UTF-8 Base64=$want actual=$got"
    }
}

try { & {
    # Decode the CLI's UTF-8 stdout explicitly; this tests request transport
    # independently of the caller's console code page. Restore it in finally.
    [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
    $init = (& $exe init powershell) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Bridge generation failed' }
    Invoke-Expression $init
    # Keep the test oracle passive too, after loading the utility test helpers;
    # the separate F1 suite exercises the bridge's own restoration guard.
    $ExecutionContext.SessionState.PSVariable.Set('global:PSModuleAutoLoadingPreference', 'None')
    function Write-WhichWhyTransportMarker { [IO.File]::WriteAllText($marker, 'executed') }

    foreach ($name in $names) {
        # Backslashes are parsed as path/module qualification by Get-Command.
        # Those strings test the bridge with a missing result, not a fictitious
        # exact-name alias. All other cases use actual session-local aliases.
        $aliasFixture = -not $name.Contains('\')
        try {
            if ($aliasFixture) { Set-Alias -Name $name -Value Write-WhichWhyTransportMarker -Scope Script }
            $before = Get-OracleSessionSnapshot
            $json = @(whichwhy $name --json)
            $jsonCode = $LASTEXITCODE
            Assert-OracleText (Get-OracleSessionSnapshot) $before 'JSON transport session'
            if ((Test-OraclePathExists $marker)) { throw 'JSON inspection executed fixture' }
            $human = @(whichwhy $name)
            $humanCode = $LASTEXITCODE
            Assert-OracleText (Get-OracleSessionSnapshot) $before 'Human transport session'
            if ((Test-OraclePathExists $marker)) { throw 'Human inspection executed fixture' }
            $oracle = @(Get-PassiveOracleMatches $name)
            Assert-OracleText (Get-OracleSessionSnapshot) $before 'Transport oracle session'
            if ($aliasFixture) {
                if ($oracle.Count -eq 0) { throw 'Alias fixture was not discovered' }
                Assert-Ordinal ([string]$oracle[0].Name) $name 'Get-Command fixture'
            } elseif ($oracle.Count -ne 0) { throw 'Expected a controlled missing qualified name' }

            $doc = ($json -join "`n") | ConvertFrom-Json
            Assert-Ordinal ([string]$doc.command) $name 'JSON command'
            Assert-Ordinal ([string]$human[0]) ('WhichWhy ' + [char]0x2014 + ' ' + $name) 'Human heading'
            if ($jsonCode -ne $humanCode -or $jsonCode -ne [int](-not $aliasFixture)) { throw 'Unexpected human/JSON exit code' }
            if ($doc.resolution_scope -ne 'powershell-loaded-session' -or $doc.policy -ne 'powershell-loaded-session-order-v1' -or $doc.claim_strength -ne 'shell-observed') { throw 'Shell claim changed' }
            Assert-Ordinal ([string]$doc.shell.version) ([string]$PSVersionTable.PSVersion) 'Version'
            Assert-Ordinal ([string]$doc.shell.edition) ([string]$PSVersionTable.PSEdition) 'Edition'
            if ($doc.candidates.Count -ne $oracle.Count) { throw 'Candidate count changed' }
            for ($i = 0; $i -lt $oracle.Count; $i++) {
                Assert-OracleCandidate $doc.candidates[$i] $oracle[$i] "Candidate $i"
            }
            if ($aliasFixture) {
                Assert-OracleCandidate $doc.selected $oracle[0] 'Selected'
            } elseif ($null -ne $doc.selected) { throw 'Missing name fabricated a selection' }
            [Console]::WriteLine("PASS [transport] $name (alias fixture=$aliasFixture)")
        } catch {
            $failures += "${name}: $_"
        } finally {
            if ($aliasFixture) { Remove-Item -LiteralPath ('Alias:' + $name) -Force -ErrorAction SilentlyContinue }
            if ((Test-OraclePathExists $marker)) { throw 'Inspected fixture executed' }
        }
    }
    if ($failures.Count -gt 0) { throw ($failures -join "`n") }

    # Exercise malformed hidden requests through the native process too. All
    # arguments here are fixed protocol fixtures, never inspected executables.
    foreach ($entry in @('__powershell', '__powershell-json')) {
        foreach ($payload in @('!!!!', 'YQ', 'YR==', 'YQ==!!!!', '/w==', '""')) {
            $process = [Diagnostics.Process]::new()
            try {
                $process.StartInfo.FileName = $exe
                $process.StartInfo.Arguments = "$entry $payload 5.1 Desktop"
                $process.StartInfo.UseShellExecute = $false
                $process.StartInfo.CreateNoWindow = $true
                $process.StartInfo.RedirectStandardOutput = $true
                $process.StartInfo.RedirectStandardError = $true
                if (-not $process.Start()) { throw 'Malformed-request process did not launch' }
                $output = $process.StandardOutput.ReadToEnd()
                $errorText = $process.StandardError.ReadToEnd()
                $process.WaitForExit()
                if ($process.ExitCode -ne 2 -or $output.Length -ne 0 -or -not $errorText.StartsWith('whichwhy: ')) {
                    throw "Malformed $entry request was not rejected: $payload"
                }
            } finally { $process.Dispose() }
        }
    }
    [Console]::WriteLine('PASS [transport] malformed native requests fail closed in human and JSON endpoints')

    # .NET strings can contain unpaired UTF-16 surrogates, which cannot be
    # losslessly encoded as UTF-8. The bridge must throw rather than replace one.
    $rejected = $false
    try { whichwhy ('invalid-surrogate-' + [char]0xd800) --json } catch {
        $cause = $_.Exception
        while ($null -ne $cause.InnerException) { $cause = $cause.InnerException }
        if ($cause -isnot [Text.EncoderFallbackException]) { throw }
        $rejected = $true
    }
    if (-not $rejected) { throw 'Bridge silently replaced an invalid UTF-16 surrogate' }
    if ((Test-OraclePathExists $marker)) { throw 'Inspected fixture executed' }
    [Console]::WriteLine('PASS [transport] invalid UTF-16 rejected before native transport')
    [Console]::WriteLine("PASS [transport] all $($names.Count) identities; no fixture executed; PowerShell $($PSVersionTable.PSVersion) $($PSVersionTable.PSEdition)")
} } finally {
    Remove-Item -LiteralPath 'Function:whichwhy' -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath 'Function:Write-WhichWhyTransportMarker' -Force -ErrorAction SilentlyContinue
    [Console]::OutputEncoding = $oldEncoding
    if ($null -eq $oldAutoLoading) { $ExecutionContext.SessionState.PSVariable.Remove('global:PSModuleAutoLoadingPreference') }
    else { $oldAutoLoading.Value = $oldAutoLoadingValue }
    if ((Test-OraclePathExists $marker)) { [IO.File]::Delete($marker) }
}
