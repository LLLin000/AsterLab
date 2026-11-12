package asterdoc

import (
	"encoding/json"
	"os"
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
