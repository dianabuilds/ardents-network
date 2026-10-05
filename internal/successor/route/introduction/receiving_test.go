package introduction

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These incomplete local owners can only refuse. They supply no retained root,
// reserved position, authenticated channel, token, successful claim or ACK.
type unadmittedRegistrationConn struct {
	net.Conn
	reads, writes, closes int
}

func (c *unadmittedRegistrationConn) Read([]byte) (int, error) {
	c.reads++
	return 0, errors.New("unadmitted stream read")
}

func (c *unadmittedRegistrationConn) Write([]byte) (int, error) {
	c.writes++
	return 0, errors.New("unadmitted stream write")
}

func (c *unadmittedRegistrationConn) Close() error { c.closes++; return nil }

func TestReceivingRegistrationCanceledCallerCannotReachChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	registry := &Registry{pending: 1}
	conn := &unadmittedRegistrationConn{}
	recorded := 0
	capacity := &Capacity{registry: registry}
	err := registry.ServeRegistration(ctx, conn, capacity, RegistrationChannel{
		Hello: ardp.Hello{Purpose: ardp.PurposeIntroduction}, Record: func(error) { recorded++ },
	})
	if !errors.Is(err, context.Canceled) || conn.reads != 0 || conn.writes != 0 || conn.closes != 0 || recorded != 0 {
		t.Fatal("canceled caller reached channel or lost its original refusal", err, conn, recorded)
	}
	if registry.pending != 1 {
		t.Fatal("pre-handoff refusal returned caller-owned capacity")
	}
	capacity.Release()
	capacity.Release()
	if registry.pending != 0 {
		t.Fatal("original caller capacity leaked or returned twice")
	}
}

func TestReceivingRegistrationPlainStreamCannotReachOperation(t *testing.T) {
	registry := &Registry{}
	conn := &unadmittedRegistrationConn{}
	hello := ardp.Hello{
		NetworkID: [32]byte{1}, StateGeneration: [32]byte{6}, StateDigest: [32]byte{2},
		ProfileDigest: [32]byte{3}, RecipientNodeID: [32]byte{4}, RecipientDutyGeneration: 1,
		ChannelNonce: [32]byte{5}, Purpose: ardp.PurposeIntroduction,
		Deadline: time.Now().UTC().Truncate(time.Second).Add(time.Minute),
	}
	if _, err := ardp.EncodeHello(hello); err != nil {
		t.Fatal("invalid HELLO cannot test missing negotiated exporter", err)
	}
	recorded := 0
	err := registry.ServeRegistration(t.Context(), conn, &Capacity{registry: registry}, RegistrationChannel{
		Hello: hello, Record: func(error) { recorded++ },
	})
	if err == nil || conn.reads != 0 || conn.writes != 0 || conn.closes != 0 || recorded != 0 {
		t.Fatal("plain stream reached operation or transferred physical ownership", err, conn, recorded)
	}
}
