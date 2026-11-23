//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	endpointpublisher "github.com/dianabuilds/ardents-network/internal/endpoint/publisher"
	endpointreader "github.com/dianabuilds/ardents-network/internal/endpoint/reader"
	"github.com/dianabuilds/ardents-network/internal/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Cold/warm latency component measurement (M2 slice 3b, #650). This binary runs
// as the installed non-root ardents-endpoint.service MainPID (the root driver
// pins a temporary qualification unit). It composes the same Network/Stock/
// Execution/Funding/Reader/Publisher owners the ordinary dual-role participant
// retains, reading the bound declarations from the sealed generation. Because
// the MainPID is this qualification binary rather than the enrolled product
// executable, it does not claim the Installation startup process binding
// (RetainSelf); the genuine worker-activation MainPID/account checks still gate
// every read. It publishes one reference text document, then measures the
// ordinary Reader composition directly:
//
//   - COLD: each attempt provisions a FRESH hour-scoped permission (external
//     approval), a fresh Stock holder and fresh Entry/Interior/Hosting roots, so
//     the read has no retained token stock, prepared prefix or destination cache
//     and must consume the cold Source bootstrap allowance (funding/source.go
//     requires BootstrapRemaining==2; the two bootstrap batches exhaust it).
//   - WARM: one retained holder/permission/selection context; a priming read
//     funds the stock, then each measured read is a new Connection 1-30 s after
//     the prior one, presenting retained tokens (bootstrap Missing is empty).
//
// Verdict: nearest-rank p50/p95/p99 over all outcomes; a non-success counts as
// infinite completion time; cold p95 <= 3 s, warm p95 <= 1 s
// (privacy-qualification.md). This is component evidence on the fixture stand,
// NOT the full 1,000+1,000 x 3-seed paced campaign and NOT whole-host privacy
// qualification. The durable presentation journal, Network authority and the
// warm context's Hosting spend floor are never reset.

const (
	latencyRootEnv    = "ARDENTS_LATENCY_ROOT"
	latencyControlEnv = "ARDENTS_LATENCY_CONTROL"
	latencyColdEnv    = "ARDENTS_LATENCY_COLD"
	latencyWarmEnv    = "ARDENTS_LATENCY_WARM"
	latencySeedEnv    = "ARDENTS_LATENCY_SEED"
)

type latencyOutcome struct {
	Kind      string  `json:"kind"`
	Index     int     `json:"index"`
	Success   bool    `json:"success"`
	ElapsedMS float64 `json:"elapsed_ms"`
	Detail    string  `json:"detail,omitempty"`
}

// latencyMS marshals a non-success infinite completion time as the JSON string
// "inf" (encoding/json cannot represent +Inf); finite values stay numbers.
type latencyMS float64

func (v latencyMS) MarshalJSON() ([]byte, error) {
	if math.IsInf(float64(v), 1) {
		return []byte(`"inf"`), nil
	}
	return json.Marshal(float64(v))
}

type latencyPercentiles struct {
	P50MS latencyMS `json:"p50_ms"`
	P95MS latencyMS `json:"p95_ms"`
	P99MS latencyMS `json:"p99_ms"`
}

type latencyVerdict struct {
	Kind        string             `json:"kind"`
	Seed        string             `json:"seed"`
	Cold        []latencyOutcome   `json:"cold"`
	Warm        []latencyOutcome   `json:"warm"`
	ColdStats   latencyPercentiles `json:"cold_stats"`
	WarmStats   latencyPercentiles `json:"warm_stats"`
	ColdP95OK   bool               `json:"cold_p95_ok"`
	WarmP95OK   bool               `json:"warm_p95_ok"`
	Observation string             `json:"observation"`
}

