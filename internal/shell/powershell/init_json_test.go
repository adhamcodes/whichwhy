package powershell

import (
	"strings"
	"testing"
)

func TestInitScriptRoutesJSONThroughLivePowerShellEvidence(t *testing.T) {
	script := InitScript(`C:\Tools\whichwhy.exe`)

	for _, want := range []string{
		"$jsonOutput",
		"--json",
		"__powershell-json",
		"$inspectionArgumentCountOK",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("InitScript() missing %q\nscript:\n%s", want, script)
		}
	}
}
