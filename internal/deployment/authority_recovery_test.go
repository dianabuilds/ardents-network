package deployment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const recoveryRealmID = "r1_00112233445566778899aabbccddeeff"

type recoveryProbeFixture struct {
	nodes      map[string]*recoveryNodeFixture
	opened     []string
	closed     []string
	openErr    map[string]error
	verifySeen AuthorityRecoveryAcknowledgement
}

func (fixture *recoveryProbeFixture) Open(
	_ context.Context,
	target AuthorityRecoveryTarget,
) (AuthorityRecoveryNode, error) {
	fixture.opened = append(fixture.opened, target.Slot)
	if err := fixture.openErr[target.Slot]; err != nil {
		return nil, err
	}
	node := fixture.nodes[target.Slot]
	node.target = target
	node.close = func() { fixture.closed = append(fixture.closed, target.Slot) }
	node.verifySeen = &fixture.verifySeen
	return node, nil
}

type recoveryNodeFixture struct {
	target      AuthorityRecoveryTarget
	clock       ClockObservation
	authority   AuthorityObservation
	verified    AuthorityObservation
	clockErr    error
	inspectErr  error
	verifyErr   error
	verifyCalls int
	close       func()
	closeErr    error
	verifySeen  *AuthorityRecoveryAcknowledgement
}

func (node *recoveryNodeFixture) ObserveClock(context.Context) (ClockObservation, error) {
	return node.clock, node.clockErr
}

func (node *recoveryNodeFixture) InspectAuthority(
	context.Context,
) (AuthorityObservation, error) {
	return node.authority, node.inspectErr
}

func (node *recoveryNodeFixture) VerifyRestoredAuthority(
	_ context.Context,
	ack AuthorityRecoveryAcknowledgement,
) (AuthorityObservation, error) {
	node.verifyCalls++
	if node.verifySeen != nil {
		*node.verifySeen = ack
	}
	return node.verified, node.verifyErr
}

func (node *recoveryNodeFixture) Close(context.Context) error {
	if node.close != nil {
		node.close()
	}
	return node.closeErr
}

func TestAuthorityRecoveryVerifiesExactObservedCheckpointAfterClockPreflight(t *testing.T) {
	raw := recoveryManifest(t)
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	authority := fixture.nodes["node-a"]
	authority.authority = AuthorityObservation{
		RealmID: recoveryRealmID, AuthoritySequence: 42,
		CheckpointDigest: "ac1_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Phase:            "recovery_only", Readiness: "degraded",
		Reason: "authority_restore_verification_required",
	}
	authority.verified = AuthorityObservation{
		RealmID: authority.authority.RealmID, AuthoritySequence: authority.authority.AuthoritySequence,
		CheckpointDigest: authority.authority.CheckpointDigest,
		Phase:            "ready", Readiness: "ready",
	}

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(context.Background(), raw)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeVerified, status.Outcome)
	require.Equal(t, "node-a", status.Slot)
	require.Equal(t, "ready", status.Readiness)
	require.Equal(t, "ready", status.Phase)
	require.Empty(t, status.Reason)
	require.Equal(t, []string{"node-b", "node-c", "node-a"}, fixture.opened)
	require.ElementsMatch(t, fixture.opened, fixture.closed)
	require.Equal(t, AuthorityRecoveryAcknowledgement{
		RealmID: authority.authority.RealmID, AuthoritySequence: authority.authority.AuthoritySequence,
		CheckpointDigest: authority.authority.CheckpointDigest,
	}, fixture.verifySeen)
}

func TestAuthorityRecoveryAlreadyReadyIsANoOp(t *testing.T) {
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	authority := fixture.nodes["node-a"]
	authority.authority = AuthorityObservation{
		RealmID: recoveryRealmID, AuthoritySequence: 7,
		CheckpointDigest: "ac1_abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Phase:            "ready", Readiness: "ready",
	}

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
		context.Background(), recoveryManifest(t),
	)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeAlreadyReady, status.Outcome)
	require.Zero(t, authority.verifyCalls)
}

func TestAuthorityRecoveryFailsClosedBeforeAuthorityCallOnExcessiveClockSkew(t *testing.T) {
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	fixture.nodes["node-c"].clock.ServerObservedAt =
		fixture.nodes["node-c"].clock.ServerObservedAt.Add(31 * time.Second)

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
		context.Background(), recoveryManifest(t),
	)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, AuthorityRecoveryReasonClockSkew, status.Reason)
	require.Zero(t, fixture.nodes["node-a"].verifyCalls)
	require.Empty(t, fixture.nodes["node-a"].authority.RealmID)
}

