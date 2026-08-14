package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type dockerProcessAdapter struct {
	scope     hostScopeEvidence
	clock     time.Time
	serviceID func(context.Context, string) (string, error)
	command   func(context.Context, time.Duration, ...string) ([]byte, error)
}

type dockerProcessInspection struct {
	observation                         processObservation
	image, path, project, service, mode string
}

type dockerProcessPublicProjection struct {
	Image, Path, Project, Service, PIDMode string
	Arguments                              []string
}

func newDockerProcessAdapter(observer dockerObserver, scope hostScopeEvidence,
	clock time.Time) dockerProcessAdapter {
	return dockerProcessAdapter{scope: scope, clock: clock,
		serviceID: observer.serviceID, command: observer.docker}
}

func (adapter dockerProcessAdapter) InjectProcessFault(ctx context.Context, ref processEvidenceRef,
	spec processFaultSpec) (processFaultReceipt, error) {
	if !adapter.validRef(ref) || spec.Kind != processStop {
		return processFaultReceipt{}, errors.New("docker process fault target or operation is invalid")
	}
	inspection, err := adapter.inspectProcess(ctx, ref.Identity)
	if err != nil {
		return processFaultReceipt{}, fmt.Errorf("confirm exact Docker process before fault: %w", err)
	}
	current := inspection.observation
	if err := validateProcessObservation(current); err != nil || bindProcessRef(current.Ref) != ref {
		return processFaultReceipt{}, errors.Join(err,
			errors.New("Docker process changed incarnation before fault"))
	}
	started := max(int64(1), time.Since(adapter.clock).Nanoseconds())
	if _, err := adapter.command(ctx, 10*time.Second, "stop", "-t", "0", ref.Identity); err != nil {
		return processFaultReceipt{}, fmt.Errorf("stop exact Docker process %q: %w", ref.Identity, err)
	}
	observed, err := adapter.AwaitProcessState(ctx, ref, processStopped, 10*time.Second)
	if err != nil {
		return processFaultReceipt{}, err
	}
	completed := max(started, time.Since(adapter.clock).Nanoseconds())
	return processFaultReceipt{Ref: observed.Ref, Kind: spec.Kind, State: observed.State,
		InvocationStartedNanos: started, InvocationCompletedNanos: completed,
		ObservedAtNanos: observed.ObservedAtNanos}, nil
}

