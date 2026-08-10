package siteexperiment

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"time"
)

func runContractFailure(ctx context.Context, name, evidenceDirectory string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	now := time.Now()
	fixture, err := newAuthorityFixture("failure-run", "failure-network", now, rand.Reader)
	if err != nil {
		return err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	switch name {
	case "invalid_name":
		err = expectLookupRejected(ctx, fixture, "wrong.reference")
	case "absent_name":
		err = expectLookupRejected(ctx, fixture, "absent.reference")
	case "modified_name_record", "stale_name_record", "ohttp_nonce_mismatch":
		err = expectNameRecordRejected(fixture, name, nonce, now)
	case "wrong_target_descriptor_binding", "wrong_instance_credential", "expired_instance_credential", "superseded_instance_credential":
		err = expectDescriptorRejected(fixture, name, nonce, now)
	case "ohttp_replay":
		err = expectOHTTPReplayRejected(ctx, fixture, now)
	case "service_offline":
		err = expectApplicationFailure(ctx, connectionFailure{class: "service_offline"}, "service_offline")
	case "route_unavailable":
		err = expectApplicationFailure(ctx, connectionFailure{class: "route_unavailable"}, "route_unavailable")
	case "ambiguous_failure":
		err = expectApplicationFailure(ctx, errors.New("unclassified connector failure"), "indeterminate")
	case "application_dns_escape", "application_socket_escape", "application_listener_escape", "forbidden_origin_query_role_view":
		err = verifyRetainedIsolation(evidenceDirectory, name)
	default:
		return errors.New("unknown Gate C failure condition")
	}
	if err != nil {
		return err
	}
	path := filepath.Join(evidenceDirectory, "failures")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return writeBoundedJSON(filepath.Join(path, name+".json"), map[string]any{
		"schema_version": "gatec-failure-evidence/v1", "condition": name, "result": "rejected_as_required",
	})
}

func expectLookupRejected(ctx context.Context, fixture *authorityFixture, lookup string) error {
	resolver, err := openActiveResolver(fixture, time.Now())
	if err != nil {
		return err
	}
	defer resolver.close()
	_, _, err = resolveMessage(ctx, resolver.transport, "name", lookup, fixture.runID, fixture.networkID, time.Now())
	if err == nil {
		return errors.New("invalid or absent Name was resolved")
	}
	return nil
}

func expectOHTTPReplayRejected(ctx context.Context, fixture *authorityFixture, now time.Time) error {
	resolver, err := openActiveResolver(fixture, now)
	if err != nil {
		return err
	}
	defer resolver.close()
	query, _, err := makeResolutionQuery("name", "site.reference", fixture.runID, fixture.networkID, now)
	if err != nil {
		return err
	}
	if _, err := sendOHTTPMessage(ctx, resolver.transport, query); err != nil {
		return err
	}
	if _, err := sendOHTTPMessage(ctx, resolver.transport, query); err == nil {
		return errors.New("replayed OHTTP request was accepted")
	}
	return nil
}

func expectNameRecordRejected(fixture *authorityFixture, name string, nonce []byte, now time.Time) error {
	data, err := fixture.signedNameRecord(nonce, now.Add(10*time.Second))
	if err != nil {
		return err
	}
	if name == "modified_name_record" {
		data[len(data)/2] ^= 1
	} else if name == "stale_name_record" {
		data, err = fixture.signedNameRecord(nonce, now.Add(-time.Second))
	} else {
		nonce = append([]byte(nil), nonce...)
		nonce[0] ^= 1
	}
	if err != nil {
		return err
	}
	if _, verifyErr := verifyNameRecord(data, fixture.namePublic, fixture.runID, fixture.networkID, nonce, now); verifyErr == nil {
		return errors.New("invalid Name Record was accepted")
	}
	return nil
}

func expectDescriptorRejected(fixture *authorityFixture, name string, nonce []byte, now time.Time) error {
	data, err := fixture.signedDescriptor(nonce, now.Add(10*time.Second))
	if err != nil {
		return err
	}
	target, generation, when, serviceKey := fixture.target, fixture.instanceGeneration, now, fixture.servicePublic
	if name == "wrong_target_descriptor_binding" {
		target += "-wrong"
	} else if name == "wrong_instance_credential" {
		serviceKey, _, err = ed25519.GenerateKey(rand.Reader)
	} else if name == "expired_instance_credential" {
		when = now.Add(10 * time.Minute)
	} else {
		if err := fixture.migrate(now, rand.Reader); err != nil {
			return err
		}
		generation = fixture.instanceGeneration
	}
	if err != nil {
		return err
	}
	if _, verifyErr := verifyDescriptor(data, serviceKey, fixture.runID, fixture.networkID, nonce, target, generation, when); verifyErr == nil {
		return errors.New("invalid Descriptor or Credential was accepted")
	}
	return nil
}

func expectApplicationFailure(ctx context.Context, connectorErr error, expected string) error {
	application, endpoint := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serveClientConnection(ctx, endpoint, func(context.Context, connectRequest) (connectionResult, io.ReadWriteCloser, error) {
			return connectionResult{}, nil, connectorErr
		})
	}()
	if err := writeConnectRequest(application); err != nil {
		return err
	}
	response, err := readConnectResponse(application)
	_ = application.Close()
	serveErr := <-done
	if err != nil || serveErr == nil || response.Status != "failed" || response.Class != expected || response.Target != "" || response.NameGeneration != 0 || response.NameRevision != 0 || response.InstanceGeneration != 0 {
		return errors.New("application Interface exposed or misclassified a connection failure")
	}
	encoded, err := json.Marshal(response)
	if err != nil || bytes.Contains(encoded, []byte("node")) || bytes.Contains(encoded, []byte("relay")) || bytes.Contains(encoded, []byte("gateway")) {
		return errors.New("application Interface failure exposed topology")
	}
	return nil
}

