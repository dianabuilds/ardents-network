//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

type sourcePublicFile struct {
	Path       string        `json:"path"`
	Commitment preparationID `json:"sha256"`
}

type sourcePreparationMember struct {
	Identity          preparationID    `json:"identity"`
	Family            string           `json:"family"`
	Handle            string           `json:"endpoint_handle"`
	Address           string           `json:"address"`
	ServerName        string           `json:"server_name"`
	Certificate       sourcePublicFile `json:"certificate"`
	Root              sourcePublicFile `json:"root"`
	Pin               preparationID    `json:"leaf_key_digest"`
	PrivateKey        string           `json:"private_key"`
	StateRoot         string           `json:"state_root"`
	ParticipationRoot string           `json:"participation_root"`
}

type sourcePreparationPlan struct {
	Schema            string                    `json:"schema"`
	Reference         string                    `json:"reference_time"`
	ClockObservedAt   string                    `json:"clock_observed_at"`
	Client            installedHeadless         `json:"client"`
	ClientCertificate sourcePublicFile          `json:"client_certificate"`
	ClientRoot        sourcePublicFile          `json:"client_root"`
	ClientPin         preparationID             `json:"client_leaf_key_digest"`
	ClientKey         string                    `json:"client_private_key"`
	OrderSeed         preparationID             `json:"order_seed"`
	MaterialIndex     uint32                    `json:"materialization_index"`
	RefreshMS         uint32                    `json:"refresh_interval_ms"`
	Sources           []sourcePreparationMember `json:"sources"`
}

func decodeSourcePreparation(raw []byte) (sourcePreparationPlan, time.Time, error) {
	var p sourcePreparationPlan
	bad := errNetworkPreparation
	if len(raw) > 64<<10 || preparationObject(raw, &p, "schema", "reference_time", "clock_observed_at", "client", "client_certificate", "client_root", "client_leaf_key_digest", "client_private_key", "order_seed", "materialization_index", "refresh_interval_ms", "sources") != nil || p.Schema != "ardents-source-trust-preparation-v1" || len(p.Sources) != 2 {
		return p, time.Time{}, bad
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if !requiredAdmissionObject(fields["client"], "schema", "role", "network_state_root", "local_role_state_root", "time_confidence_file", "text_token_root", "entry_state_root", "application_socket", "administration_socket", "publication_root", "service_instance_root", "network_id", "network_authorities", "network_threshold", "closed_profile_authority", "broker_id", "connection_principal", "administration_principal", "reader_permission", "publisher_permission") {
		return p, time.Time{}, bad
	}
	var client map[string]json.RawMessage
	_ = json.Unmarshal(fields["client"], &client)
	for _, field := range []string{"reader_permission", "publisher_permission"} {
		if !requiredAdmissionObject(client[field], "request_path", "response_path", "maxima") {
			return p, time.Time{}, bad
		}
		var permission map[string]json.RawMessage
		_ = json.Unmarshal(client[field], &permission)
		var maxima []uint32
		if json.Unmarshal(permission["maxima"], &maxima) != nil || len(maxima) != 3 {
			return p, time.Time{}, bad
		}
	}
	var members []json.RawMessage
	_ = json.Unmarshal(fields["sources"], &members)
	for _, member := range members {
		if !requiredAdmissionObject(member, "identity", "family", "endpoint_handle", "address", "server_name", "certificate", "root", "leaf_key_digest", "private_key", "state_root", "participation_root") {
			return p, time.Time{}, bad
		}
		var entry map[string]json.RawMessage
		_ = json.Unmarshal(member, &entry)
		for _, field := range []string{"certificate", "root"} {
			if !requiredAdmissionObject(entry[field], "path", "sha256") {
				return p, time.Time{}, bad
			}
		}
	}
	for _, field := range []string{"client_certificate", "client_root"} {
		if !requiredAdmissionObject(fields[field], "path", "sha256") {
			return p, time.Time{}, bad
		}
	}
	reference, err := admissionTime(p.Reference)
	if _, e := admissionTime(p.ClockObservedAt); e != nil {
		return p, time.Time{}, bad
	}
	if err != nil || p.MaterialIndex >= 64 || p.RefreshMS < 100 || p.RefreshMS > 60000 || p.OrderSeed == (preparationID{}) || p.Client.Schema != "ardents-headless-runtime-v2" {
		return p, time.Time{}, bad
	}
	return p, reference, nil
}

func runSourcePreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || args[0] != "prepare-trust" {
		return 2
	}
	for _, path := range args[1:] {
		if !absoluteAdmissionPath(path) {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	files, err := executeSourcePreparation(ctx, args[1], args[2], nil)
	if err != nil {
		return 1
	}
	// Local plans include opaque private paths; the receipt contains names and
	// public commitments only, after every original native resource has joined.
	receipt, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Files      map[string]string `json:"files"`
		Validation string            `json:"validation"`
	}{"ardents-source-trust-prepared-v1", files, "offline-public-observation"})
	if err != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
}

