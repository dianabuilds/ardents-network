package servicesmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"
)

type negativeReceipt struct {
	Schema     string            `json:"schema"`
	Negatives  map[string]bool   `json:"negatives"`
	Mechanisms map[string]string `json:"mechanisms"`
	Operations map[string]bool   `json:"operations"`
	Classes    map[string]string `json:"classes"`
	Counts     map[string]uint32 `json:"counts"`
}

func (observer dockerObserver) negativeReceipt(ctx context.Context) (negativeReceipt, error) {
	raw, err := observer.compose(ctx, time.Minute, "--profile", "negative", "run", "--no-deps", "--rm", "negative-suite")
	if err != nil {
		return negativeReceipt{}, err
	}
	var value negativeReceipt
	if json.Unmarshal(bytes.TrimSpace(raw), &value) != nil || !validNegativeReceiptShape(value) {
		return negativeReceipt{}, errors.New("stage 3 negative-suite receipt is malformed")
	}
	for _, passed := range value.Negatives {
		if !passed {
			return negativeReceipt{}, errors.New("stage 3 negative suite observed an accepted forbidden case")
		}
	}
	for _, passed := range value.Operations {
		if !passed {
			return negativeReceipt{}, errors.New("stage 3 negative suite failed a stream observation")
		}
	}
	return value, nil
}

func validNegativeReceiptShape(value negativeReceipt) bool {
	return value.Schema == "ardents-h3-service-negative-v1" && len(value.Negatives) == 24 &&
		len(value.Mechanisms) == 24 && len(value.Operations) == 4
}
