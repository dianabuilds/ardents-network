package runtime

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestAdministrationRequestRetainsOriginalGrantWithoutWorkerAuthority(t *testing.T) {
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if request, err := owner.AdmitAdministration(t.Context(), [32]byte{3}); err == nil || request != nil {
		t.Fatal("foreign Principal acquired Administration admission")
	}
	ctx, cancel := context.WithCancel(t.Context())
	request, err := owner.AdmitAdministration(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	copyOfRequest := *request
	if copyOfRequest.Check() == nil || copyOfRequest.Context() != nil || copyOfRequest.Close() == nil {
		t.Fatal("copied request acquired the original admission")
	}
	if request.Check() != nil {
		t.Fatal("copied request retired the original admission")
	}
	cancel()
	<-request.Context().Done()
	if request.Check() == nil {
		t.Fatal("canceled caller retained local admission")
	}
	if err := request.Close(); err != nil {
		t.Fatal(err)
	}
	if request.Check() == nil {
		t.Fatal("joined request became current again")
	}
}

func TestAdministrationRequestSharesOriginalGenerationRetirement(t *testing.T) {
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	request, err := owner.AdmitAdministration(t.Context(), principal)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if request.Check() == nil || request.Context().Err() == nil {
		t.Fatal("retired generation retained request authority")
	}
	if err := request.Close(); err != nil {
		t.Fatal(err)
	}
	if later, err := owner.AdmitAdministration(t.Context(), principal); err == nil || later != nil {
		t.Fatal("closed generation admitted a replacement request")
	}
}