func executeSourcePreparation(ctx context.Context, path, destination string, observe func(string, *os.File)) (receipt map[string]string, err error) {
	if os.Geteuid() != 0 {
		return nil, errNetworkPreparation
	}
	plan, err := openSigningInput(ctx, path, 64<<10, false)
	if err != nil {
		return nil, err
	}
	inputs := []*signingInput{plan}
	defer func() {
		for i := len(inputs) - 1; i >= 0; i-- {
			err = errors.Join(err, inputs[i].close())
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			receipt = nil
		}
	}()
	if err = sourcePreparationMount(plan); err != nil {
		return nil, err
	}
	p, reference, err := decodeSourcePreparation(plan.raw)
	if err != nil {
		return nil, err
	}
	authority, err := installedNetworkAuthority(p.Client)
	if err != nil || !validAdmissionAuthority("", authority, p.Client.LocalRoleStateRoot) {
		return nil, errNetworkPreparation
	}
	// All runtime roots are opaque declarations. Check separation without opening
	// them. The existing pure configuration projection performs no file I/O;
	// no State owner is opened offline.
	roots := []string{p.Client.NetworkStateRoot, p.Client.LocalRoleStateRoot}
	privatePaths := []string{p.ClientKey}
	for _, member := range p.Sources {
		roots = append(roots, member.StateRoot, member.ParticipationRoot)
		privatePaths = append(privatePaths, member.PrivateKey)
	}
	if !independentRouteRoots(append(roots, destination)...) {
		return nil, errNetworkPreparation
	}
	for i, private := range privatePaths {
		if !absoluteAdmissionPath(private) || private == destination || strings.HasPrefix(private, destination+string(filepath.Separator)) {
			return nil, errNetworkPreparation
		}
		for _, other := range privatePaths[:i] {
			if private == other {
				return nil, errNetworkPreparation
			}
		}
	}
	check := func() error {
		if os.Geteuid() != 0 {
			return errNetworkPreparation
		}
		for _, input := range inputs {
			if e := input.check(ctx); e != nil {
				return e
			}
		}
		now := time.Now()
		if now.Before(reference.Add(-time.Minute)) || now.After(reference.Add(5*time.Minute)) {
			return errNetworkPreparation
		}
		return ctx.Err()
	}
	var artifacts []networkOutputFile
	read := func(declared sourcePublicFile, name string) ([]byte, error) {
		if !absoluteAdmissionPath(declared.Path) {
			return nil, errNetworkPreparation
		}
		for _, private := range privatePaths {
			if declared.Path == private {
				return nil, errNetworkPreparation
			}
		}
		input, e := openSigningInput(ctx, declared.Path, 64<<10, false)
		if e != nil {
			return nil, e
		}
		inputs = append(inputs, input)
		if e = sourcePreparationMount(input); e != nil {
			return nil, e
		}
		if sha256.Sum256(input.raw) != [32]byte(declared.Commitment) {
			return nil, errNetworkPreparation
		}
		artifacts = append(artifacts, networkOutputFile{name, input.raw})
		return input.raw, check()
	}
	if err = check(); err != nil {
		return nil, err
	}
	var trust source.PublicTrust
	trust.ClientCertificatePEM, err = read(p.ClientCertificate, "client-certificate.pem")
	if err != nil {
		return nil, err
	}
	trust.ClientRootPEM, err = read(p.ClientRoot, "client-root.pem")
	if err != nil {
		return nil, err
	}
	trust.ClientKeyDigest = p.ClientPin
	// Reuse the existing installed Source declaration, without introducing a
	// second runtime schema or granting Installation startup admission.
	declared := installedSource{Schema: "ardents-source-plan-v1", ClockObservedAt: p.ClockObservedAt, OrderSeed: hex.EncodeToString(p.OrderSeed[:]), MaterializationIndex: p.MaterialIndex, RefreshIntervalMS: p.RefreshMS, ClientCertificate: filepath.Join(destination, "client-certificate.pem"), ClientKey: p.ClientKey}
	appendJSON := func(name string, value any) error {
		raw, e := json.Marshal(value)
		if e == nil {
			artifacts = append(artifacts, networkOutputFile{name, append(raw, '\n')})
		}
		return e
	}
	for i, member := range p.Sources {
		name := fmt.Sprintf("source-%d", i+1)
		chain, e := read(member.Certificate, name+"-certificate.pem")
		if e != nil {
			return nil, e
		}
		root, e := read(member.Root, name+"-root.pem")
		if e != nil {
			return nil, e
		}
		trust.Sources[i] = source.PublicSource{Source: source.Source{Identity: member.Identity, Family: member.Family, EndpointHandle: member.Handle, Address: member.Address, ServerName: member.ServerName, RootPEM: root, LeafKeyDigest: member.Pin}, CertificatePEM: chain}
		// JSON adaptation uses the existing runtime decoder as its shape oracle.
		entry := struct {
			Address        string `json:"address"`
			ServerName     string `json:"server_name"`
			Identity       string `json:"identity"`
			Family         string `json:"family"`
			EndpointHandle string `json:"endpoint_handle"`
			RootCA         string `json:"root_ca"`
			LeafKeyDigest  string `json:"leaf_key_digest"`
		}{member.Address, member.ServerName, hex.EncodeToString(member.Identity[:]), member.Family, member.Handle, filepath.Join(destination, name+"-root.pem"), hex.EncodeToString(member.Pin[:])}
		declared.Sources = append(declared.Sources, entry)
		network := *authority
		network.Root = member.StateRoot
		serving := sourceServePlan{Network: &network, ProcessProfile: processresource.SourceProfile, ParticipationRoot: member.ParticipationRoot, Address: member.Address, Certificate: filepath.Join(destination, name+"-certificate.pem"), PrivateKey: member.PrivateKey, ClientRoot: filepath.Join(destination, "client-root.pem"), ClientPins: [][32]byte{p.ClientPin}}
		if !validSourceServePlan(serving) {
			return nil, errNetworkPreparation
		}
		if err = appendJSON(name+"-serve.json", serving); err != nil {
			return nil, err
		}
	}
	config, err := networkStateConfig(authority)
	if err != nil {
		return nil, err
	}
	if err = source.ObservePublicTrust(trust, config.Authorities, reference); err != nil {
		return nil, err
	}
	if err = appendJSON("client-source.json", declared); err != nil {
		return nil, err
	}
	if err = appendJSON("client-headless.json", p.Client); err != nil {
		return nil, err
	}
	if observe != nil {
		observe("inputs-read", plan.file)
	}
	if err = persistNetworkFiles(ctx, destination, artifacts, false, check, observe); err != nil {
		return nil, err
	}
	receipt = make(map[string]string, len(artifacts))
	for _, file := range artifacts {
		digest := sha256.Sum256(file.raw)
		receipt[file.name] = hex.EncodeToString(digest[:])
	}
	return receipt, nil
}

func sourcePreparationMount(input *signingInput) error {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(input.file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return errors.Join(errNetworkPreparation, err)
	}
	return nil
}
