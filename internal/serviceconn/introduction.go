package serviceconn

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
)

func requestIntroductionAcknowledgement(ctx context.Context, socket string, credential Credential,
	broker [32]byte) ([]byte, error) {
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	body := make([]byte, acknowledgementBodySize)
	copy(body[:4], "ASIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	copy(body[85:117], broker[:])
	if _, err := rand.Read(body[117:149]); err != nil {
		return nil, err
	}
	if err := writeAll(connection, body); err != nil {
		return nil, err
	}
	raw := make([]byte, acknowledgementSize)
	copy(raw, body)
	_, err = io.ReadFull(connection, raw[acknowledgementBodySize:])
	return raw, err
}
