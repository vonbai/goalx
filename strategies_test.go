package goalx

import (
	"strings"
	"testing"
)

func TestResolveStrategiesSupportsPerfectionistBuiltin(t *testing.T) {
	hints, err := ResolveStrategies([]string{"perfectionist"})
	if err != nil {
		t.Fatalf("ResolveStrategies(perfectionist): %v", err)
	}
	if len(hints) != 1 {
		t.Fatalf("len(hints) = %d, want 1", len(hints))
	}

	for _, want := range []string{
		"ironclad evidence",
		"code references",
		"fewer high-quality findings",
		"Re-read before commit",
		"Depth over breadth",
	} {
		if !strings.Contains(hints[0], want) {
			t.Fatalf("perfectionist hint missing %q: %q", want, hints[0])
		}
	}
}
