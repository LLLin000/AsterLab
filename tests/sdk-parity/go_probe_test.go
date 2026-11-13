package asterdoc

import (
	"encoding/json"
	"os"
	"testing"
)

type parityCase struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func TestSDKParityProbe(t *testing.T) {
	file, err := os.Open(os.Getenv("SDK_PARITY_FIXTURES"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var cases []parityCase
	if err := json.NewDecoder(file).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(sample.Source)
			result := struct {
				Name  string `json:"name"`
				Valid bool   `json:"valid"`
				Error string `json:"error,omitempty"`
			}{Name: sample.Name, Valid: err == nil}
			if err != nil {
				result.Error = err.Error()
			}
			if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
				t.Fatal(err)
			}
		})
	}
}
