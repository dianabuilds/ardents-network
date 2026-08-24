package endpoint

import (
	"net/http"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

func TestOpenReferencePresentationRequiresAnAuthenticatedTarget(t *testing.T) {
	if server, err := OpenReferencePresentation(ReferencePresentation{Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}}); err == nil || server != nil {
		t.Fatalf("missing Target presentation result = (%v, %v)", server, err)
	}
	server, err := OpenReferencePresentation(ReferencePresentation{AuthenticatedTarget: [32]byte{1},
		Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	response, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Reference presentation status = %d", response.StatusCode)
	}
}
