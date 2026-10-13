package connection

import (
	"context"
	"testing"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

func TestMissingOriginalReaderCannotConstructLogicalBinding(t *testing.T) {
	for _, operation := range []*executionruntime.Operation{nil, {}} {
		source := &InitiatorSource{config: InitiatorSourceConfig{Operation: operation}}
		binding, _, err := prepareInitiator(context.Background(), source, publication.Proof{}, [32]byte{}, capsule.Request{}, time.Now().Add(time.Minute))
		if err == nil || binding != nil {
			t.Fatal("missing original Reader supplied logical context")
		}
	}
	if owner, err := AuthenticateInitiator(context.Background(), nil, nil); err == nil || owner != nil {
		t.Fatal("missing Reader/JOIN supplied authenticated initial state")
	}
}
