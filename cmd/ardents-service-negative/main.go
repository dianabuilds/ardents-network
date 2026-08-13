package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type receipt struct {
	Schema    string          `json:"schema"`
	Negatives map[string]bool `json:"negatives"`
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output io.Writer) error {
	fixture, err := newFixture()
	if err != nil {
		return err
	}
	result := receipt{Schema: "ardents-h3-service-negative-v1", Negatives: fixture.run(context.Background())}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return err
	}
	for _, passed := range result.Negatives {
		if !passed {
			return fmt.Errorf("one or more Stage 3 negative cases failed")
		}
	}
	return nil
}
