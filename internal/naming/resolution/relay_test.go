package resolution_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	nameresolution "github.com/dianabuilds/ardents-network/internal/naming/resolution"
	"github.com/openpcc/ohttp"
)

func TestRelayStripsEndpointHeaders(t *testing.T) {
	t.Parallel()
	seen := make(chan http.Header, 1)
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen <- request.Header.Clone()
		writer.Header().Set("Content-Type", ohttp.ResponseMediaType)
		_, _ = writer.Write([]byte{1})
	}))
	defer gateway.Close()
	relay, err := nameresolution.NewRelay(gateway.URL, gateway.Client())
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://relay.invalid/ohttp", bytes.NewReader([]byte{1}))
	request.Header.Set("Content-Type", ohttp.RequestMediaType)
	request.Header.Set("Forwarded", "for=endpoint")
	request.Header.Set("Via", "endpoint")
	request.Header.Set("Cookie", "identity=endpoint")
	request.Header.Set("Authorization", "Bearer endpoint")
	recorder := httptest.NewRecorder()
	relay.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("relay status=%d", recorder.Code)
	}
	headers := <-seen
	for _, forbidden := range []string{"Forwarded", "Via", "Cookie", "Authorization"} {
		if headers.Get(forbidden) != "" {
			t.Fatalf("Relay forwarded %s", forbidden)
		}
	}
}
