package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func auditLogPath(runDir string) string {
	return filepath.Join(runDir, "sidecar.log")
}

func appendAuditLog(runDir, format string, args ...any) {
	if stringsTrimmed := filepath.Clean(runDir); stringsTrimmed == "." || runDir == "" {
		return
	}
	path := auditLogPath(runDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
