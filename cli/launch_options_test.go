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
		for _, unwanted := range []string{"--preset", "--route-role", "--route-profile"} {
			if strings.Contains(usage, unwanted) {
				t.Fatalf("%s usage should omit removed legacy flag %s:\n%s", command, unwanted, usage)
			}
		}
	}
}
