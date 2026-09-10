package powershell

import (
	"fmt"
	"strings"
)

// InitScript returns the PowerShell function used to collect live session
// command evidence before delegating analysis back to the WhichWhy executable.
func InitScript(executable string) string {
	escapedExecutable := strings.ReplaceAll(executable, "'", "''")

	return fmt.Sprintf(`function global:whichwhy {
    $whichWhyExecutable = '%s'

    if ($args.Count -eq 0) {
        & $whichWhyExecutable
        return
    }

    $command = [string]$args[0]

    if ($args.Count -ne 1 -or $command -in @('-h', '--help', 'help', '-v', '--version', 'version', 'path', 'doctor', 'init')) {
        & $whichWhyExecutable @args
        return
    }

    if ($command -match '[\*\?\[\]]') {
        & $whichWhyExecutable @args
        return
    }

    $matches = @(Microsoft.PowerShell.Core\Get-Command -Name $command -All -ListImported -ErrorAction SilentlyContinue)
    $records = @()

    foreach ($item in $matches) {
        $path = ''
        if ($null -ne $item.PSObject.Properties['Path']) {
            $path = [string]$item.Path
        }

        $aliasTarget = ''
        if ([string]$item.CommandType -eq 'Alias') {
            $aliasTarget = [string]$item.Definition
        }

        $fields = @(
            [string]$item.CommandType,
            [string]$item.Name,
            [string]$item.Source,
            $path,
            $aliasTarget
        )

        $record = $fields -join [char]31
        $records += [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($record))
    }

    & $whichWhyExecutable '__powershell' $command ([string]$PSVersionTable.PSVersion) ([string]$PSVersionTable.PSEdition) @records
}
`, escapedExecutable)
}
