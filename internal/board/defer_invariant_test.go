package board

import (
	"os"
	"strings"
	"testing"
)

func TestDeferFieldsRequireDeferred(t *testing.T) {
	for _, tc := range []struct {
		name           string
		reason, review *string
	}{
		{"reason", strPtr("waiting"), nil},
		{"review", nil, strPtr("2027-01-01")},
		{"both", strPtr("waiting"), strPtr("2027-01-01")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, explicitFalse := range []bool{false, true} {
				b := seedDeferEvidenceBoard(t)
				before, err := os.ReadFile(b.TasksPath())
				if err != nil {
					t.Fatal(err)
				}
				var deferred *bool
				if explicitFalse {
					deferred = boolPtr(false)
				}
				_, err = b.Create(TaskRowSpec{ID: "DE-NEW", Title: "new", Deferred: deferred, DeferReason: tc.reason, DeferReviewBy: tc.review})
				if err == nil || !strings.Contains(err.Error(), "--deferred") {
					t.Fatalf("create refusal: %v", err)
				}
				_, err = b.UpdateTask("DE-1", UpdateSpec{Deferred: deferred, DeferReason: tc.reason, DeferReviewBy: tc.review})
				if err == nil || !strings.Contains(err.Error(), "--deferred") {
					t.Fatalf("update refusal: %v", err)
				}
				after, err := os.ReadFile(b.TasksPath())
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(after) {
					t.Fatal("refusal changed board")
				}
			}
		})
	}
}

func TestDeferredUpdatesAndUnDefer(t *testing.T) {
	b := seedDeferEvidenceBoard(t)
	if _, err := b.UpdateTask("DE-2", UpdateSpec{DeferReviewBy: strPtr("2027-01-01")}); err != nil {
		t.Fatal(err)
	}
	before := deferEvidenceRow(t, b, "DE-2")
	if _, err := b.UpdateTask("DE-2", UpdateSpec{Deferred: boolPtr(false), DeferReason: strPtr("contradiction")}); err == nil || !strings.Contains(err.Error(), "--deferred") {
		t.Fatalf("contradictory un-defer refusal: %v", err)
	}
	if got := deferEvidenceRow(t, b, "DE-2"); got != before {
		t.Fatalf("refusal changed row: %s", got)
	}
	if _, err := b.UpdateTask("DE-2", UpdateSpec{Deferred: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if got := deferEvidenceRow(t, b, "DE-2"); strings.Contains(got, "defer") {
		t.Fatalf("un-defer left audit fields: %s", got)
	}
}

func TestValidateNonDeferredDeferFieldsWarn(t *testing.T) {
	for _, tc := range []struct{ name, fields string }{
		{"reason", `"defer_reason":"waiting"`},
		{"review", `"defer_review_by":"2027-01-01"`},
		{"false", `"deferred":false,"defer_reason":"waiting"`},
		{"null", `"deferred":null,"defer_review_by":null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := seedDeferEvidenceBoard(t)
			f, err := os.OpenFile(b.TasksPath(), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.WriteString(`{"id":"DE-LEGACY","title":"legacy","status":"pending","priority":"P2",` + tc.fields + "}\n")
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			rep, err := b.Validate()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, finding := range rep.Findings {
				if finding.Level == "warn" && strings.Contains(finding.Msg, "DE-LEGACY") && strings.Contains(finding.Msg, "non-deferred") {
					found = true
				}
			}
			if !found || rep.HasErrors() {
				t.Fatalf("expected warning only: %v", rep.Findings)
			}
		})
	}
}
