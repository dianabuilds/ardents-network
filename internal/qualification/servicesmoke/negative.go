package servicesmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

func (observer dockerObserver) negativeReceipt(ctx context.Context) (map[string]bool, error) {
	raw, err := observer.compose(ctx, time.Minute, "--profile", "negative", "run", "--no-deps", "--rm", "negative-suite")
	if err != nil {
		return nil, err
	}
	var value struct {
		Schema    string          `json:"schema"`
		Negatives map[string]bool `json:"negatives"`
	}
	if json.Unmarshal(bytes.TrimSpace(raw), &value) != nil || value.Schema != "ardents-h3-service-negative-v1" ||
		len(value.Negatives) != 24 {
		return nil, errors.New("stage 3 negative-suite receipt is malformed")
	}
	for _, passed := range value.Negatives {
		if !passed {
			return nil, errors.New("stage 3 negative suite observed an accepted forbidden case")
		}
	}
	return value.Negatives, nil
}
