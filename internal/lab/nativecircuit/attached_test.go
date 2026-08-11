package nativecircuit

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestAttachedEndpointCarriesArbitraryBidirectionalBytes(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	certificate, trust := newEndpointFixture(t, "attached-instance")
	userTransport, serviceTransport := net.Pipe()
	userApplication, userRoute := net.Pipe()
	serviceApplication, serviceRoute := net.Pipe()
	nonce := randomHandle(t)
	errorsFound := make(chan error, 2)
	go func() {
		_, err := runEndpointUserAttached(ctx, userTransport, trust, nonce, userRoute, nil)
		errorsFound <- err
	}()
	go func() {
		_, err := runEndpointServiceAttached(ctx, serviceTransport, certificate, nonce, serviceRoute)
		errorsFound <- err
	}()

	upload := bytes.Repeat([]byte{0x00, 0xff, 0x51, 0x7e}, 32*1024)
	download := bytes.Repeat([]byte{0x13, 0x37, 0x00, 0x80}, 16*1024)
	go func() { _, _ = userApplication.Write(upload) }()
	receivedUpload := make([]byte, len(upload))
	if _, err := io.ReadFull(serviceApplication, receivedUpload); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(receivedUpload, upload) {
		t.Fatal("attached Route changed upload bytes")
	}
	go func() { _, _ = serviceApplication.Write(download) }()
	receivedDownload := make([]byte, len(download))
	if _, err := io.ReadFull(userApplication, receivedDownload); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(receivedDownload, download) {
		t.Fatal("attached Route changed download bytes")
	}
	_ = userApplication.Close()
	_ = serviceApplication.Close()
	for range 2 {
		if err := <-errorsFound; err != nil {
			t.Fatal(err)
		}
	}
}
