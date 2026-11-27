//go:build linux

package document_test

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func TestDocumentReadUsesOneAAI3ConnectionAndSafePresentation(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, body := range [][]byte{nil, []byte("safe\n\x1b[31m\u202etext\t\u0085"), bytes.Repeat([]byte("x"), document.MaximumBytes)} {
		peer := &documentReadPeer{response: documentResponse(body), class: endpointstream.CleanClose}
		socket := documentReadSocket(t, peer)
		var want, output, diagnostic bytes.Buffer
		if err := document.WritePlainText(&want, body); err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), binary, "read", socket)
		command.Stdin = strings.NewReader("fixture-link\n")
		command.Stdout, command.Stderr = &output, &diagnostic
		if err := command.Run(); err != nil || !bytes.Equal(output.Bytes(), want.Bytes()) ||
			diagnostic.String() != `{"operation":"application.read","phase":"command","outcome":"completed"}`+"\n" || peer.opens.Load() != 1 || peer.requestBytes.Load() != 0 {
			t.Fatalf("document read: %v, output=%q, diagnostic=%q, opens=%d, request=%d",
				err, &output, &diagnostic, peer.opens.Load(), peer.requestBytes.Load())
		}
	}
}

func TestDocumentReadNeverPresentsPartialOrFailedConnection(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, scenario := range []struct {
		response []byte
		class    endpointstream.OutcomeClass
	}{
		{documentResponse([]byte("secret")), endpointstream.LocalFailure},
		{bytes.Repeat([]byte("x"), document.MaximumBytes+1), endpointstream.CleanClose},
		{documentResponse([]byte{0xff}), endpointstream.CleanClose},
	} {
		peer := &documentReadPeer{response: scenario.response, class: scenario.class}
		socket := documentReadSocket(t, peer)
		command := exec.CommandContext(t.Context(), binary, "read", socket)
		command.Stdin = strings.NewReader("fixture-link\n")
		var output, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &output, &diagnostic
		if err := command.Run(); err == nil || output.Len() != 0 || peer.opens.Load() != 1 {
			t.Fatalf("failed response: error=%v, output=%q, opens=%d", err, &output, peer.opens.Load())
		}
		if bytes.Contains(diagnostic.Bytes(), []byte("secret")) {
			t.Fatal("diagnostic exposed response")
		}
	}
}

func TestDocumentReadPreservesRefusedSetupClassWithoutPresentingItsReason(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, scenario := range []struct {
		class endpointstream.OutcomeClass
		want  string
	}{
		{endpointstream.ServiceUnavailable, "document operation unavailable"},
		{endpointstream.LocalFailure, "local connection failed"},
		{endpointstream.IndeterminateFailure, "connection outcome unknown"},
		{endpointstream.CapacityUnavailable, "document capacity unavailable"},
		{endpointstream.LocalCancellation, "read cancelled"},
		{endpointstream.LocalTimeout, "read timed out"},
		{endpointstream.CleanClose, "document operation unavailable"},
		{"fixture private class", "document operation unavailable"},
	} {
		t.Run(string(scenario.class), func(t *testing.T) {
			peer := &refusingDocumentReadPeer{outcome: endpointstream.Outcome{Class: scenario.class, Reason: "private fixture detail"}}
			socket := documentReadSocket(t, peer)
			command := exec.CommandContext(t.Context(), binary, "read", socket)
			command.Stdin = strings.NewReader("fixture-link\n")
			var output, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &output, &diagnostic
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 ||
				diagnostic.String() != scenario.want+"\n"+`{"operation":"application.read","phase":"command","outcome":"usage-or-output-failure"}`+"\n" || peer.opens.Load() != 1 {
				t.Fatalf("refused setup: error=%v, output=%q, diagnostic=%q, opens=%d", err, &output, &diagnostic, peer.opens.Load())
			}
		})
	}
}

func TestDocumentDestinationIsBoundedAndNeverRewritten(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, scenario := range []struct {
		name, input, destination string
	}{
		{"newline", "fixture-link\n", "fixture-link"},
		{"crlf", "fixture-link\r\n", "fixture-link"},
		{"eof", "fixture-link", "fixture-link"},
		{"maximum", strings.Repeat("x", 512) + "\r\n", strings.Repeat("x", 512)},
		{"preserve-spaces", " first link \nsecond\n", " first link "},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			peer := &documentReadPeer{destination: scenario.destination, response: documentResponse([]byte("ok")), class: endpointstream.CleanClose}
			command := exec.CommandContext(t.Context(), binary, "read", documentReadSocket(t, peer))
			command.Stdin = strings.NewReader(scenario.input)
			var output, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &output, &diagnostic
			if err := command.Run(); err != nil || output.String() != "ok" || diagnostic.String() != `{"operation":"application.read","phase":"command","outcome":"completed"}`+"\n" || peer.opens.Load() != 1 {
				t.Fatalf("input %q: error=%v, output=%q, diagnostic=%q, opens=%d", scenario.name, err, &output, &diagnostic, peer.opens.Load())
			}
		})
	}
	for _, input := range []string{"", "\n", "\r\n", "\xff\n", strings.Repeat("x", 513), strings.Repeat("x", 100000)} {
		peer := &documentReadPeer{response: documentResponse([]byte("secret")), class: endpointstream.CleanClose}
		command := exec.CommandContext(t.Context(), binary, "read", documentReadSocket(t, peer))
		command.Stdin = strings.NewReader(input)
		var output, diagnostic bytes.Buffer
		command.Stdout, command.Stderr = &output, &diagnostic
		err := command.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 ||
			diagnostic.String() != "document destination or local input is invalid\n"+`{"operation":"application.read","phase":"command","outcome":"usage-or-output-failure"}`+"\n" || peer.opens.Load() != 0 {
			t.Fatalf("invalid input length %d: error=%v, output=%q, diagnostic=%q, opens=%d", len(input), err, &output, &diagnostic, peer.opens.Load())
		}
	}
}
