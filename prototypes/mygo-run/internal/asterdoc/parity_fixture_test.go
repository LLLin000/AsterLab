package asterdoc

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSharedParityFixture consumes the same sample file as the Rust test
// `asterdoc_parity_samples`, so a validator drift shows up on both sides from
// one source of truth.
func TestSharedParityFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/asterdoc_parity_samples.json")
	if err != nil {
		t.Fatalf("read shared parity fixture: %v", err)
	}
	var samples []struct {
		Name   string `json:"name"`
		Valid  bool   `json:"valid"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(raw, &samples); err != nil {
		t.Fatalf("parse shared parity fixture: %v", err)
	}
	if len(samples) < 8 {
		t.Fatalf("fixture has %d samples, want at least 8", len(samples))
	}
	for _, sample := range samples {
		sample := sample
		t.Run(sample.Name, func(t *testing.T) {
			_, err := Parse(sample.Source)
			if (err == nil) != sample.Valid {
				t.Fatalf("valid=%v but err=%v", sample.Valid, err)
			}
		})
	}
}

// The Rust validator rejects a manifest rule it does not implement; Go must do
// the same at startup, not on a user's document.
func TestManifestRuleMustBeRegisteredInGo(t *testing.T) {
	module := specModule{Descriptor: Descriptor{
		Kind: "probe", Schema: 1, SDK: "1.0.0", Category: "workflow",
		Rules: []string{"not-a-real-rule"},
	}}
	err := validateSpecModule(module, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "unregistered rule") {
		t.Fatalf("validateSpecModule() = %v", err)
	}
}
