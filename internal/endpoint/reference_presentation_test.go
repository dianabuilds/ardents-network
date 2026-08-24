package endpoint

import (
	"errors"
	"net/http"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
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

func TestEndpointOpenReferenceFromLinkRequiresExactAuthenticatedTarget(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	selected := targetLinkBytes(33)
	text, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: selected})
	if err != nil {
		t.Fatal(err)
	}
	input := ReferencePresentation{AuthenticatedTarget: targetLinkBytes(34),
		Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}}
	if server, err := endpoint.OpenReferenceFromLink(text, input); !errors.Is(err, ErrReferenceTargetMismatch) || server != nil {
		t.Fatalf("mismatched presentation result = (%v, %v)", server, err)
	}
	input.AuthenticatedTarget = selected
	server, err := endpoint.OpenReferenceFromLink(text, input)
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
