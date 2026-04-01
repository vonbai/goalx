package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestRenderSubagentProtocolIncludesCompilerComposedDoctrineLine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/inbox.jsonl",
		SessionCursorPath: "/tmp/cursor.json",
		Composition: ProtocolComposition{
			Enabled:           true,
			Philosophy:        []string{"durable_state_first", "localized_override_not_reset"},
			BehaviorContract:  []string{"automatic_follow_through", "evidence_backed_completion"},
			RequiredRoles:     []string{"builder", "critic"},
			RequiredGates:     []string{"critic_pass"},
			SelectedPriorRefs: []string{"prior/operator-cockpit"},
		},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"Compiler-composed doctrine for this run:",
		"philosophy=`durable_state_first`, `localized_override_not_reset`",
		"contract=`automatic_follow_through`, `evidence_backed_completion`",
		"roles=`builder`, `critic`",
		"gates=`critic_pass`",
		"priors=`prior/operator-cockpit`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
		}
	}
}
