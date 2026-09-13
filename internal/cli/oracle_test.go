package cli

import (
	"os"
	"testing"
)

// Constrained local environments may lack a capability; supported release
// runners must never turn missing native evidence into a green skipped gate.
func unavailableOracle(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("WHICHWHY_REQUIRE_ORACLES") == "1" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}
