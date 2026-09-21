package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

type externalResolver func(string) (resolver.Result, error)

// Run executes the command-line interface and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	// Only the private wire shape can enter evidence decoding. Public inspect
	// operands cannot reach this route; malformed private requests fail closed.
	if len(args) >= 4 && (args[0] == "__powershell" || args[0] == "__powershell-json") {
		return runPowerShellEvidence(args[1:], stdout, stderr, args[0] == "__powershell-json")
	}
	return run(args, stdout, stderr, version, resolver.ResolveExternal)
}

func run(args []string, stdout, stderr io.Writer, version string, resolve externalResolver) int {
	request, err := parseInvocation(args)
	if err != nil {
		printError(stderr, err.Error())
		fmt.Fprintln(stderr, "\nTry:\n  whichwhy --help")
		return 2
	}
	switch request.operation {
	case showHelp:
		printHelp(stdout)
		return 0
	case showVersion:
		newHuman(stdout).line("", "whichwhy "+version, "")
		return 0
	case initPowerShell:
		executable, err := os.Executable()
		if err != nil {
			printError(stderr, "locate executable: "+err.Error())
			return 2
		}
		fmt.Fprint(stdout, ps.InitScript(executable))
		return 0
	case pathOperation:
		if request.json {
			return runPathJSON(stdout, stderr, os.LookupEnv, pathdiag.Inspect)
		}
		return runPath(stdout, stderr, os.LookupEnv, pathdiag.Inspect)
	case doctorOperation:
		if request.json {
			return runDoctorJSON(stdout, stderr, version, os.Executable, resolve)
		}
		return runDoctor(stdout, stderr, version, os.Executable, resolve)
	}
	result, err := resolve(request.command)
	if err != nil {
		printError(stderr, err.Error())
		return 2
	}
	report := resolution.ProcessExternal(result)
	if request.json {
		return printCommandJSON(stdout, stderr, report)
	}
	return printCommandReport(stdout, report)
}
