//go:build linux || darwin

package introduction

import (
	"github.com/dianabuilds/ardents-network/internal/domain/route/ardp"
	"testing"
	"time"
)

func TestIndependentPlainStreamWithGenuineRegistryBeforeRead(t *testing.T) {
	history, _, _, _ := newHistoryFixture(t)
	r, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := &unadmittedRegistrationConn{}
	hello := ardp.Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{6}, StateDigest: [32]byte{2}, ProfileDigest: [32]byte{3}, RecipientNodeID: [32]byte{4}, RecipientDutyGeneration: 1, ChannelNonce: [32]byte{5}, Purpose: ardp.PurposeIntroduction, Deadline: time.Now().UTC().Truncate(time.Second).Add(time.Minute)}
	if _, err := ardp.EncodeHello(hello); err != nil {
		t.Fatal(err)
	}
	err = r.ServeRegistration(t.Context(), c, &Capacity{registry: r}, RegistrationChannel{Hello: hello, Record: func(error) {}, InterruptIO: c.interrupt})
	if err == nil || c.reads != 0 || c.writes != 0 {
		t.Fatal("plain stream reached operation despite absent exporter", err, c.reads, c.writes)
	}
}