func TestNativeEndpointColdWarmLatencyParticipant(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: cold/warm latency requires the non-root Endpoint MainPID")
	}
	root := os.Getenv(latencyRootEnv)
	control := os.Getenv(latencyControlEnv)
	if !filepath.IsAbs(root) || !filepath.IsAbs(control) {
		t.Fatal("invalid environment: absolute installed root and control directory required")
	}
	coldCount, warmCount, seed := latencyCounts(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()

	participant, err := openLatencyParticipant(ctx, root)
	if err != nil {
		t.Fatalf("cold/warm participant setup: %v", err)
	}
	defer func() {
		if err := participant.close(); err != nil {
			t.Errorf("cold/warm participant cleanup: %v", err)
		}
	}()
	link, err := participant.publish(ctx, []byte("cold/warm latency reference document\n"))
	if err != nil {
		t.Fatalf("cold/warm reference publication: %v", err)
	}
	// Let the committed publication's Serve loop and Introduction registration
	// settle before the first read; a read racing the Serve startup otherwise
	// sees an interrupted response and trips the publication failure cancel.
	latencyPause(ctx, 3*time.Second)
	verdict := latencyVerdict{Kind: "cold-warm-component", Seed: seed,
		Observation: "bounded component measurement on the fixture stand exercising the verdict logic; not the full 1,000+1,000 x 3-seed paced campaign, not the normal-reference bound qualification, and not whole-host privacy qualification. The reader open phase (per-read confined worker activation plus Source prefix setup and retained-token presentation) dominates the elapsed time on this stand; any 3 s / 1 s exceedance is reported as a finding for the separate campaign and open-phase latency investigation."}
	verdict.Warm = participant.warmLoop(ctx, t, link, warmCount)
	if os.Getenv("ARDENTS_LATENCY_OPEN_TIMING") == "1" {
		participant.diagnosticOpenTiming(ctx, t, link)
	}
	verdict.Cold = participant.coldLoop(ctx, t, link, coldCount, control)
	verdict.ColdStats = latencyStats(verdict.Cold)
	verdict.WarmStats = latencyStats(verdict.Warm)
	verdict.ColdP95OK = verdict.ColdStats.P95MS <= 3000
	verdict.WarmP95OK = verdict.WarmStats.P95MS <= 1000
	raw, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "result.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("cold/warm verdict: cold p95=%.1fms warm p95=%.1fms", verdict.ColdStats.P95MS, verdict.WarmStats.P95MS)
}

func latencyCounts(t *testing.T) (cold, warm int, seed string) {
	t.Helper()
	seed = os.Getenv(latencySeedEnv)
	if seed == "" {
		t.Fatal("invalid environment: declared seed required")
	}
	for _, item := range []struct {
		env  string
		dest *int
	}{{latencyColdEnv, &cold}, {latencyWarmEnv, &warm}} {
		value := os.Getenv(item.env)
		if value == "" {
			t.Fatal("invalid environment: attempt counts required")
		}
		if _, err := fmt.Sscanf(value, "%d", item.dest); err != nil || *item.dest < 1 || *item.dest > 1000 {
			t.Fatal("invalid environment: attempt count out of bound", value)
		}
	}
	return cold, warm, seed
}

// latencyParticipant retains the original dual-role setup. It mirrors Run's
// owner composition exactly; only the socket-serving phase is replaced by the
// bounded measurement loop. The Reader borrows these owners and is not closed
// separately; close joins Stock, Parents, Network and Execution.
// latencyNetworkOwner borrows the opened Network state through its public
// observation/lifetime operations; the concrete state owner is unexported.
type latencyNetworkOwner interface {
	Refresh(context.Context) (state.Snapshot, error)
	CurrentRuntime() (network.RuntimeView, error)
	CurrentRuntimeWitness() (state.RuntimeWitness, error)
	Close() error
}

type latencyParticipant struct {
	owner           *executionruntime.Owner
	network         latencyNetworkOwner
	routes          *funding.Parents
	publisher       *endpointpublisher.Publisher
	reader          *endpointreader.Reader
	readerHolder    *stock.Owner
	pubHolder       *stock.Owner
	readerPrep      *executionruntime.Preparation
	pubPrep         *executionruntime.Preparation
	plan            HeadlessPlan
	authority       funding.Authority
	readerPlan      funding.Plan
	readerPrincipal [32]byte
	cancel          context.CancelFunc
}

