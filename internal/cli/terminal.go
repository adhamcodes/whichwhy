package cli

import (
	"io"
	"os"
	"strings"
)

// terminalOptions is presentation-only. A zero value is readable, unstyled,
// unwrapped text. Detection never changes console modes or caller state.
type terminalOptions struct {
	color, unicode bool
	width          int
}

type terminalFacts struct {
	tty, ansi, utf8 bool
	width           int
}

func terminalPolicy(f terminalFacts, getenv func(string) string) terminalOptions {
	if !f.tty || getenv("TERM") == "dumb" {
		return terminalOptions{}
	}
	return terminalOptions{color: f.ansi && getenv("NO_COLOR") == "", unicode: f.utf8, width: f.width}
}

func detectTerminal(w io.Writer) terminalOptions {
	f, ok := w.(*os.File)
	if !ok {
		return terminalOptions{}
	}
	return terminalPolicy(probeTerminal(f), os.Getenv)
}

func utf8Locale(getenv func(string) string) bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if value := getenv(key); value != "" {
			value = strings.ToUpper(value)
			return strings.Contains(value, "UTF-8") || strings.Contains(value, "UTF8")
		}
	}
	return false
}
