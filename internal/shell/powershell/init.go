package powershell

import (
	"fmt"
	"strings"
)

// InitScript returns a single-line PowerShell function definition so callers can
// pass the native command output directly to Invoke-Expression on Windows
// PowerShell 5.1 as well as modern PowerShell.
func InitScript(executable string) string {
	escapedExecutable := strings.ReplaceAll(executable, "'", "''")
	routing := strings.Join([]string{
		`if ($args.Count -eq 0) { & $whichWhyExecutable; return };`,
		`$command = [string]$args[0]; $literal = $command -ceq 'inspect'; $jsonOutput = $false;`,
		`if ($literal) {`,
		`if ($args.Count -lt 2 -or $args.Count -gt 3 -or [string]$args[1] -eq '' -or ($args.Count -eq 3 -and [string]$args[2] -cne '--json')) { [Console]::Error.WriteLine('whichwhy: inspect requires one nonempty command name and optional trailing --json'); $global:LASTEXITCODE = 2; return };`,
		`$command = [string]$args[1]; $jsonOutput = $args.Count -eq 3; $global:LASTEXITCODE = 2`,
		`} else {`,
		// User arguments must never be forwarded as private shell evidence.
		`if ($command -cin @('__powershell', '__powershell-json')) { [Console]::Error.WriteLine('whichwhy: use inspect to inspect this command name'); $global:LASTEXITCODE = 2; return };`,
		`$jsonOutput = $args.Count -eq 2 -and [string]$args[1] -eq '--json';`,
		`$inspectionArgumentCountOK = $args.Count -eq 1 -or $jsonOutput;`,
		`if (-not $inspectionArgumentCountOK -or $command -in @('-h', '--help', 'help', '-v', '--version', 'version', 'path', 'doctor', 'init')) { & $whichWhyExecutable @args; return };`,
		`if ($command -match '[\*\?\[\]]') { & $whichWhyExecutable @args; return }`,
		`};`,
	}, " ")

	// Broad discovery preserves order within an exact name, including shadowed
	// exports and literal filenames. It sorts different filenames, however, so it
	// cannot supply extension-inferred PATH/PATHEXT order. For non-wildcard names,
	// keep the narrow external query's actual observations. Wildcard
	// identities match full names only, as opposed to inventing suffix inference
	// the shell's wildcard discovery does not perform. The oracle suites prove
	// the local/external boundary and each order; no candidates are sorted.
	literalDiscovery := strings.Join([]string{
		`$wildcardName = $command.IndexOfAny([char[]]'*?[]') -ge 0;`,
		`$externalTypes = [System.Management.Automation.CommandTypes]::Application -bor [System.Management.Automation.CommandTypes]::ExternalScript;`,
		`$broadTypes = [System.Management.Automation.CommandTypes]::All; if (-not $wildcardName) { $broadTypes = $broadTypes -band (-bnot $externalTypes) };`,
		`$broadCommands = @(Microsoft.PowerShell.Core\Get-Command -Name '*' -CommandType $broadTypes -All -ListImported -ErrorAction Stop);`,
		`$matches = @(foreach ($item in $broadCommands) { if ([string]::Equals($item.Name, $command, [StringComparison]::OrdinalIgnoreCase) -or (($item.CommandType -band $externalTypes) -eq 0 -and $item.Source -and [string]::Equals(($item.Source + '\' + $item.Name), $command, [StringComparison]::OrdinalIgnoreCase))) { $item } });`,
		`if (-not $wildcardName) {`,
		`$discoveryErrors = @();`,
		`$externalCommands = @(Microsoft.PowerShell.Core\Get-Command -Name $command -CommandType $externalTypes -All -ListImported -ErrorAction SilentlyContinue -ErrorVariable discoveryErrors);`,
		`foreach ($failure in $discoveryErrors) { if ($failure.Exception -isnot [System.Management.Automation.CommandNotFoundException]) { throw $failure } };`,
		`$matches += $externalCommands;`,
		`}`,
	}, " ")
	// Only the inspected name needs new transport: version/edition have fixed,
	// quoting-safe domains and records already use Base64. Strict UTF-8 encoding
	// rejects unpaired UTF-16 surrogates instead of changing command identity.

	// Qualified Get-Command can auto-import even with -ListImported, and on
	// PowerShell 5.1 and 7 it observes the global autoload preference, not a
	// function-local override. Guard only discovery and restore the original
	// variable (including absence) before collecting records or invoking the CLI.
	// Use session-state APIs so the guard itself needs no utility-module import.
	passiveDiscovery := strings.Join([]string{
		`$whichWhyAutoLoadingVariable = $ExecutionContext.SessionState.PSVariable.Get('global:PSModuleAutoLoadingPreference');`,
		`$whichWhyAutoLoadingValue = $null;`,
		`if ($null -ne $whichWhyAutoLoadingVariable) { $whichWhyAutoLoadingValue = $whichWhyAutoLoadingVariable.Value };`,
		`$whichWhyAutoLoadingChanged = $false;`,
		`try {`,
		`if (-not (($whichWhyAutoLoadingValue -is [string] -or $whichWhyAutoLoadingValue -is [System.Management.Automation.PSModuleAutoLoadingPreference]) -and 'None' -eq $whichWhyAutoLoadingValue)) {`,
		`$ExecutionContext.SessionState.PSVariable.Set('global:PSModuleAutoLoadingPreference', 'None');`,
		`$whichWhyAutoLoadingChanged = $true;`,
		`$whichWhyAutoLoadingGuardValue = $ExecutionContext.SessionState.PSVariable.GetValue('global:PSModuleAutoLoadingPreference');`,
		`if (-not (($whichWhyAutoLoadingGuardValue -is [string] -or $whichWhyAutoLoadingGuardValue -is [System.Management.Automation.PSModuleAutoLoadingPreference]) -and 'None' -eq $whichWhyAutoLoadingGuardValue)) { throw 'whichwhy: cannot disable PowerShell module auto-loading for passive inspection' }`,
		`};`,
		`if ($literal) { ` + literalDiscovery + ` } else { $matches = @(Microsoft.PowerShell.Core\Get-Command -Name $command -All -ListImported -ErrorAction SilentlyContinue) }`,
		`} finally {`,
		`if ($whichWhyAutoLoadingChanged) {`,
		`if ($null -eq $whichWhyAutoLoadingVariable) { $ExecutionContext.SessionState.PSVariable.Remove('global:PSModuleAutoLoadingPreference') }`,
		`else { $whichWhyAutoLoadingVariable.Value = $whichWhyAutoLoadingValue }`,
		`}`,
		`};`,
	}, " ")

	return fmt.Sprintf(`function global:whichwhy { $whichWhyExecutable = '%s'; %s %s $records = @(); foreach ($item in $matches) { $path = ''; if ($null -ne $item.PSObject.Properties['Path']) { $path = [string]$item.Path }; $aliasTarget = ''; if ([string]$item.CommandType -eq 'Alias') { $aliasTarget = [string]$item.Definition }; $fields = @([string]$item.CommandType, [string]$item.Name, [string]$item.Source, $path, $aliasTarget); $record = $fields -join [char]31; $records += [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($record)) }; $entryPoint = '__powershell'; if ($jsonOutput) { $entryPoint = '__powershell-json' }; $encodedCommand = [Convert]::ToBase64String([Text.UTF8Encoding]::new($false, $true).GetBytes($command)); & $whichWhyExecutable $entryPoint $encodedCommand ([string]$PSVersionTable.PSVersion) ([string]$PSVersionTable.PSEdition) @records }`, escapedExecutable, routing, passiveDiscovery)
}
