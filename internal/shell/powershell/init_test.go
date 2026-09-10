package powershell

import (
	"strings"
	"testing"
)

func TestInitScriptBuildsMinimalPowerShellBridge(t *testing.T) {
	script := InitScript(`C:\Tools\whichwhy.exe`)

	for _, want := range []string{
		"function global:whichwhy",
		"Microsoft.PowerShell.Core\\Get-Command",
		"-All -ListImported",
		"__powershell",
		"C:\\Tools\\whichwhy.exe",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("InitScript() missing %q\nscript:\n%s", want, script)
		}
	}

	if strings.ContainsAny(script, "\r\n") {
		t.Fatalf("InitScript() must be one physical line for direct Invoke-Expression use: %q", script)
	}
	if strings.Contains(script, "ConvertTo-Json") {
		t.Fatal("InitScript() should not require ConvertTo-Json or module auto-loading")
	}
}

func TestInitScriptEscapesSingleQuoteInExecutablePath(t *testing.T) {
	script := InitScript(`C:\Users\O'Brien\whichwhy.exe`)
	if !strings.Contains(script, `O''Brien`) {
		t.Fatalf("InitScript() did not escape single quote: %s", script)
	}
}
