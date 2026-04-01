package cli

import (
	"strings"
	"testing"
)

func TestLaunchUsageHighlightsSelectionDefaults(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"start", "init"} {
		usage := launchUsage(command)
		if !strings.Contains(usage, "selection uses detected candidate pools by default") {
			t.Fatalf("%s usage missing selection-default note:\n%s", command, usage)
		}
		if !strings.Contains(usage, "--worker ENGINE/MODEL") {
			t.Fatalf("%s usage missing worker override:\n%s", command, usage)
		}
		if !strings.Contains(usage, "--readonly") {
			t.Fatalf("%s usage missing readonly flag:\n%s", command, usage)
		}
		if strings.Contains(usage, "--guided") {
			t.Fatalf("%s usage should omit removed guided flag:\n%s", command, usage)
		}
		for _, unwanted := range []string{"--research", "--develop", "--research-role", "--develop-role", "--research-effort", "--develop-effort", "--preset", "--route-role", "--route-profile"} {
			if strings.Contains(usage, unwanted) {
				t.Fatalf("%s usage should omit removed legacy flag %s:\n%s", command, unwanted, usage)
			}
		}
	}
}
