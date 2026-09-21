package cli

import (
	"io"

	"github.com/adhamcodes/whichwhy/internal/resolution"
)

func printDoctorResult(stdout io.Writer, report doctorReport) int {
	return newHuman(stdout).doctor(report)
}

func (h humanRenderer) doctor(r doctorReport) int {
	status, style := "healthy", good
	if r.Status != 0 {
		status, style = "attention", warn
	}
	h.heading("doctor", status, style)
	h.section("Command discovery")
	switch r.Discovery {
	case doctorDiscoveryCurrent:
		h.item("The process policy selects this running executable for 'whichwhy'.", true, good)
	case doctorDiscoveryDifferent:
		h.item("The process policy selects a different executable for 'whichwhy'.", false, warn)
		h.item("Invoking by name may reach a different installation. Review PATH order with whichwhy path and choose the intended installation.", true, "")
	case doctorDiscoveryMissing:
		h.item("No 'whichwhy' candidate was observed under the process policy.", false, warn)
		h.item("Invoking by name may fail. Review whichwhy path and add the intended build directory to PATH yourself if needed.", true, "")
	case doctorDiscoveryUncertain:
		h.item("Incomplete inspection prevents definitive process-policy discovery for 'whichwhy'.", false, warn)
		h.item("Unresolved attempts may conceal candidates. Review the observation failures and directory access, then retry.", true, "")
	}
	if r.Resolution.Selected != nil {
		h.line("    ", "Policy candidate: "+r.Resolution.Selected.Path, "")
	}
	if len(r.Resolution.Alternatives) > 0 {
		h.section("Other WhichWhy candidates")
		for i, c := range r.Resolution.Alternatives {
			h.dataItem(c.Path, i == len(r.Resolution.Alternatives)-1, "")
		}
		h.item("Multiple installations can make upgrades confusing. Review their PATH order and retain the installation you intend to use.", true, "")
	}
	if r.Discovery != doctorDiscoveryUncertain && r.Resolution.Inspection != nil && r.Resolution.Inspection.Completeness == resolution.InspectionIncomplete {
		h.item("Inspection is incomplete even though selection is definitive. Review the observation failures and directory access, then retry.", true, warn)
	}
	h.section("Installation")
	h.item("Version: "+r.Version+" / "+r.OS+"/"+r.Arch, false, quiet)
	h.dataItem("Running executable: "+r.RunningExecutable, true, "")
	h.claim(r.Resolution)
	h.section("Safe inspection")
	h.item("Doctor only inspected the running executable and command search results. It changed nothing.", true, quiet)
	return r.Status
}
