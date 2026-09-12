package processpath

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProcessParsingRules(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		windows   bool
		want      []string
	}{
		{"windows-empty-path", "", true, []string{}},
		{"windows-empty-entries", `;"";.;`, true, []string{"", "", ".", ""}},
		{"windows-quoted-semicolon", `"C:\one;two";D:\last`, true, []string{`C:\one;two`, `D:\last`}},
		{"windows-relative", `bin;C:bin;\bin;\\host\share`, true, []string{`bin`, `C:bin`, `\bin`, `\\host\share`}},
		{"windows-unmatched", `C:\one;"two;three`, true, []string{`C:\one`, `two;three`}},
		{"windows-embedded-quotes", `a"b;c"d;""; x ;%PATH%;$HOME`, true, []string{`ab;cd`, "", " x ", "%PATH%", "$HOME"}},
		{"unix-empty-path", "", false, []string{}},
		{"unix-empty-relative", `:bin:.:`, false, []string{"", "bin", ".", ""}},
		{"unix-quotes-literal", `"/one:two":/three;four`, false, []string{`"/one`, `two"`, `/three;four`}},
		{"unix-whitespace-expansion", ` ~/bin :$HOME/bin:%PATH%`, false, []string{` ~/bin `, `$HOME/bin`, `%PATH%`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			separator := byte(':')
			if tc.windows {
				separator = ';'
			}
			p := parse(tc.raw, separator, tc.windows)
			assertEntries(t, p, tc.want, separator)
		})
	}
}

func assertEntries(t *testing.T, p Path, want []string, separator byte) {
	t.Helper()
	values, raw := []string{}, []string{}
	for i, entry := range p.Entries {
		values = append(values, entry.Value)
		raw = append(raw, entry.Raw)
		if entry.Index != i+1 {
			t.Fatalf("unstable index: %#v", entry)
		}
		effective := entry.Value
		if effective == "" {
			effective = "."
		}
		if entry.EffectiveValue() != effective {
			t.Fatalf("effective value: %#v", entry)
		}
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("Parse(%q) = %#v; want %#v", p.Raw, values, want)
	}
	if strings.Join(raw, string(separator)) != p.Raw {
		t.Fatalf("raw evidence lost: %#v", p)
	}
}

// Native filepath.SplitList is the compatibility reference, never a shell oracle.
// Enumerate short strings exhaustively to cover quote/separator interactions on
// every host, including malformed entries and embedded NUL observation operands.
func TestParseMatchesNativeSplitList(t *testing.T) {
	alphabet := []byte{'a', ':', ';', '"', '\\', ' ', 0}
	var visit func(string, int)
	visit = func(value string, remaining int) {
		assertEntries(t, Parse(value), filepath.SplitList(value), byte(os.PathListSeparator))
		if remaining > 0 {
			for _, c := range alphabet {
				visit(value+string(c), remaining-1)
			}
		}
	}
	visit("", 5)
}
