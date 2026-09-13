package cli

import (
	"fmt"
	"strings"
)

type operation uint8

const (
	inspectCommand operation = iota
	showHelp
	showVersion
	pathOperation
	doctorOperation
	initPowerShell
)

type invocation struct {
	operation operation
	command   string
	literal   bool
	json      bool
}

// parseInvocation handles public syntax only. An inspect operand is never
// parsed again as either product syntax or a private evidence request.
func parseInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{operation: showHelp}, nil
	}
	if args[0] == "inspect" {
		if len(args) < 2 || len(args) > 3 || args[1] == "" || (len(args) == 3 && args[2] != "--json") {
			return invocation{}, fmt.Errorf("inspect requires one nonempty command name and optional trailing --json")
		}
		return invocation{command: args[1], literal: true, json: len(args) == 3}, nil
	}
	i := invocation{command: args[0], json: len(args) == 2 && args[1] == "--json"}
	switch args[0] {
	case "-h", "--help", "help":
		return invocation{operation: showHelp}, nil
	case "-v", "--version", "version":
		return invocation{operation: showVersion}, nil
	case "path":
		i.operation = pathOperation
	case "doctor":
		i.operation = doctorOperation
	case "init":
		if len(args) == 2 && strings.EqualFold(args[1], "powershell") {
			return invocation{operation: initPowerShell}, nil
		}
		return invocation{}, fmt.Errorf("init requires powershell")
	case "__powershell", "__powershell-json":
		return invocation{}, fmt.Errorf("invalid internal request; use inspect to inspect this command name")
	}
	if args[0] == "" || (len(args) != 1 && !i.json) {
		return invocation{}, fmt.Errorf("command inspection accepts one nonempty command name and optional --json")
	}
	return i, nil
}
