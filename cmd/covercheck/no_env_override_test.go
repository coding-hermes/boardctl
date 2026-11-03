package main

import (
	"testing"
)

// TestCovercheckFloorsComeOnlyFromFlags pins the rejection of an undocumented
// environment override (BT-85): the per-package floor must be settable ONLY by
// --min-package, never by an env var, because an env-provided floor is a hidden
// bypass surface on a coverage gate — any shell or CI job exporting the name
// would change the verdict without a reviewable diff, and the README states the
// floors live in the Makefile alone.
func TestCovercheckFloorsComeOnlyFromFlags(t *testing.T) {
	// A floor that WOULD fail this profile if it were honoured (board is 50%
	// against a 95% floor): the verdict must stay 0 because the name is not a
	// supported input.
	t.Setenv("MIN_PACKAGE_COVERAGE", "95")
	t.Setenv("MIN_PACKAGE", "95")
	t.Setenv("MIN_PACKAGE_FLOOR", "95")
	if code := run([]string{"--profile", writeProfile(t, lowPackageProfile)}); code != 1 {
		t.Fatalf("run exit = %d, want 1 (default 60%% floor must still apply)", code)
	}
	if code := run([]string{"--profile", writeProfile(t, passingProfile)}); code != 0 {
		t.Fatalf("run exit = %d, want 0 (env must not raise the floor)", code)
	}
}
