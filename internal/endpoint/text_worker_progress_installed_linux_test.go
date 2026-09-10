//go:build linux && text_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// This is an explicit local stream fixture, not authenticated Route admission.
// Actual worker framing and the immutable snapshot must still make progress
// after a different hostile cgroup is removed.
func requireInstalledPublisherProgress(t *testing.T, ctx context.Context, worker *qualifiedTextWorker, document []byte) {
	t.Helper()
	_, finish, err := worker.beginOperation(ctx, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	attachment := worker.lifetime.attachment
	if err := attachment.connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := attachment.connection.SetDeadline(time.Time{}); err != nil {
			t.Error(err)
		}
	}()
	write := func(kind byte, body []byte) {
		t.Helper()
		var header [9]byte
		header[0] = kind
		binary.BigEndian.PutUint32(header[1:5], 1)
		binary.BigEndian.PutUint32(header[5:], uint32(len(body)))
		if _, err := io.Copy(attachment, io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(body))); err != nil {
			t.Fatal(err)
		}
	}
	request := make([]byte, 512)
	copy(request, "ARDTXT01")
	request[8] = 1
	write(1, nil)
	write(2, request)
	write(4, nil)
	var response []byte
	for frame := 0; frame < 8; frame++ {
		var header [9]byte
		if _, err := io.ReadFull(attachment, header[:]); err != nil {
			t.Fatal(err)
		}
		if binary.BigEndian.Uint32(header[1:5]) != 1 {
			t.Fatal("unsolicited sibling stream")
		}
		size := binary.BigEndian.Uint32(header[5:])
		if size > 16<<10 {
			t.Fatal("oversized sibling frame")
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(attachment, body); err != nil {
			t.Fatal(err)
		}
		switch header[0] {
		case 2:
			if size == 0 || len(response)+len(body) > len(document)+13 {
				t.Fatal("invalid sibling response length")
			}
			response = append(response, body...)
		case 3:
			if size != 4 || binary.BigEndian.Uint32(body) != uint32(len(request)) {
				t.Fatal("invalid sibling credit")
			}
		case 4:
			if size != 0 || len(response) != len(document)+13 || string(response[:8]) != "ARDTXT01" || response[8] != 0 ||
				binary.BigEndian.Uint32(response[9:13]) != uint32(len(document)) || !bytes.Equal(response[13:], document) {
				t.Fatal("sibling snapshot did not survive victim cleanup")
			}
			write(5, []byte{0})
			return
		default:
			t.Fatal("sibling worker refused or emitted an unknown frame")
		}
	}
	t.Fatal("sibling response never completed")
}
