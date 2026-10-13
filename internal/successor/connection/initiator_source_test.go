package connection

import "testing"

func TestDetachedInitiatorSourceRefusesWithoutOwnerEffects(t *testing.T) {
	for _, source := range []*InitiatorSource{nil, {}} {
		if owner, err := source.AuthenticateInitial(); err == nil || owner != nil {
			t.Fatal("detached Source supplied authenticated state")
		}
	}
	if source, err := NewInitiatorSource(InitiatorSourceConfig{}); err == nil || source != nil {
		t.Fatal("missing original owners supplied a Source")
	}
	original := &InitiatorSource{}
	original.self = original
	copied := InitiatorSource{self: original}
	if err := copied.Close(); err == nil {
		t.Fatal("copied Source could retire the original")
	}
	if owner, err := copied.AuthenticateInitial(); err == nil || owner != nil {
		t.Fatal("copied Source could authenticate an Attachment")
	}
}
