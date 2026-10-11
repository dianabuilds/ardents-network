//go:build linux && text_worker_installed

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Runs inside the original installed non-root Endpoint MainPID. Production
// console composition launches the actual selected snapshot worker and uses
// genuine signed Network, Stock, receiving Admission, Hosting and Store owners.
// Authority approval and blind issuance are independently encoded fixture
// inputs; neither supplies a qualified operation or successful transport ACK.
func runInstalledPublisher(t *testing.T, carrier transport.CarrierProfile) {
	f, reservations, certificates := installedPublisherNetwork(t, carrier)
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	plan, target := installedPublisherInstance(t, f.profile.NetworkID)
	storeRoot := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	servers := installedPublisherReceivers(t, f, reservations, certificates, store)
	clock, joinClock := joinProcessClock(t)
	defer joinClock()
	var authority [32]byte
	copy(authority[:], f.spec.Authority.Public().(ed25519.PublicKey))
	network := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.profile.NetworkID, Authorities: [][32]byte{authority}, Threshold: 1, ProfileAuthority: authority, ClockObservationFile: clock}
	config, err := networkStateConfig(&network)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = seed.Accept(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials)
	if err == nil {
		_, err = seed.AcceptClosedProfile(bundle.Profile)
	}
	if err := errors.Join(err, seed.Close()); err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	routePlan := routePrefixPlan{EntryRoot: filepath.Join(local, "entry"), InteriorRoot: filepath.Join(local, "introduction"), SourceInteriorRoot: filepath.Join(local, "source"), ResponderInteriorRoot: filepath.Join(local, "responder"), HostingRoot: routeProcessBudget(t), Domain: 4, Deadline: time.Now().Add(40 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	snapshot := filepath.Join(local, "snapshot.txt")
	if err := os.WriteFile(snapshot, []byte("genuine Publisher selected snapshot\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := hostingConfig(t, map[string]any{"generation": [32]byte{31}, "principal": [32]byte{32}, "snapshot": snapshot, "holder": holderPlan{Root: t.TempDir(), Network: &network, Role: admission.AllocationPublisher, Route: &routePlan, Publication: &plan}})
	input, sendInput := io.Pipe()
	readOutput, output := io.Pipe()
	var diagnostics bytes.Buffer
	finished := make(chan int, 1)
	go func() {
		defer input.Close()
		defer output.Close()
		finished <- runExecutionRouteHolder(ctx, []string{"--config", path}, input, output, &diagnostics)
	}()
	joined := false
	t.Cleanup(func() {
		cancel()
		_ = sendInput.Close()
		_ = readOutput.Close()
		if !joined {
			<-finished
		}
		if t.Failed() {
			t.Log(diagnostics.String())
		}
	})
	encoder, replies := json.NewEncoder(sendInput), bufio.NewReader(readOutput)
	send := func(command any) localAdmissionReply {
		t.Helper()
		if err := encoder.Encode(command); err != nil {
			t.Fatal(err)
		}
		var result localAdmissionReply
		if err := readConsoleReply(replies, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	installedPublisherStock(t, f, send)
	completed := func(operation string) localAdmissionReply {
		t.Helper()
		result := send(holderCommand{Operation: operation})
		if result.Outcome != "completed" {
			t.Fatalf("Publisher %s refused: %+v", operation, result)
		}
		return result
	}
	completed("publication-open")
	if result := send(holderCommand{Operation: "publication-link"}); result.Outcome != "refused" || result.Link != "" {
		t.Fatal("pending Publisher supplied Link")
	}
	expectedLink, err := publication.TargetLink(f.profile.NetworkID, target)
	if err != nil {
		t.Fatal(err)
	}
	if result := completed("publication-publish"); result.Link != expectedLink {
		t.Fatal("actual ACK returned foreign Link")
	}
	first, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
	if err != nil || outcome != reachability.Accepted {
		t.Fatal("actual receiving Store lacks acknowledged Descriptor", outcome, err)
	}
	firstProof, err := reachability.Verify(first, target, f.profile.NetworkID, f.profile.Digest, time.Now())
	if err != nil || firstProof.Introduction.Node != [32]byte{25} || firstProof.Introduction.Revision != 1 {
		t.Fatal("genuine registration Descriptor binding", err)
	}
	if !t.Run("delivery", func(t *testing.T) { installedPublisherDelivery(t, f, firstProof) }) {
		t.Fatal("genuine Publisher delivery failed")
	}
	if result := completed("publication-publish"); result.Link != expectedLink {
		t.Fatal("exact retry changed Link")
	}
	again, _, err := store.Lookup(target, f.profile.Digest, time.Now())
	if err != nil || !bytes.Equal(first, again) {
		t.Fatal("exact retry changed signed recipient bytes", err)
	}
	if result := completed("publication-refresh"); result.Link != expectedLink {
		t.Fatal("refresh changed destination")
	}
	second, outcome, err := store.Lookup(target, f.profile.Digest, time.Now())
	if err != nil || outcome != reachability.Accepted {
		t.Fatal("replacement lacks genuine Store commit", err)
	}
	secondProof, err := reachability.Verify(second, target, f.profile.NetworkID, f.profile.Digest, time.Now())
	if err != nil || secondProof.Introduction.Revision != 2 || secondProof.Introduction.Slot == firstProof.Introduction.Slot || secondProof.Introduction.RecipientKey == firstProof.Introduction.RecipientKey || secondProof.PublicationDigest != firstProof.PublicationDigest {
		t.Fatal("replacement did not retain Publication with fresh slot/key", err)
	}
	if result := send(holderCommand{Operation: "publication-refresh"}); result.Outcome != "refused" {
		t.Fatal("retained predecessor admitted third recipient")
	}
	completed("publication-withdraw")
	if result := send(holderCommand{Operation: "publication-link"}); result.Outcome != "refused" || result.Link != "" {
		t.Fatal("withdrawn Publisher retained accepting Link")
	}
	completed("close")
	_ = sendInput.Close()
	select {
	case code := <-finished:
		joined = true
		if code != 0 {
			t.Fatalf("joined Publisher exit %d: %s", code, diagnostics.String())
		}
	case <-ctx.Done():
		t.Fatal("Publisher did not join original worker", ctx.Err())
	}
	for _, server := range servers {
		if err := server.Close(); err != nil {
			t.Fatal("actual receiver join", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	retained, outcome, lookupErr := reopened.Lookup(target, f.profile.Digest, time.Now())
	if err := errors.Join(lookupErr, reopened.Close()); err != nil || outcome != reachability.Accepted || !bytes.Equal(retained, second) {
		t.Fatal("independently reopened Store lost actual committed proof", err)
	}
	private, err := instance.Open(t.Context(), plan.InstanceRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, unavailable := private.Credential(t.Context())
	if err := private.Close(); err != nil || !errors.Is(unavailable, instance.ErrSuccessorRequired) {
		t.Fatal("consumed Instance restart revived authority", unavailable, err)
	}
}

// The actual snapshot-console Publisher consumes the delivery itself. This
// independent Source owns separate permission, Stock, selections and Hosting;
// verified Store bytes supply the recipient key, never an accepting callback.
func installedPublisherDelivery(t *testing.T, f *networkAdmissionFixture, proof reachability.Descriptor) {
	source := installedPublisherDeliverySource(t, f, f.authority)
	// Join before this subtest returns and Go cancels its original Context.
	// Post-cancellation Cleanup cannot require graceful physical retirement.
	defer func() {
		if err := source.close(); err != nil {
			t.Error("Source physical join", err)
		}
	}()
	installedPublisherDeliveryReplay(t, f, proof, source)
}

// Each independent Source consumes its own unchanged two bootstrap batches and
// finite User allocation. The caller owns physical Close before Context ends.
func installedPublisherDeliverySource(t *testing.T, f *networkAdmissionFixture, authority admissionAuthority) routeHandle {
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, 4, 0})
	challenge := func(id byte, class uint8) token.ClosedTokenContext {
		return token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest,
			IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9,
			Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	}
	var forwards []token.ClosedTokenContext
	for _, id := range []byte{12, 13, 14, 15} {
		forwards = append(forwards, challenge(id, 2))
	}
	for i, challenges := range [][]token.ClosedTokenContext{forwards, {challenge(25, 1), challenge(25, 1), challenge(25, 1), challenge(25, 1)}} {
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges,
			Selection: stock.ExchangeBinding{ID: [32]byte{byte(80 + i)}, ProfileDigest: f.profile.Digest},
			Bootstrap: true, Deadline: f.profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		raw, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		// Offline fixture issuance grants finite tokens, not network or Service
		// acceptance. The original two bounded bootstrap batches are unchanged.
		issued := issuer.IssueCurrent(t.Context(), f.plan, raw, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal("Source fixture issuance", issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	plan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "source"),
		HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	source, err := startRoutePrefix(t.Context(), plan, authority, holder)
	if err != nil {
		t.Fatal("independent Source opening", err)
	}
	return source
}

// One genuine successful RESULT/CLOSE supplies the private-history oracle for
// later original worker loss. Source owns separate unchanged finite permission;
// this helper never calls a private commit or returns a readiness substitute.
func installedPublisherAcceptedNonce(t *testing.T, f *networkAdmissionFixture, proof reachability.Descriptor) [32]byte {
	return installedPublisherSubmitNonce(t, f, proof, true)
}

func installedPublisherSubmitNonce(t *testing.T, f *networkAdmissionFixture, proof reachability.Descriptor, accepting bool) [32]byte {
	t.Helper()
	source := installedPublisherDeliverySource(t, f, f.authority)
	defer func() {
		if err := source.close(); err != nil {
			t.Error("independent accepted-nonce Source physical join", err)
		}
	}()
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	rendezvous, err := view.RetainDuty([32]byte{41}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	intro, err := view.RetainDuty(proof.Introduction.Node, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	header := capsule.Header{Slot: proof.Introduction.Slot, Revision: proof.Introduction.Revision, Expiry: expiry}
	request := capsule.Request{Network: proof.Network, Target: proof.Target, PublicationDigest: proof.PublicationDigest,
		Revision: proof.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: proof.ProfileDigest, AttachmentGeneration: 1, Deadline: expiry,
		WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
	for _, secret := range []*[32]byte{&header.DeliveryNonce, &request.JoinSecret, &request.HandshakeContext, &request.ConnectionNonce, &request.InitiatorBinding} {
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
	}
	// Independent canonical plaintext feeds the real Source preparation path.
	raw := make([]byte, 0, 344)
	for _, field := range [][32]byte{request.Network, request.Target, request.PublicationDigest} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, request.Revision)
	raw = append(raw, request.RendezvousNode[:]...)
	raw = binary.BigEndian.AppendUint64(raw, request.RendezvousDutyGeneration)
	for _, field := range [][32]byte{request.JoinSecret, request.HandshakeContext, request.ProfileDigest, request.ConnectionNonce} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, request.AttachmentGeneration)
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.Deadline.Unix()))
	raw = append(raw, request.InitiatorBinding[:]...)
	for _, bound := range []int64{request.WorkSafetyNotAfter, request.WorkSafetyMaximum, request.NoNewRecoveryAfter} {
		raw = binary.BigEndian.AppendUint64(raw, uint64(bound))
	}
	defer clear(raw)
	digest, err := request.Digest()
	if err != nil || sha256.Sum256(raw) != digest {
		t.Fatal("independent request bytes differ", err)
	}
	err = source.sealSubmitCapsule(t.Context(), routeRecipient{Node: intro.NodeID, Generation: intro.RecordGeneration}, routeCapsuleIntent{
		Slot: header.Slot, DeliveryNonce: header.DeliveryNonce, Revision: header.Revision, Expiry: header.Expiry,
		RecipientKey: proof.Introduction.RecipientKey, Request: raw,
	})
	if accepting && err != nil || !accepting && !submissionRefusalOnly(err) {
		t.Fatal("genuine delivery disagreed with accepting pair", accepting, err)
	}
	return header.DeliveryNonce
}

func installedPublisherDeliveryReplay(t *testing.T, f *networkAdmissionFixture, proof reachability.Descriptor, source routeHandle) {
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{41}, view.ObservedAt())
	if err != nil || member.RoleDomain != 2 || member.Subrole != 4 {
		t.Fatal("genuine signed Rendezvous absent", err)
	}
	rendezvous, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	intro, err := view.RetainDuty(proof.Introduction.Node, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	header := capsule.Header{Slot: proof.Introduction.Slot, Revision: proof.Introduction.Revision, Expiry: expiry}
	request := capsule.Request{Network: proof.Network, Target: proof.Target, PublicationDigest: proof.PublicationDigest,
		Revision: proof.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: proof.ProfileDigest, AttachmentGeneration: 1, Deadline: expiry,
		WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
	for _, secret := range []*[32]byte{&header.DeliveryNonce, &request.JoinSecret, &request.HandshakeContext, &request.ConnectionNonce, &request.InitiatorBinding} {
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
	}
	envelope, _, err := capsule.Seal(header, proof.Introduction.RecipientKey, request)
	if err != nil {
		t.Fatal(err)
	}
	recipient := routeRecipient{Node: intro.NodeID, Generation: intro.RecordGeneration}
	if !t.Run("accepted", func(t *testing.T) {
		t.Run("concurrent", func(t *testing.T) {
			// Each submission uses its own actual role TLS, Control token and
			// request nonce. Only the sealed delivery nonce is shared.
			start := make(chan struct{})
			ready := make(chan struct{}, 2)
			results := make(chan error, 2)
			for range 2 {
				go func() {
					ready <- struct{}{}
					<-start
					results <- source.submitCapsule(t.Context(), recipient, envelope.Bytes())
				}()
			}
			<-ready
			<-ready
			close(start)
			first, second := <-results, <-results
			if !(first == nil && submissionRefusalOnly(second) || second == nil && submissionRefusalOnly(first)) {
				t.Fatal("concurrent duplicate requires one successful RESULT/CLOSE and one exact joined refusal", first, second)
			}
		})
	}) {
		return
	}
	if !t.Run("replay", func(t *testing.T) {
		if err := source.submitCapsule(t.Context(), recipient, envelope.Bytes()); !submissionRefusalOnly(err) {
			t.Fatal("accepted nonce replay lacked exact refusal and clean physical join", err)
		}
	}) {
		return
	}
	t.Run("retained-replay", func(t *testing.T) {
		// The original initial-delivery user expires under its real timer.
		// Fresh valid bytes then isolate accepted history from an expired
		// envelope or a still-pending duplicate reservation. The causal control
		// removing only nonce commitment must accept and fail this assertion.
		timer := time.NewTimer(time.Until(expiry.Add(250 * time.Millisecond)))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal("original delivery expiry wait interrupted", t.Context().Err())
		}
		freshExpiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
		header.Expiry = freshExpiry
		request.Deadline = freshExpiry
		request.WorkSafetyNotAfter = freshExpiry.Unix()
		request.WorkSafetyMaximum = freshExpiry.Unix()
		request.NoNewRecoveryAfter = freshExpiry.Unix()
		fresh, _, err := capsule.Seal(header, proof.Introduction.RecipientKey, request)
		if err != nil {
			t.Fatal(err)
		}
		if err := source.submitCapsule(t.Context(), recipient, fresh.Bytes()); !submissionRefusalOnly(err) {
			t.Fatal("accepted nonce lost after original delivery expiry", err)
		}
	})
}

// Hold actual receiving CLOSE after Publisher acceptance, then let the original
// capsule expire. A replacement already acknowledged before the failure supplies
// the independent live recipient for retained replay and a new-nonce control.
func installedPublisherFailedDelivery(t *testing.T, f *networkAdmissionFixture, predecessor, current reachability.Descriptor, gate *publicationDeliveryCloseGate) {
	source := installedPublisherDeliverySource(t, f, f.authority)
	expired := false
	defer func() {
		if err := source.close(); err != nil {
			if !expired || !publicationDeliveryExpiryOnly(err) {
				t.Error("independent Source physical join", err)
			} else {
				t.Log("retained original failed Source after capsule expiry", err)
			}
		}
	}()
	// An independently admitted Source supplies the replay oracle. A failed
	// first Source cannot acquire a fresh permit or reuse/refund spent Stock.
	// Each retains its own unchanged [4,4,0] allocation and two bootstrap batches.
	control := installedPublisherDeliverySource(t, f, f.authority)
	defer func() {
		if err := control.close(); err != nil {
			t.Error("independent replay Source physical join", err)
		}
	}()
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	rendezvous, err := view.RetainDuty([32]byte{41}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	intro, err := view.RetainDuty(current.Introduction.Node, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	if predecessor.Introduction.Node != current.Introduction.Node || predecessor.PublicationDigest != current.PublicationDigest || predecessor.Introduction.RecipientKey == current.Introduction.RecipientKey {
		t.Fatal("failed-delivery control lacks independent current and predecessor")
	}
	expiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	header := capsule.Header{Slot: predecessor.Introduction.Slot, Revision: predecessor.Introduction.Revision, Expiry: expiry}
	request := capsule.Request{Network: predecessor.Network, Target: predecessor.Target, PublicationDigest: predecessor.PublicationDigest,
		Revision: predecessor.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: predecessor.ProfileDigest, AttachmentGeneration: 1, Deadline: expiry,
		WorkSafetyNotAfter: expiry.Unix(), WorkSafetyMaximum: expiry.Unix(), NoNewRecoveryAfter: expiry.Unix()}
	for _, secret := range []*[32]byte{&header.DeliveryNonce, &request.JoinSecret, &request.HandshakeContext, &request.ConnectionNonce, &request.InitiatorBinding} {
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
	}
	envelope, _, err := capsule.Seal(header, predecessor.Introduction.RecipientKey, request)
	if err != nil {
		t.Fatal(err)
	}
	recipient := routeRecipient{Node: intro.NodeID, Generation: intro.RecordGeneration}
	gate.arm()
	finished := make(chan error, 1)
	go func() { finished <- source.submitCapsule(t.Context(), recipient, envelope.Bytes()) }()
	joined := false
	defer func() {
		gate.release()
		if !joined {
			<-finished
		}
	}()
	select {
	case <-gate.entered:
	case err := <-finished:
		joined = true
		t.Fatal("original submission completed before actual receiving CLOSE gate", err)
	case <-t.Context().Done():
		t.Fatal("receiving CLOSE was not observed", t.Context().Err())
	}
	timer := time.NewTimer(time.Until(expiry.Add(250 * time.Millisecond)))
	defer timer.Stop()
	select {
	case <-timer.C:
		expired = true
	case <-t.Context().Done():
		t.Fatal("original capsule expiry wait interrupted", t.Context().Err())
	}
	gate.release()
	err = <-finished
	joined = true
	if err == nil || !publicationDeliveryExpiryOnly(err) {
		t.Fatal("original failed RESULT/CLOSE lost its expiry or retained foreign failure", err)
	}
	if _, err := f.current(); err != nil {
		t.Fatal("Network ended instead of original predecessor delivery", err)
	}
	// All original private pending users have expired. New valid bytes and the
	// independent current recipient isolate accepted history from old wire state.
	freshExpiry := time.Now().Add(8 * time.Second).UTC().Truncate(time.Second)
	header.Slot, header.Revision, header.Expiry = current.Introduction.Slot, current.Introduction.Revision, freshExpiry
	request.Revision, request.Deadline = current.Introduction.Revision, freshExpiry
	request.WorkSafetyNotAfter, request.WorkSafetyMaximum, request.NoNewRecoveryAfter = freshExpiry.Unix(), freshExpiry.Unix(), freshExpiry.Unix()
	fresh, _, err := capsule.Seal(header, current.Introduction.RecipientKey, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := control.submitCapsule(t.Context(), recipient, fresh.Bytes()); !submissionRefusalOnly(err) {
		t.Fatal("accepted nonce lost after actual predecessor RESULT/CLOSE failure", err)
	}
	if _, err := rand.Read(header.DeliveryNonce[:]); err != nil {
		t.Fatal(err)
	}
	positive, _, err := capsule.Seal(header, current.Introduction.RecipientKey, request)
	if err != nil {
		t.Fatal(err)
	}
	if err := control.submitCapsule(t.Context(), recipient, positive.Bytes()); err != nil {
		t.Fatal("new nonce failed through acknowledged current recipient", err)
	}
}

func installedPublisherNetwork(t *testing.T, carrier transport.CarrierProfile) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	return publicationRouteNetwork(t, carrier)
}

func installedPublisherReceivers(t *testing.T, f *networkAdmissionFixture, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate, store *reachability.Store, descriptorCurrent ...func() (network.RuntimeView, error)) []*receiver.Receiver {
	return installedPublisherReceiversChecked(t, f, reservations, certificates, store, nil, descriptorCurrent...)
}

func installedPublisherReceiversChecked(t *testing.T, f *networkAdmissionFixture, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate, store *reachability.Store, checkRetirement func(error) bool, descriptorCurrent ...func() (network.RuntimeView, error)) []*receiver.Receiver {
	var servers []*receiver.Receiver
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}, {21}, {22}, {23}, {24}, {25}, {31}, {32}, {33}, {34}, {41}} {
		view, err := f.current()
		if err != nil {
			t.Fatal(err)
		}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		duty, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		binding := receiving.Receiver{NetworkID: f.profile.NetworkID, StateGeneration: f.profile.StateGeneration, StateDigest: f.profile.StateDigest, ProfileDigest: f.profile.Digest, NodeID: id, DutyGeneration: 9}
		owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return f.authority.receiver(binding, member.NotAfter()) })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := owner.Close(); err != nil {
				t.Error(err)
			}
		})
		budget := networkTestBudget(t)
		config := receiver.ReceiverConfig{Authority: role.Authority{Current: f.current, Duty: duty, Profile: view.Profile().ProfileBinding}, Certificate: certificates[id]}
		if id == [32]byte{17} {
			config.DescriptorStore = store
			if len(descriptorCurrent) != 0 {
				config.Authority.Current = descriptorCurrent[0]
			}
		}
		if id == [32]byte{25} {
			config.IntroductionRoot = t.TempDir()
			if err := os.Chmod(config.IntroductionRoot, 0700); err != nil {
				t.Fatal(err)
			}
			config.Receiving = owner
		}
		config.Admit = func(ctx context.Context, channel receiver.Channel, raw []byte) (receiving.Grant, error) {
			class, err := role.AdmissionClass(channel.Hello.Purpose)
			if err != nil {
				return receiving.Grant{}, err
			}
			return owner.Accept(ctx, class, raw, channel.Hello.Deadline, func() (func() error, error) {
				release, err := networkTestReservation(t, budget, channel.Hello.Deadline)
				if err != nil {
					return nil, err
				}
				return channel.HoldReservation(release)
			})
		}
		reservations[id]()
		server, err := receiver.Listen(t.Context(), config)
		if err != nil {
			t.Fatal(err)
		}
		servers = append(servers, server)
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				if checkRetirement == nil || !checkRetirement(err) {
					t.Error(err)
				} else {
					t.Log("retained original failed receiving result", err)
				}
			}
		})
	}
	return servers
}

func installedPublisherStock(t *testing.T, f *networkAdmissionFixture, send func(any) localAdmissionReply) {
	routeConsolePermissionStock(t, f, send, [3]uint32{2, 12, 2})
	installedPublisherBatches(t, f, send)
}

func installedPublisherBatches(t *testing.T, f *networkAdmissionFixture, send func(any) localAdmissionReply) {
	var forwards, registrations, controls, responders []token.ClosedTokenContext
	challenge := func(id byte, class uint8) token.ClosedTokenContext {
		return token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9, Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)}
	}
	for _, id := range []byte{12, 13, 14, 15, 21, 22, 23, 24} {
		forwards = append(forwards, challenge(id, 2))
	}
	for range 2 {
		registrations = append(registrations, challenge(25, 3))
		controls = append(controls, challenge(17, 1))
	}
	for _, id := range []byte{31, 32, 33, 34} {
		responders = append(responders, challenge(id, 2))
	}
	for i, challenges := range [][]token.ClosedTokenContext{forwards, registrations, controls, responders} {
		// The two finite bootstrap batches remain class-specific. Independent
		// offline fixture issuance supplies the later admitted Control batch;
		// quota kind is not a claim that a network issuing channel ran here.
		bootstrap, kind := i < 2, quota.Admitted
		if bootstrap {
			kind = quota.Bootstrap
		}
		batch := send(holderCommand{Operation: "begin", Intent: stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(51 + i)}, ProfileDigest: f.profile.Digest}, Bootstrap: bootstrap, Deadline: f.profile.NotAfter}})
		if batch.Outcome != "completed" {
			t.Fatal("genuine Publisher blind request", i, batch.Outcome)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch.Request, kind, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if issued.Outcome != "issued-offline" {
			t.Fatal(issued)
		}
		if result := send(holderCommand{Operation: "complete", Payload: issued.Response}); result.Outcome != "completed" {
			t.Fatal("genuine Publisher Stock import", result.Outcome)
		}
	}
}

func installedPublisherInstance(t *testing.T, network [32]byte) (publicationPlan, [32]byte) {
	plan := publicationPlan{InstanceRoot: filepath.Join(t.TempDir(), "instance"), Root: filepath.Join(t.TempDir(), "publication")}
	before := time.Now().UTC().Truncate(time.Second)
	root, err := instance.Initialize(t.Context(), instance.Config{Root: plan.InstanceRoot, NetworkID: network, NotBefore: before, NotAfter: before.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	request, err := root.Request(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	defer clear(authority)
	public := authority[32:]
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public...))
	credential := append([]byte{0, 3}, public...)
	credential = append(credential, target[:]...)
	credential = append(credential, view.InstancePublic[:]...)
	for _, value := range []uint64{7, uint64(view.NotBefore), uint64(view.NotAfter)} {
		credential = binary.BigEndian.AppendUint64(credential, value)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	response := append([]byte("ardents-service-instance-response-v3\x00"), view.Commitment[:]...)
	response = append(response, credential...)
	if _, err := root.Accept(t.Context(), response); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	return plan, target
}