func verifyRetainedIsolation(evidenceDirectory, condition string) error {
	reference := filepath.Join(evidenceDirectory, "attempts", "001", "reference")
	if condition == "forbidden_origin_query_role_view" {
		var relay struct {
			Cleartext bool `json:"exact_name_or_target_visible"`
		}
		var gateway struct {
			Queries          []string `json:"plaintext_query_types"`
			AuthorityPrivate bool     `json:"authority_private_key_present"`
		}
		if readStrictEvidence(filepath.Join(reference, "relay", "relay.json"), &relay) != nil || readStrictEvidence(filepath.Join(reference, "gateway", "gateway.json"), &gateway) != nil || relay.Cleartext || gateway.AuthorityPrivate || !slices.Equal(gateway.Queries, []string{"name", "reachability"}) {
			return errors.New("resolution role view violates the fixed knowledge matrix")
		}
		return nil
	}
	var isolation struct {
		NetworkNone     bool `json:"application_network_mode_none"`
		FilesystemViews bool `json:"principal_filesystem_views"`
		PublishedPorts  bool `json:"published_ports"`
		DNSRejected     bool `json:"active_dns_escape_rejected"`
		SocketRejected  bool `json:"active_socket_escape_rejected"`
		ListenerAbsent  bool `json:"active_listener_absent"`
	}
	if err := readStrictEvidence(filepath.Join(reference, "isolation.json"), &isolation); err != nil {
		return err
	}
	if condition == "application_dns_escape" && (!isolation.NetworkNone || !isolation.DNSRejected) || condition == "application_socket_escape" && (!isolation.FilesystemViews || !isolation.SocketRejected) || isolation.PublishedPorts {
		return errors.New("application isolation evidence does not reject the requested escape")
	}
	if condition == "application_listener_escape" && !isolation.ListenerAbsent {
		return errors.New("HTTP Application opened an ordinary listener")
	}
	return nil
}

func readStrictEvidence(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 1024*1024 {
		return errors.New("required bounded failure evidence is absent")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return err
	}
	return nil
}