func (adapter dockerProcessAdapter) AwaitProcessState(ctx context.Context, ref processEvidenceRef,
	wanted processState, limit time.Duration) (processStateObservation, error) {
	if !adapter.validRef(ref) || wanted != processStopped || limit <= 0 {
		return processStateObservation{}, errors.New("docker process state observation is invalid")
	}
	bounded, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	for {
		inspection, err := adapter.inspectProcess(bounded, ref.Identity)
		if err != nil {
			return processStateObservation{}, fmt.Errorf("inspect Docker process state: %w", err)
		}
		observed := inspection.observation
		if bindProcessRef(observed.Ref) != ref {
			return processStateObservation{}, errors.New("Docker process changed incarnation while awaiting state")
		}
		if !observed.Running {
			return processStateObservation{Ref: ref, State: processStopped,
				ObservedAtNanos: observed.ObservedAtNanos}, nil
		}
		select {
		case <-bounded.Done():
			return processStateObservation{}, fmt.Errorf("await stopped Docker process %q: %w",
				ref.Identity, bounded.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (adapter dockerProcessAdapter) validRef(value processEvidenceRef) bool {
	return validDockerEvidenceRef(value) && value.Scope == adapter.scope.Commitment
}

func validDockerEvidenceRef(value processEvidenceRef) bool {
	return value.Adapter == "docker-compose-v1" && value.Scope != [32]byte{} &&
		value.Executable != [32]byte{} && value.Tree != [32]byte{} &&
		validContainerID(value.Identity) && value.Incarnation != "" &&
		value.Commitment != [32]byte{} && value.Commitment == processRefCommitment(value)
}

func (adapter dockerProcessAdapter) ResolveProcess(ctx context.Context,
	selector processSelector) (processObservation, error) {
	if selector.LogicalRole == "" || selector.AdapterKey == "" {
		return processObservation{}, errors.New("Docker process selector is incomplete")
	}
	container, err := adapter.serviceID(ctx, selector.AdapterKey)
	if err != nil {
		return processObservation{}, fmt.Errorf("resolve Docker service %q: %w", selector.AdapterKey, err)
	}
	inspection, err := adapter.inspectProcess(ctx, container)
	if err != nil {
		return processObservation{}, err
	}
	if inspection.image == "" || sha256.Sum256([]byte(inspection.image)) != adapter.scope.Image ||
		inspection.path == "" || inspection.project != adapter.scope.AdapterProjection ||
		inspection.service != selector.AdapterKey || inspection.mode != "" && inspection.mode != "private" {
		return processObservation{}, errors.New("Docker process image, executable, or tree scope is invalid")
	}
	if err := validateProcessObservation(inspection.observation); err != nil {
		return processObservation{}, fmt.Errorf("resolve live Docker process %q: %w", selector.AdapterKey, err)
	}
	return inspection.observation, nil
}

func (adapter dockerProcessAdapter) inspectProcess(ctx context.Context,
	container string) (dockerProcessInspection, error) {
	raw, err := adapter.command(ctx, 10*time.Second, "inspect", "--format",
		`{"id":{{json .Id}},"pid":{{.State.Pid}},"started":{{json .State.StartedAt}},`+
			`"running":{{.State.Running}},"image":{{json .Image}},"path":{{json .Path}},"args":{{json .Args}},`+
			`"project":{{json (index .Config.Labels "com.docker.compose.project")}},`+
			`"service":{{json (index .Config.Labels "com.docker.compose.service")}},`+
			`"pid_mode":{{json .HostConfig.PidMode}}}`, container)
	if err != nil {
		return dockerProcessInspection{}, fmt.Errorf("inspect Docker process %q: %w", container, err)
	}
	if len(raw) == 0 || len(raw) > 64<<10 {
		return dockerProcessInspection{}, errors.New("Docker process inspection size is invalid")
	}
	var value struct {
		ID, Started, Image, Path, Project, Service, PIDMode string
		Args                                                []string
		PID                                                 uint32
		Running                                             bool
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return dockerProcessInspection{}, errors.Join(err, errors.New("docker process inspection is invalid"))
	}
	if len(value.ID) > 128 || len(value.Started) > 128 || len(value.Image) > 256 || len(value.Path) > 4096 ||
		len(value.Project) > 128 || len(value.Service) > 128 || len(value.PIDMode) > 32 || len(value.Args) > 64 {
		return dockerProcessInspection{}, errors.New("Docker process projection exceeds its bounds")
	}
	for _, argument := range value.Args {
		if len(argument) > 4096 {
			return dockerProcessInspection{}, errors.New("Docker process argument exceeds its bound")
		}
	}
	incarnation, err := parseProcessIdentity(container, []byte(value.ID+" "+value.Started))
	if err != nil || value.ID != container {
		return dockerProcessInspection{}, errors.Join(err, errors.New("docker process identity is not exact"))
	}
	executable := dockerProcessProjection("ardents-qualification-executable-v1\x00",
		append([]string{value.Image, value.Path}, value.Args...)...)
	tree := dockerProcessProjection("ardents-qualification-process-tree-v1\x00",
		value.Project, value.Service, value.PIDMode, container)
	observedAt := max(int64(1), time.Since(adapter.clock).Nanoseconds())
	projection, err := json.Marshal(dockerProcessPublicProjection{Image: value.Image, Path: value.Path,
		Project: value.Project, Service: value.Service, PIDMode: value.PIDMode, Arguments: value.Args})
	if err != nil {
		return dockerProcessInspection{}, fmt.Errorf("encode Docker process projection: %w", err)
	}
	observed := processObservation{Ref: processRef{Adapter: adapter.scope.Adapter, Scope: adapter.scope.Commitment,
		Executable: executable, Tree: tree, Identity: container, Incarnation: incarnation},
		AdapterProjection: projection, OSProcessID: value.PID, Running: value.Running, ObservedAtNanos: observedAt}
	return dockerProcessInspection{observation: observed, image: value.Image, path: value.Path,
		project: value.Project, service: value.Service, mode: value.PIDMode}, nil
}

func dockerProcessProjection(domain string, values ...string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}