func openLatencyParticipant(ctx context.Context, root string) (*latencyParticipant, error) {
	ctx, cancel := context.WithCancel(ctx)
	p := &latencyParticipant{cancel: cancel}
	fail := func(err error) (*latencyParticipant, error) {
		_ = p.close()
		return nil, err
	}
	// The qualification MainPID is this test binary, not the enrolled product
	// executable, so the Installation startup process binding (RetainSelf) cannot
	// be satisfied and is not claimed here. Read the bound declarations directly
	// from the sealed generation and compose the same Network/Stock/Execution/
	// Funding/Reader/Publisher owners the ordinary participant retains; the
	// genuine worker-activation MainPID/account checks still gate every read.
	headlessRaw, sourceRaw, err := latencyDeclarations(root)
	if err != nil {
		return fail(err)
	}
	plan, sourcePlan, err := Declarations(headlessRaw, sourceRaw)
	if err != nil {
		return fail(err)
	}
	p.plan = plan
	local, err := installedLocalConfig(plan)
	if err != nil {
		return fail(err)
	}
	p.readerPrincipal = local.Grants[0].Principal
	owner, err := executionruntime.New(local)
	if err != nil {
		return fail(err)
	}
	p.owner = owner
	readerPrep, err := owner.Prepare(ctx, local.Grants[0].Principal, execution.Connection)
	if err != nil {
		return fail(err)
	}
	p.readerPrep = readerPrep
	check := func() error { return errors.Join(readerPrep.Check(), ctx.Err()) }
	pubPrep, err := owner.Prepare(ctx, local.Grants[1].Principal, execution.Administration)
	if err != nil {
		return fail(err)
	}
	p.pubPrep = pubPrep
	readerCheck := check
	check = func() error { return errors.Join(readerCheck(), pubPrep.Check()) }
	if err := check(); err != nil {
		return fail(err)
	}
	config, err := NetworkConfig(plan, sourcePlan, check)
	if err != nil {
		return fail(err)
	}
	config.PermitWork = func() error { return ctx.Err() }
	networkOwner, err := state.OpenRetained(config)
	if err != nil {
		return fail(err)
	}
	p.network = networkOwner
	if _, err := networkOwner.Refresh(ctx); err != nil {
		return fail(err)
	}
	authority := funding.Observe(networkOwner.CurrentRuntime, networkOwner.CurrentRuntimeWitness, presentationGuard(networkOwner.CurrentRuntime))
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := authority.Current()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	preparationCheck := check
	check = func() error {
		if err := preparationCheck(); err != nil {
			return err
		}
		_, _, err := observe()
		return err
	}
	// Publisher holder: one retained hour-scoped permission for the reference
	// publication, approved externally through the original handover paths.
	pubHolder, err := openParticipantStock(filepath.Join(plan.ApplicationTokenRoot, "publisher"), admission.AllocationPublisher, observe, check)
	if err != nil {
		return fail(err)
	}
	p.pubHolder = pubHolder
	if err := latencyFundPermission(ctx, pubHolder, plan.PublisherPermission, check); err != nil {
		return fail(err)
	}
	// Reader holder: the retained warm context. Its permission is approved once;
	// the priming read consumes the cold bootstrap allowance, later warm reads
	// present retained tokens.
	readerHolder, err := openParticipantStock(filepath.Join(plan.ApplicationTokenRoot, "reader"), admission.AllocationUser, observe, check)
	if err != nil {
		return fail(err)
	}
	p.readerHolder = readerHolder
	if err := latencyFundPermission(ctx, readerHolder, plan.ReaderPermission, check); err != nil {
		return fail(err)
	}
	if err := readerPrep.Close(); err != nil {
		return fail(err)
	}
	p.readerPrep = nil
	if err := pubPrep.Close(); err != nil {
		return fail(err)
	}
	p.pubPrep = nil
	current := func() error { return ctx.Err() }
	boundary := func() error { return ctx.Err() }
	originalCurrent := authority.Current
	serviceCurrent := func() (network.RuntimeView, error) {
		if err := ctx.Err(); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := originalCurrent()
		return view, errors.Join(err, current())
	}
	serviceAuthority := funding.Observe(serviceCurrent, networkOwner.CurrentRuntimeWitness, presentationGuard(serviceCurrent))
	p.authority = serviceAuthority
	readerPlan := funding.Plan{EntryRoot: filepath.Join(plan.EntryStateRoot, "entry"), InteriorRoot: filepath.Join(plan.EntryStateRoot, "reader-interior"),
		HostingRoot: filepath.Join(plan.EntryStateRoot, "hosting"), Domain: 1,
		Work:        hosting.Traffic{Tx: admission.ForwardClass.ByteLimit(), Rx: admission.ForwardClass.ByteLimit()},
		Termination: hosting.Traffic{Tx: admission.ControlClass.ByteLimit(), Rx: admission.ControlClass.ByteLimit()}}
	p.readerPlan = readerPlan
	routes, err := funding.OpenParents(readerPlan, serviceAuthority)
	if err != nil {
		return fail(err)
	}
	p.routes = routes
	publisherPlan := readerPlan
	publisherPlan.Domain = 4
	publisherPlan.InteriorRoot = filepath.Join(plan.EntryStateRoot, "introduction-interior")
	publisherPlan.SourceInteriorRoot = filepath.Join(plan.EntryStateRoot, "publisher-source-interior")
	publisherPlan.ResponderInteriorRoot = filepath.Join(plan.EntryStateRoot, "responder-interior")
	p.publisher = endpointpublisher.New(endpointpublisher.Config{Context: ctx,
		Local: owner, Principal: local.Grants[1].Principal,
		Plan:  endpointpublisher.Plan{InstanceRoot: plan.ServiceInstanceRoot, Root: plan.PublicationRoot},
		Route: publisherPlan, Parents: routes, Authority: serviceAuthority,
		Holder: pubHolder, Check: current, Boundary: boundary, Failed: cancel})
	p.reader = endpointreader.New(endpointreader.Config{Local: owner, Principal: local.Grants[0].Principal,
		Authority: serviceAuthority, Holder: readerHolder, Check: current, Boundary: boundary, Plan: readerPlan, Parents: routes})
	if err := current(); err != nil {
		return fail(err)
	}
	return p, nil
}

