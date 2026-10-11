package runtime_test

import (
	"context"
	"testing"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/route/prefix"
)

// This is admission/refusal coverage only. Empty handles never substitute for
// a successful qualified Job, registration, durable ACK or accepting Publisher.
func TestPublisherRefusesAbsentOrDetachedAuthority(t *testing.T) {
	for _, config := range []publicationruntime.Config{
		{},
		{Operation: &executionruntime.Operation{}, Binding: &instance.Binding{}, Source: &prefix.Prefix{}, Introduction: &prefix.Prefix{}, Deadline: time.Now().Add(time.Hour).UTC().Truncate(time.Second)},
	} {
		if owner, err := publicationruntime.New(context.Background(), config); err == nil || owner != nil {
			t.Fatal("detached handles created live Publisher", err)
		}
	}
	var owner *publicationruntime.Publisher
	if _, err := owner.Link(context.Background()); err == nil {
		t.Fatal("absent Publisher returned Link")
	}
	if _, err := owner.Publish(context.Background()); err == nil {
		t.Fatal("absent Publisher published")
	}
	if _, err := owner.Refresh(context.Background()); err == nil {
		t.Fatal("absent Publisher refreshed")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Withdraw(context.Background()); err != nil {
		t.Fatal("absent Publisher withdrawal must be idempotent", err)
	}
}
