package slowtest

import "testing"

func TestEnabledDefaultsFalse(t *testing.T) {
	t.Setenv(EnvVar, "")
	if Enabled() {
		t.Fatal("Enabled() = true, want false by default")
	}
}

func TestEnabledAcceptsTruthyValues(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv(EnvVar, value)
		if !Enabled() {
			t.Fatalf("Enabled() = false for %q, want true", value)
		}
	}
}
