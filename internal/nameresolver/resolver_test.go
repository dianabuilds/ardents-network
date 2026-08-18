package nameresolver

import (
	"testing"
	"time"
)

import "github.com/dianabuilds/ardents-network/internal/namelease"

func TestResolveNameRole(t *testing.T) {
	t.Parallel()

	record := namelease.Record{
		Name:           "blog.example",
		Generation:     1,
		Revision:       1,
		State:          "active",
		Authority:      "alice",
		Target:         "target",
		LeaseExpiresAt: time.Now().UTC().Unix() + 10,
		GraceExpiresAt: time.Now().UTC().Unix() + 20,
	}
	now := time.Now().UTC().Unix()

	res, err := Resolve(record, now, ResolverQuery{Role: "lookup", Name: "blog.example"})
	if err != nil {
		t.Fatalf("lookup resolve: %v", err)
	}
	if res.Name != "blog.example" || res.Target != "target" {
		t.Fatalf("unexpected lookup result: %+v", res)
	}
}

func TestResolveEndpointIsolation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	record := namelease.Record{
		Name:           "blog.example",
		Generation:     2,
		Revision:       3,
		State:          "active",
		Authority:      "alice",
		Target:         "target",
		LeaseExpiresAt: now + 10,
		GraceExpiresAt: now + 20,
	}

	res, err := Resolve(record, now, ResolverQuery{Role: "endpoint", Target: "target"})
	if err != nil {
		t.Fatalf("endpoint resolve: %v", err)
	}
	if res.Name != "" || res.Target != "target" {
		t.Fatalf("expected endpoint-only target view, got %+v", res)
	}

	_, err = Resolve(record, now, ResolverQuery{Role: "endpoint", Target: "target", Name: "blog.example"})
	if err == nil {
		t.Fatal("expected mixed role rejection")
	}
}

func TestStaleOrClosedStatesFailClosed(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Unix()
	record := namelease.Record{
		Name:       "blog.example",
		Generation: 1,
		Revision:   1,
		State:      "released",
	}

	if _, err := Resolve(record, now, ResolverQuery{Role: "lookup", Name: "blog.example"}); err == nil {
		t.Fatal("expected release state to fail")
	}
}