func (p *latencyParticipant) publish(ctx context.Context, body []byte) (string, error) {
	if err := p.publisher.PublishSnapshot(ctx, body); err != nil {
		return "", err
	}
	return p.publisher.PublishedLink(ctx)
}

func (p *latencyParticipant) close() error {
	var result error
	if p.publisher != nil {
		result = errors.Join(result, p.publisher.Close())
	}
	if p.routes != nil {
		result = errors.Join(result, p.routes.Close())
	}
	if p.readerHolder != nil {
		result = errors.Join(result, p.readerHolder.Close())
	}
	if p.pubHolder != nil {
		result = errors.Join(result, p.pubHolder.Close())
	}
	if p.pubPrep != nil {
		result = errors.Join(result, p.pubPrep.Close())
	}
	if p.readerPrep != nil {
		result = errors.Join(result, p.readerPrep.Close())
	}
	if p.network != nil {
		result = errors.Join(result, p.network.Close())
	}
	if p.owner != nil {
		result = errors.Join(result, p.owner.Close())
	}
	if p.cancel != nil {
		p.cancel()
	}
	return result
}

// latencyDeclarations reads the bound headless/source declarations from the
// sealed generation the provision command selected. It supplies the same plan
// bytes Startup.Plans yields, without the Installation startup process binding
// (which requires the enrolled product executable as MainPID).
func latencyDeclarations(root string) ([]byte, []byte, error) {
	selectionRaw, err := os.ReadFile(filepath.Join(root, "selection.json"))
	if err != nil {
		return nil, nil, err
	}
	var selected struct {
		GenerationDigest string `json:"generation_digest"`
	}
	if err := json.Unmarshal(selectionRaw, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		return nil, nil, errors.New("cold/warm generation selection unavailable")
	}
	generation := filepath.Join(root, "generations", selected.GenerationDigest)
	headlessRaw, err := os.ReadFile(filepath.Join(generation, "headless.json"))
	if err != nil {
		return nil, nil, err
	}
	sourceRaw, err := os.ReadFile(filepath.Join(generation, "source.json"))
	if err != nil {
		return nil, nil, err
	}
	return headlessRaw, sourceRaw, nil
}

// latencyFundPermission drives the original Request -> external approval ->
// Import handover. The signing key never enters this MainPID; the root driver
// approves the exact qualified request and writes the signed response.
func latencyFundPermission(ctx context.Context, holder *stock.Owner, permission PermissionPlan, check func() error) error {
	if err := installedPermissionRetire(permission.RequestPath, permission.ResponsePath, check); err != nil {
		return err
	}
	request, digest, err := holder.Request(permission.Maxima)
	if err != nil {
		return err
	}
	decoded, err := admission.DecodePermissionRequest(request)
	if err != nil {
		return err
	}
	if err := installedPermissionRequest(permission.RequestPath, request, check); err != nil {
		return err
	}
	return installedPermissionResponse(ctx, permission.ResponsePath, digest, decoded.Permission.NotAfter, holder, check)
}

