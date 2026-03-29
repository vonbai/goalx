package cli

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
)

type structuredMutatorCounter struct {
	Value int `json:"value"`
}

func TestMutateStructuredFileBootstrapsMissingFile(t *testing.T) {
	path := t.TempDir() + "/counter.json"

	err := MutateStructuredFile(
		path,
		func(data []byte) (*structuredMutatorCounter, error) {
			var state structuredMutatorCounter
			if err := json.Unmarshal(data, &state); err != nil {
				return nil, err
			}
			return &state, nil
		},
		func() *structuredMutatorCounter {
			return &structuredMutatorCounter{}
		},
		func(state *structuredMutatorCounter) error {
			state.Value = 7
			return nil
		},
		func(state *structuredMutatorCounter) ([]byte, error) {
			return json.Marshal(state)
		},
	)
	if err != nil {
		t.Fatalf("MutateStructuredFile: %v", err)
	}

	state, err := loadStructuredMutatorCounter(path)
	if err != nil {
		t.Fatalf("loadStructuredMutatorCounter: %v", err)
	}
	if state == nil || state.Value != 7 {
		t.Fatalf("state = %#v, want value=7", state)
	}
}

func TestMutateStructuredFileSerializesConcurrentUpdates(t *testing.T) {
	path := t.TempDir() + "/counter.json"

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := MutateStructuredFile(
				path,
				func(data []byte) (*structuredMutatorCounter, error) {
					var state structuredMutatorCounter
					if err := json.Unmarshal(data, &state); err != nil {
						return nil, err
					}
					return &state, nil
				},
				func() *structuredMutatorCounter {
					return &structuredMutatorCounter{}
				},
				func(state *structuredMutatorCounter) error {
					state.Value++
					return nil
				},
				func(state *structuredMutatorCounter) ([]byte, error) {
					return json.Marshal(state)
				},
			); err != nil {
				t.Errorf("MutateStructuredFile: %v", err)
			}
		}()
	}
	wg.Wait()

	state, err := loadStructuredMutatorCounter(path)
	if err != nil {
		t.Fatalf("loadStructuredMutatorCounter: %v", err)
	}
	if state == nil {
		t.Fatal("state missing")
	}
	if state.Value != 8 {
		t.Fatalf("counter value = %d, want 8", state.Value)
	}
}

func loadStructuredMutatorCounter(path string) (*structuredMutatorCounter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state structuredMutatorCounter
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
