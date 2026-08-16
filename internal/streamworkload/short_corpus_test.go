package streamworkload

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

func TestShortCorpusCarriesCanonicalNonceRequest(t *testing.T) {
	client, publisher := net.Pipe()
	defer client.Close()
	defer publisher.Close()
	nonce, responseSeed := [32]byte{17}, [32]byte{91}
	clientDone := make(chan error, 1)
	go func() {
		result, err := ExchangeShort(client, "client", nonce, responseSeed, nil, nil)
		if result.Terminal != "success" || result.Corpus != shortCorpus || result.RequestNonce != nonce {
			err = fmt.Errorf("client short result = %+v: %w", result, err)
		}
		clientDone <- err
	}()
	requestRaw := make([]byte, 512)
	if _, err := io.ReadFull(publisher, requestRaw); err != nil {
		t.Fatal(err)
	}
	request, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(requestRaw)))
	if err != nil {
		t.Fatal(err)
	}
	if request.Method != http.MethodPost || request.URL.Path != "/" ||
		request.Header.Get("X-Connection-Nonce") != fmt.Sprintf("%x", nonce) || request.ContentLength != 324 {
		t.Fatalf("short request = method %s path %s nonce %q length %d", request.Method, request.URL.Path,
			request.Header.Get("X-Connection-Nonce"), request.ContentLength)
	}
	response := make([]byte, 64<<10)
	(&generator{seed: responseSeed}).fill(response)
	if _, err := publisher.Write(response); err != nil {
		t.Fatal(err)
	}
	if err := <-clientDone; err != nil {
		t.Fatal(err)
	}
}
