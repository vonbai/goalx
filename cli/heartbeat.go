package cli

import (
	"fmt"
	"time"
)

func normalizeSidecarInterval(checkInterval time.Duration) (int, string) {
	checkSec := int(checkInterval.Seconds())
	if checkSec < 30 {
		return 300, fmt.Sprintf("⚠ check_interval %ds is below 30s minimum, using default 300s\n", checkSec)
	}
	return checkSec, ""
}
