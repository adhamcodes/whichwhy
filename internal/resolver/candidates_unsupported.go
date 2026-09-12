//go:build !unix && !windows

package resolver

import (
	"fmt"
	"runtime"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func findCandidates(string, []processpath.Entry, string) ([]Candidate, error) {
	return nil, fmt.Errorf("process candidate eligibility is unsupported on %s", runtime.GOOS)
}
