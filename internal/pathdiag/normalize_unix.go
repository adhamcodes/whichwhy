//go:build !windows

package pathdiag

func normalizeForComparison(value string) string {
	return value
}
