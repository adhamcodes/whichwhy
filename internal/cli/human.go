package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const tagline = "Which command gets picked — and why?"

const (
	accent = "33" // Warm ANSI amber; works without assuming 256/truecolor support.
	good   = "32"
	warn   = "33"
	bad    = "31"
	quiet  = "2"
)

type humanRenderer struct {
	out io.Writer
	terminalOptions
}

func newHuman(out io.Writer) humanRenderer {
	return humanRenderer{out: out, terminalOptions: detectTerminal(out)}
}

// safeText is the only entry to terminal text. Escape C0/C1/DEL, Unicode format
// controls (including bidi overrides), line/paragraph separators and invalid
// UTF-8 bytes. Data can neither inject lines nor supply terminal escape codes.
// Ordinary Unicode and literal backslashes remain unchanged. JSON bypasses this.
func safeText(text string) string {
	var b strings.Builder
	for len(text) > 0 {
		r, n := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&b, "\\x%02x", text[0])
		} else if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			switch r {
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if r < 256 {
					fmt.Fprintf(&b, "\\x%02x", r)
				} else if r > 0xffff {
					fmt.Fprintf(&b, "\\U%08x", r)
				} else {
					fmt.Fprintf(&b, "\\u%04x", r)
				}
			}
		} else {
			b.WriteRune(r)
		}
		text = text[n:]
	}
	return b.String()
}

func (h humanRenderer) styled(s, style string) string {
	if h.color && style != "" {
		return "\x1b[" + style + "m" + s + "\x1b[0m"
	}
	return s
}

func (h humanRenderer) blank() { fmt.Fprintln(h.out) }

func (h humanRenderer) line(prefix, text, style string) {
	fmt.Fprintln(h.out, prefix+h.styled(safeText(text), style))
}

func (h humanRenderer) heading(title, status, style string) {
	separator := " - "
	if h.unicode {
		separator = " · "
	}
	h.line("", "WhichWhy"+separator+title, accent)
	if status != "" {
		h.line("", "[ "+status+" ]", style)
	}
}

func (h humanRenderer) section(title string) {
	h.blank()
	h.line("  ", title, accent)
}

func (h humanRenderer) itemPrefix(last bool) string {
	if h.unicode {
		if last {
			return "  └─ "
		}
		return "  ├─ "
	}
	return "  - "
}

// Data stays contiguous even when it contains spaces, for copy/paste and exact
// identity correlation. A narrow terminal can soft-wrap it without data loss.
func (h humanRenderer) dataItem(text string, last bool, style string) {
	h.line(h.itemPrefix(last), text, style)
}

// Keep identities/path operands contiguous for copying. The terminal soft-wraps
// long tokens naturally. Prose wraps only at spaces, never drops characters, and
// uses hanging indentation. Redirected output has no artificial width limit.
func (h humanRenderer) item(text string, last bool, style string) {
	prefix := h.itemPrefix(last)
	text = safeText(text)
	indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
	available := h.width - len(indent) - 1 // Avoid the terminal's last column.
	for h.width > 0 && available >= 16 && displayWidth(text) > available {
		cut, columns := -1, 0
		for i, r := range text {
			columns += runeWidth(r)
			if columns > available {
				break
			}
			if r == ' ' {
				cut = i
			}
		}
		if cut <= 0 {
			break // Unbroken identity: preserve it, allow terminal soft wrapping.
		}
		fmt.Fprintln(h.out, prefix+h.styled(text[:cut], style))
		text, prefix = text[cut+1:], indent
	}
	fmt.Fprintln(h.out, prefix+h.styled(text, style))
}

// Conservative cell estimate: non-ASCII occupies at most two columns on the
// supported terminals. Overestimating avoids broken panels without a width
// table/dependency or any text normalization. Combining marks occupy no cells.
func runeWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r > 127 {
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func (h humanRenderer) panel(lines []string) {
	width := 0
	// Borders are useful for a compact primary answer. If any identity is long
	// or non-ASCII, use the open layout instead of guessing glyph widths.
	boxed := h.unicode && h.width >= 40
	for _, line := range lines {
		s := safeText(line)
		width = max(width, len(s))
		boxed = boxed && len(s) == utf8.RuneCountInString(s)
	}
	boxed = boxed && width+7 < h.width
	if boxed {
		fmt.Fprintln(h.out, "  "+h.styled("╭"+strings.Repeat("─", width+2)+"╮", quiet))
	} else {
		h.blank()
	}
	for i, line := range lines {
		if boxed {
			s := safeText(line)
			fmt.Fprintln(h.out, "  "+h.styled("│", quiet)+" "+s+strings.Repeat(" ", width-len(s))+" "+h.styled("│", quiet))
		} else {
			if i == 0 {
				h.line("  "+h.styled(">", accent)+" ", line, "")
			} else {
				h.line("    ", line, quiet)
			}
		}
	}
	if boxed {
		fmt.Fprintln(h.out, "  "+h.styled("╰"+strings.Repeat("─", width+2)+"╯", quiet))
	}
}

// Errors stay on stderr, including machine-mode failures. No terminal styling
// is used here so the frozen machine-error stream contract remains unchanged.
func printError(out io.Writer, message string) {
	fmt.Fprintf(out, "whichwhy: %s\n", safeText(message))
}

func printHelp(out io.Writer) {
	h := newHuman(out)
	h.line("", "WhichWhy", accent)
	h.line("", tagline, "")
	for _, section := range []struct {
		title string
		lines []string
	}{
		{"Usage", []string{"whichwhy <command> [--json]", "whichwhy inspect <literal-command> [--json]"}},
		{"Commands", []string{"path             Diagnose PATH entries", "doctor           Check WhichWhy command discovery", "init powershell  Print the optional session bridge"}},
		{"Options", []string{"--json           Machine-readable inspection, path, or doctor report", "--help, -h       Show this help", "--version, -v    Show the build version"}},
		{"Examples", []string{"whichwhy python", "whichwhy inspect path", "whichwhy inspect 'name[1].cmd' --json", "whichwhy path", "whichwhy doctor"}},
	} {
		h.section(section.title)
		for _, line := range section.lines {
			h.line("    ", line, "")
		}
	}
	h.section("Scope")
	h.item("Standalone: external commands in PATH; your shell may choose differently.", false, quiet)
	h.item("PowerShell bridge: passive discovery in the current loaded session (experimental).", false, quiet)
	h.item("Use inspect for reserved or wildcard names. Quote names for your shell.", true, quiet)
}