// warmLoop measures new Connections 1-30 s after the prior read in the same
// retained context. The first (priming) read funds the stock and is not a
// measured warm sample.
func (p *latencyParticipant) warmLoop(ctx context.Context, t *testing.T, link string, count int) []latencyOutcome {
	t.Helper()
	outcomes := make([]latencyOutcome, 0, count)
	readOnce := func() (time.Duration, error) {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		began := time.Now()
		stream, err := p.reader.Open(bounded, application.Request{Destination: application.TargetLink, Value: link})
		opened := time.Since(began)
		if err != nil {
			t.Logf("warm read open refused after %v: %v", opened, err)
			return opened, err
		}
		readBegan := time.Now()
		body, readErr := document.Read(bounded, stream)
		elapsed := time.Since(began)
		t.Logf("warm read open=%v exchange=%v total=%v err=%v", opened, time.Since(readBegan), elapsed, readErr)
		clear(body)
		return elapsed, errors.Join(readErr, stream.Close())
	}
	if _, err := readOnce(); err != nil {
		// A failed priming read leaves no retained warm stock; every measured
		// warm attempt then counts as a non-success (infinite) rather than being
		// silently replaced by a fresh cold context.
		for i := 0; i < count; i++ {
			outcomes = append(outcomes, latencyOutcome{Kind: "warm", Index: i, Detail: "priming read failed: " + err.Error()})
		}
		return outcomes
	}
	status := p.readerHolder.Status()
	t.Logf("warm stock after priming: bootstrapRemaining=%d remaining=%v", status.BootstrapRemaining, status.Remaining)
	for i := 0; i < count; i++ {
		latencyPause(ctx, 1*time.Second)
		elapsed, err := readOnce()
		status := p.readerHolder.Status()
		t.Logf("warm stock after read %d: bootstrapRemaining=%d remaining=%v", i, status.BootstrapRemaining, status.Remaining)
		outcome := latencyOutcome{Kind: "warm", Index: i, Success: err == nil, ElapsedMS: float64(elapsed.Milliseconds())}
		if err != nil {
			outcome.Success = false
			outcome.Detail = err.Error()
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

// coldLoop measures each attempt from a fresh permission/holder/selection/hosting
// context. Only the permitted volatile cold-test state is fresh; the durable
// Network authority and the warm context's floors are untouched.
func (p *latencyParticipant) coldLoop(ctx context.Context, t *testing.T, link string, count int, control string) []latencyOutcome {
	t.Helper()
	outcomes := make([]latencyOutcome, 0, count)
	for i := 0; i < count; i++ {
		// Let the shared Introduction slot drain between fresh-context attempts;
		// back-to-back cold submissions can otherwise outrun its finite delivery.
		latencyPause(ctx, 2*time.Second)
		outcome := latencyOutcome{Kind: "cold", Index: i}
		elapsed, err := p.coldAttempt(ctx, link, i, control)
		outcome.ElapsedMS = float64(elapsed.Milliseconds())
		if err != nil {
			outcome.Detail = err.Error()
		} else {
			outcome.Success = true
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

// coldAttempt measures one read from a wholly fresh cold context: a FRESH
// hour-scoped permission (external approval), a FRESH Stock holder with no
// retained token stock, and FRESH Entry/Interior selection and Hosting roots, so
// the read has no private destination cache, token stock or prepared prefix and
// must consume the cold Source bootstrap allowance. This is the code-derived
// cold boundary: funding/source.go requires BootstrapRemaining==2, which
// only a never-bootstrapped permission satisfies. The durable Network authority
// and the warm context's floors are untouched; each cold attempt is an
// independent fresh context (its own journal and provider period), not a reset
// of a shared ledger.
func (p *latencyParticipant) coldAttempt(ctx context.Context, link string, index int, control string) (time.Duration, error) {
	base := filepath.Join(control, fmt.Sprintf("cold-%d", index))
	// hosting.Initialize creates its own period directory and refuses a
	// pre-existing one, so only the other fresh roots are prepared here.
	for _, dir := range []string{"stock", "entry", "interior", "permission"} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0700); err != nil {
			return 0, err
		}
	}
	period := time.Now().UTC().Truncate(time.Second)
	if err := hosting.Initialize(filepath.Join(base, "hosting"), hosting.Policy{Provider: "cold-warm-latency-fixture",
		Start: period.Add(-time.Minute), End: period.Add(time.Hour), Unit: "GiB", Quantity: 16,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}); err != nil {
		return 0, fmt.Errorf("cold hosting initialize: %w", err)
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := p.network.CurrentRuntime()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	holder, err := stock.Open(filepath.Join(base, "stock"), admission.AllocationUser, observe)
	if err != nil {
		return 0, fmt.Errorf("cold stock open: %w", err)
	}
	defer holder.Close()
	permission := PermissionPlan{RequestPath: filepath.Join(base, "permission", "request"),
		ResponsePath: filepath.Join(base, "permission", "response"), Maxima: p.plan.ReaderPermission.Maxima}
	check := func() error { return ctx.Err() }
	if err := latencyFundPermission(ctx, holder, permission, check); err != nil {
		return 0, err
	}
	plan := p.readerPlan
	plan.EntryRoot = filepath.Join(base, "entry")
	plan.InteriorRoot = filepath.Join(base, "interior")
	plan.HostingRoot = filepath.Join(base, "hosting")
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	reader := endpointreader.New(endpointreader.Config{
		Local: p.owner, Principal: p.readerPrincipal, Plan: plan,
		Authority: p.authority, Holder: holder, Check: bounded.Err,
	})
	began := time.Now()
	stream, err := reader.Open(bounded, application.Request{Destination: application.TargetLink, Value: link})
	if err != nil {
		return time.Since(began), err
	}
	body, readErr := document.Read(bounded, stream)
	elapsed := time.Since(began)
	clear(body)
	return elapsed, errors.Join(readErr, stream.Close())
}

// diagnosticOpenTiming performs one extra retained-context read with a logging
// authority observer so the receipt can attribute the open phase: each Current
// observation's duration and the gap since the previous one bracket the TLS
// dial, token presentation and receiver admission that run between observations.
func (p *latencyParticipant) diagnosticOpenTiming(ctx context.Context, t *testing.T, link string) {
	t.Helper()
	originalCurrent := p.authority.Current
	var last time.Time
	calls := 0
	loggingCurrent := func() (network.RuntimeView, error) {
		began := time.Now()
		if !last.IsZero() {
			t.Logf("open-timing gap-before-current=%v", began.Sub(last))
		}
		view, err := originalCurrent()
		calls++
		t.Logf("open-timing current#%d took=%v err=%v", calls, time.Since(began), err)
		last = time.Now()
		return view, err
	}
	authority := funding.Observe(loggingCurrent, p.authority.Witness, p.authority.Presentation)
	reader := endpointreader.New(endpointreader.Config{Local: p.owner, Principal: p.readerPrincipal,
		Authority: authority, Holder: p.readerHolder, Check: func() error { return ctx.Err() },
		Boundary: func() error { return ctx.Err() }, Plan: p.readerPlan, Parents: p.routes})
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	began := time.Now()
	stream, err := reader.Open(bounded, application.Request{Destination: application.TargetLink, Value: link})
	opened := time.Since(began)
	if err != nil {
		t.Logf("open-timing open refused after %v: %v", opened, err)
		return
	}
	readBegan := time.Now()
	body, readErr := document.Read(bounded, stream)
	t.Logf("open-timing open=%v exchange=%v total=%v currents=%d err=%v", opened, time.Since(readBegan), time.Since(began), calls, readErr)
	clear(body)
	_ = stream.Close()
}

// latencyPause holds the bounded 1-30 s warm spacing. The short validation uses
// the lower bound; the full campaign draws its spacing from the declared seed.
func latencyPause(ctx context.Context, spacing time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(spacing):
	}
}

func latencyStats(outcomes []latencyOutcome) latencyPercentiles {
	if len(outcomes) == 0 {
		return latencyPercentiles{P50MS: latencyMS(math.Inf(1)), P95MS: latencyMS(math.Inf(1)), P99MS: latencyMS(math.Inf(1))}
	}
	elapsed := make([]float64, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.Success {
			elapsed = append(elapsed, outcome.ElapsedMS)
		} else {
			elapsed = append(elapsed, math.Inf(1))
		}
	}
	sort.Float64s(elapsed)
	rank := func(percent int) float64 {
		index := (percent*len(elapsed)+99)/100 - 1
		if index < 0 {
			index = 0
		}
		return elapsed[index]
	}
	return latencyPercentiles{P50MS: latencyMS(rank(50)), P95MS: latencyMS(rank(95)), P99MS: latencyMS(rank(99))}
}
