param([Parameter(Mandatory = $true)][string]$ExecutablePath)

# The oracle parent launches this suite in a fresh 5.1/7 process. Each collision
# lives in a disposable caller scope (including Constant variables).
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Microsoft.PowerShell.Core\Import-Module Microsoft.PowerShell.Utility
$wwTestExe = (Resolve-Path -LiteralPath $ExecutablePath).Path
Invoke-Expression ((& $wwTestExe init powershell) -join "`n")
function Test-WhichWhyIsolationTarget { throw 'Inspected body executed' }

# Derive writes from the generated function, so adding scratch state without
# extending the guard fails this regression. Include foreach and ErrorVariable.
$wwTestWrites = @('command')
$wwTestAst = (Get-Item Function:whichwhy).ScriptBlock.Ast
foreach ($wwTestNode in $wwTestAst.FindAll({ param($n) $n -is [System.Management.Automation.Language.AssignmentStatementAst] }, $true)) {
    if ($wwTestNode.Left -is [System.Management.Automation.Language.VariableExpressionAst]) {
        $wwTestWrites += $wwTestNode.Left.VariablePath.UserPath
    }
}
foreach ($wwTestNode in $wwTestAst.FindAll({ param($n) $n -is [System.Management.Automation.Language.ForEachStatementAst] }, $true)) {
    $wwTestWrites += $wwTestNode.Variable.VariablePath.UserPath
}
foreach ($wwTestNode in $wwTestAst.FindAll({ param($n) $n -is [System.Management.Automation.Language.CommandAst] }, $true)) {
    for ($wwTestIndex = 0; $wwTestIndex -lt $wwTestNode.CommandElements.Count - 1; $wwTestIndex++) {
        if ($wwTestNode.CommandElements[$wwTestIndex].Extent.Text -eq '-ErrorVariable') {
            $wwTestWrites += $wwTestNode.CommandElements[$wwTestIndex + 1].Extent.Text
        }
    }
}
$wwTestWrites = @($wwTestWrites | Where-Object { $_ -notlike '*:*' } | Sort-Object -Unique)

function Assert-WhichWhyIsolation([string]$wwTestName, [string]$wwTestOptions) {
    New-Variable -Name $wwTestName -Scope Local -Option $wwTestOptions -Value ([object]::new())
    $wwTestOriginal = Get-Variable -Name $wwTestName -Scope Local
    $wwTestValue = $wwTestOriginal.Value
    $wwTestOriginalOptions = $wwTestOriginal.Options
    foreach ($wwTestLiteral in @($false, $true)) {
        foreach ($wwTestJSON in @($false, $true)) {
            foreach ($wwTestTarget in @('Test-WhichWhyIsolationTarget', 'ww-isolation-missing', ('invalid-' + [char]0xd800))) {
                $wwTestArguments = @($wwTestTarget)
                if ($wwTestLiteral) { $wwTestArguments = @('inspect') + $wwTestArguments }
                if ($wwTestJSON) { $wwTestArguments += '--json' }
                $wwTestRejected = $false
                $wwTestOutput = @()
                try { $wwTestOutput = @(whichwhy @wwTestArguments) } catch {
                    if ([string]$_ -notlike '*whichwhy: cannot isolate PowerShell scratch variable*') { throw }
                    $wwTestRejected = $true
                }
                $wwTestAfter = Get-Variable -Name $wwTestName -Scope Local
                if (-not [object]::ReferenceEquals($wwTestOriginal, $wwTestAfter) -or
                    -not [object]::ReferenceEquals($wwTestValue, $wwTestAfter.Value) -or
                    $wwTestAfter.Options -ne $wwTestOriginalOptions) { throw "Caller variable mutated: $wwTestName ($wwTestOptions)" }
                if (-not $wwTestRejected -or $wwTestOutput.Count -ne 0 -or $LASTEXITCODE -ne 2) { throw "Unsafe collision was not refused: $wwTestName" }
            }
        }
    }
}

foreach ($wwTestOptions in @('AllScope', 'AllScope,ReadOnly', 'AllScope,Constant')) {
    foreach ($wwTestName in $wwTestWrites) { Assert-WhichWhyIsolation $wwTestName $wwTestOptions }
}

# Non-AllScope variables are shadowed normally: successful and missing results
# must still work with the exact same caller-owned variable names.
& {
    foreach ($wwTestName in $wwTestWrites) { New-Variable -Name $wwTestName -Scope Local -Value 'CALLER-SENTINEL' }
    foreach ($wwTestLiteral in @($false, $true)) {
        foreach ($wwTestJSON in @($false, $true)) {
            foreach ($wwTestTarget in @('Test-WhichWhyIsolationTarget', 'ww-isolation-missing')) {
                $wwTestArguments = @($wwTestTarget)
                if ($wwTestLiteral) { $wwTestArguments = @('inspect') + $wwTestArguments }
                if ($wwTestJSON) { $wwTestArguments += '--json' }
                $wwTestOutput = @(whichwhy @wwTestArguments)
                if ($LASTEXITCODE -ne [int]($wwTestTarget -eq 'ww-isolation-missing') -or $wwTestOutput.Count -eq 0) { throw 'Non-AllScope inspection regressed' }
                foreach ($wwTestName in $wwTestWrites) {
                    if ((Get-Variable -Name $wwTestName -Scope Local).Value -cne 'CALLER-SENTINEL') { throw 'Non-AllScope caller mutated' }
                }
            }
        }
    }
}

# Original #31 reproduction uses a global binding. Check its object as well as
# its sentinel, even though the safe result is now an explicit refusal.
New-Variable command -Value 'CALLER-SENTINEL' -Scope Global -Option AllScope
$wwTestOriginal = Get-Variable command -Scope Global
$wwTestValue = $wwTestOriginal.Value
try {
    $wwTestRejected = $false
    try { whichwhy Get-Date --json | Out-Null } catch { $wwTestRejected = $true }
    $wwTestAfter = Get-Variable command -Scope Global
    if (-not $wwTestRejected -or $LASTEXITCODE -ne 2 -or
        -not [object]::ReferenceEquals($wwTestOriginal, $wwTestAfter) -or
        -not [object]::ReferenceEquals($wwTestValue, $wwTestAfter.Value) -or
        $wwTestAfter.Options -ne [System.Management.Automation.ScopedItemOptions]::AllScope) { throw 'Original #31 reproduction mutated caller state' }
} finally { Remove-Variable command -Scope Global -Force }
[Console]::WriteLine("PASS [isolation] $($wwTestWrites.Count) scratch names, three AllScope options, both grammars/formats, hit/miss/failure; PowerShell $($PSVersionTable.PSVersion)")