func TestAuthorityRecoveryRejectsChangedVerificationTruth(t *testing.T) {
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	authority := fixture.nodes["node-a"]
	authority.authority = AuthorityObservation{
		RealmID: recoveryRealmID, AuthoritySequence: 42,
		CheckpointDigest: "ac1_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Phase:            "recovery_only", Readiness: "degraded",
		Reason: "authority_restore_verification_required",
	}
	authority.verified = authority.authority
	authority.verified.AuthoritySequence++
	authority.verified.Phase = "ready"
	authority.verified.Readiness = "ready"

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
		context.Background(), recoveryManifest(t),
	)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, AuthorityRecoveryReasonCheckpointMismatch, status.Reason)
}

func TestAuthorityRecoveryTreatsFailedSessionCleanupAsRecoveryRequired(t *testing.T) {
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	authority := fixture.nodes["node-a"]
	authority.authority = AuthorityObservation{
		RealmID: recoveryRealmID, AuthoritySequence: 7,
		CheckpointDigest: "ac1_abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Phase:            "ready", Readiness: "ready",
	}
	authority.closeErr = ProbeError(ProbeTunnelTimeout)

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
		context.Background(), recoveryManifest(t),
	)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, AuthorityRecoveryReason(ProbeTunnelTimeout), status.Reason)
}

func TestAuthorityRecoveryMapsUnavailableClockWithoutLeakingError(t *testing.T) {
	fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
	fixture.nodes["node-b"].clockErr = errors.New("operator@example /secret/path")

	status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
		context.Background(), recoveryManifest(t),
	)

	require.NoError(t, err)
	require.Equal(t, AuthorityRecoveryOutcomeRecoveryRequired, status.Outcome)
	require.Equal(t, AuthorityRecoveryReasonClockUnavailable, status.Reason)
}

func TestAuthorityRecoveryPreservesStableAuthorityFailureOwnership(t *testing.T) {
	tests := []struct {
		remote string
		want   AuthorityRecoveryReason
	}{
		{"checkpoint_repository_unavailable", AuthorityRecoveryReasonRepositoryUnavailable},
		{"checkpoint_head_missing", AuthorityRecoveryReasonCheckpointMissing},
		{"checkpoint_head_mismatch", AuthorityRecoveryReasonCheckpointHeadMismatch},
		{"authority_state_invalid", AuthorityRecoveryReasonPersistedStateInvalid},
		{"authority_signer_mismatch", AuthorityRecoveryReasonSignerMismatch},
	}
	for _, test := range tests {
		t.Run(test.remote, func(t *testing.T) {
			fixture := readyRecoveryProbe(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC))
			fixture.nodes["node-a"].authority = AuthorityObservation{
				RealmID: recoveryRealmID, AuthoritySequence: 7,
				CheckpointDigest: "ac1_abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
				Phase:            "recovery_required", Readiness: "recovery_required", Reason: test.remote,
			}

			status, err := (AuthorityRecoveryInspector{Probe: fixture}).Recover(
				context.Background(), recoveryManifest(t),
			)

			require.NoError(t, err)
			require.Equal(t, AuthorityRecoveryOutcomeRecoveryRequired, status.Outcome)
			require.Equal(t, test.want, status.Reason)
		})
	}
}

func TestAuthorityRolloutOrderKeepsAuthorityLastOrFirst(t *testing.T) {
	raw := recoveryManifest(t)

	compatible, err := AuthorityRolloutOrder(raw, AuthorityChangeCompatible)
	require.NoError(t, err)
	require.Equal(t, []string{"node-b", "node-c", "node-a"}, compatible)

	migration, err := AuthorityRolloutOrder(raw, AuthorityChangeMigration)
	require.NoError(t, err)
	require.Equal(t, []string{"node-a", "node-b", "node-c"}, migration)

	_, err = AuthorityRolloutOrder(raw, "operator_guess")
	require.EqualError(t, err, "topology_invalid_authority_change_kind")
}

func readyRecoveryProbe(now time.Time) *recoveryProbeFixture {
	nodes := map[string]*recoveryNodeFixture{}
	for _, slot := range []string{"node-a", "node-b", "node-c"} {
		nodes[slot] = &recoveryNodeFixture{clock: ClockObservation{
			RequestStarted:   now.Add(-time.Second),
			ServerObservedAt: now,
			ResponseReceived: now.Add(time.Second),
		}}
	}
	return &recoveryProbeFixture{nodes: nodes, openErr: map[string]error{}}
}

func recoveryManifest(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "public-direct.json"))
	require.NoError(t, err)
	return raw
}
