//go:build windows

package pathdiag

import "strings"

func normalizeForComparison(value string) string {
	return strings.ToLower(value)
}
