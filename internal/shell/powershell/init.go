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
		`$matches = @(Microsoft.PowerShell.Core\Get-Command -Name $command -All -ListImported -ErrorAction SilentlyContinue)`,
		`} finally {`,
		`if ($whichWhyAutoLoadingChanged) {`,
		`if ($null -eq $whichWhyAutoLoadingVariable) { $ExecutionContext.SessionState.PSVariable.Remove('global:PSModuleAutoLoadingPreference') }`,
		`else { $whichWhyAutoLoadingVariable.Value = $whichWhyAutoLoadingValue }`,
		`}`,
		`};`,
	}, " ")

	return fmt.Sprintf(`function global:whichwhy { $whichWhyExecutable = '%s'; if ($args.Count -eq 0) { & $whichWhyExecutable; return }; $command = [string]$args[0]; $jsonOutput = $args.Count -eq 2 -and [string]$args[1] -eq '--json'; $inspectionArgumentCountOK = $args.Count -eq 1 -or $jsonOutput; if (-not $inspectionArgumentCountOK -or $command -in @('-h', '--help', 'help', '-v', '--version', 'version', 'path', 'doctor', 'init')) { & $whichWhyExecutable @args; return }; if ($command -match '[\*\?\[\]]') { & $whichWhyExecutable @args; return }; %s $records = @(); foreach ($item in $matches) { $path = ''; if ($null -ne $item.PSObject.Properties['Path']) { $path = [string]$item.Path }; $aliasTarget = ''; if ([string]$item.CommandType -eq 'Alias') { $aliasTarget = [string]$item.Definition }; $fields = @([string]$item.CommandType, [string]$item.Name, [string]$item.Source, $path, $aliasTarget); $record = $fields -join [char]31; $records += [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($record)) }; $entryPoint = '__powershell'; if ($jsonOutput) { $entryPoint = '__powershell-json' }; $encodedCommand = [Convert]::ToBase64String([Text.UTF8Encoding]::new($false, $true).GetBytes($command)); & $whichWhyExecutable $entryPoint $encodedCommand ([string]$PSVersionTable.PSVersion) ([string]$PSVersionTable.PSEdition) @records }`, escapedExecutable, passiveDiscovery)
}
