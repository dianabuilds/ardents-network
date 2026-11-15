package systemd

import "testing"

func TestManagerReplyVocabularyKeepsFixedUnitsSeparate(t *testing.T) {
	for _, c := range []struct {
		reply  managerUnitReply
		object string
	}{
		{sourceUnitReply, "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"},
		{forwardingUnitReply, "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice"},
		{nodeSlotsReply, ""},
		{descriptorUnitReply, "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice"},
		{descriptorControlReply, "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice"},
		{joinUnitReply, "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"},
		{joinControlReply, "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"},
		{introductionUnitReply, "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice"},
		{introductionControlReply, "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice"},
	} {
		if !c.reply.valid() || c.reply.object() != c.object {
			t.Fatalf("reply %d maps to %q, want %q", c.reply, c.reply.object(), c.object)
		}
	}
	if managerUnitReply(255).valid() || managerUnitReply(255).object() != "" {
		t.Fatal("unknown reply vocabulary admitted")
	}
}
