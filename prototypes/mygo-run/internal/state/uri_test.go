package state

import "testing"

func TestParseAsterLabURI(t *testing.T) {
	openIntent, err := ParseAsterLabURI("asterlab://open?file=2026%2FCCK8.asterml&step=step-2")
	if err != nil {
		t.Fatalf("ParseAsterLabURI open: %v", err)
	}
	if openIntent.Kind != "open" || openIntent.FilePath != "2026/CCK8.asterml" || openIntent.StepID != "step-2" {
		t.Fatalf("unexpected open intent: %+v", openIntent)
	}

	pantryIntent, err := ParseAsterLabURI("asterlab://pantry?code=RPMI-1640")
	if err != nil {
		t.Fatalf("ParseAsterLabURI pantry: %v", err)
	}
	if pantryIntent.Kind != "pantry" || pantryIntent.PantryCode != "RPMI-1640" {
		t.Fatalf("unexpected pantry intent: %+v", pantryIntent)
	}

	runIntent, err := ParseAsterLabURI("asterlab://run/run-01/well/B12")
	if err != nil {
		t.Fatalf("ParseAsterLabURI run: %v", err)
	}
	if runIntent.Kind != "run" || runIntent.RunID != "run-01" || runIntent.Well != "B12" {
		t.Fatalf("unexpected run intent: %+v", runIntent)
	}

	if _, err := ParseAsterLabURI("https://example.com/bad"); err == nil {
		t.Fatalf("expected error for non-asterlab scheme")
	}
}
