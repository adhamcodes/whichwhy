//go:build !unix && !windows

package resolver

import (
	"fmt"
	"runtime"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func findCandidates(string, []processpath.Entry, string) (candidateEvidence, error) {
	return candidateEvidence{}, fmt.Errorf("process candidate eligibility is unsupported on %s", runtime.GOOS)
}
