//go:build !windows && !linux && !darwin

package cli

import "os"

func probeTerminal(*os.File) terminalFacts { return terminalFacts{} }
