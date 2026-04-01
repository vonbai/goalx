package slowtest

import (
	"os"
	"strings"
	"testing"
)

const EnvVar = "GOALX_RUN_SLOW"

func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvVar))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func Require(t *testing.T, label string) {
	t.Helper()
	if Enabled() {
		return
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = "slow integration test"
	}
	t.Skipf("%s skipped by default; set %s=1 to run", label, EnvVar)
}
