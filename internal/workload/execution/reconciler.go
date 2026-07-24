package execution

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	db "ardents/internal/storage"
	workloadregistry "ardents/internal/workload/registry"
)

type Service struct {
	mu               sync.Mutex
	operationGate    chan struct{}
	path             string
	state            string
	executor         Executor
	admission        AdmissionFunc
	items            map[string]Status
	restartBudget    int
	now              func() time.Time
	ancillaryBackoff ancillaryBackoff
}

func New(path string, executor Executor) *Service {
	if executor == nil {
		panic("controller.New requires executor")
	}
	return &Service{
		path:          path,
		state:         "new",
		executor:      executor,
		items:         map[string]Status{},
		operationGate: make(chan struct{}, 1),
		restartBudget: DefaultRestartBudget,
		now:           time.Now,
	}
}

func NewInDir(dir string) *Service {
	return NewWithExecutorInDir(dir, NewLocalExecutor())
}

func NewWithExecutorInDir(dir string, executor Executor) *Service {
	return New(db.PathInDir(dir), executor)
}

func (s *Service) SetAdmission(fn AdmissionFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admission = fn
}

func (s *Service) Load() error {
	return s.LoadContext(context.Background())
}

func (s *Service) LoadContext(ctx context.Context) error {
	release, err := s.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()

	var loaded map[string]Status
	if s.path == "" {
		loaded = map[string]Status{}
	} else {
		var stored persistentState
		found, loadErr := db.LoadJSONStrict(s.path, "workload", "snapshot", &stored)
		if loadErr != nil {
			return loadErr
		}
		if found {
			loaded, loadErr = validatePersistentState(stored)
			if loadErr != nil {
				return loadErr
			}
		} else {
			loaded = map[string]Status{}
		}
	}
	loaded, err = s.recoverLoadedProcesses(ctx, loaded)
	if err != nil {
		return err
	}
	if err := s.reconcileRuntimeInventory(ctx, loaded); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = loaded
	s.state = "ready"
	return nil
}

func validatePersistentState(stored persistentState) (map[string]Status, error) {
	if stored.Version != persistentStateVersion {
		return nil, fmt.Errorf("unsupported workload state version")
	}
	if stored.Items == nil {
		return nil, fmt.Errorf("workload state items are missing")
	}
	items := make(map[string]Status, len(stored.Items))
	for id, item := range stored.Items {
		if id == "" || item.Spec.ID != id {
			return nil, fmt.Errorf("workload state key does not match workload id")
		}
		if err := workloadregistry.ValidateSpec(item.Spec); err != nil {
			return nil, fmt.Errorf("workload state spec is invalid: %w", err)
		}
		items[id] = NormalizeStatus(item)
	}
	return items, nil
}

func (s *Service) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Service) Seed(specs []workloadregistry.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, spec := range specs {
		spec = workloadregistry.NormalizeSpec(spec)
		if spec.ID == "" || spec.Kind == "" {
			continue
		}
		if err := workloadregistry.ValidateWorkloadRequirements(spec.Requirements); err != nil {
			return err
		}
		if _, ok := s.items[spec.ID]; ok {
			continue
		}
		s.items[spec.ID] = Status{
			Spec:              spec,
			Observed:          ObservedAccepted,
			LastTransitionAt:  time.Now().UTC(),
			PublishedServices: ServiceStatuses(spec, false, "workload not running"),
		}
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) Register(spec workloadregistry.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec = workloadregistry.NormalizeSpec(spec)
	if spec.ID == "" || spec.Kind == "" {
		return fmt.Errorf("workload id and kind are required")
	}
	if err := workloadregistry.ValidateWorkloadRequirements(spec.Requirements); err != nil {
		return err
	}
	if _, exists := s.items[spec.ID]; exists {
		return fmt.Errorf("workload %s already exists", spec.ID)
	}
	s.items[spec.ID] = Status{
		Spec:              spec,
		Observed:          ObservedAccepted,
		LastTransitionAt:  time.Now().UTC(),
		PublishedServices: ServiceStatuses(spec, false, "workload not running"),
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) Get(id string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return Status{}, false
	}
	return CloneStatus(item), true
}

func (s *Service) SetDesired(id, desired string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("workload %s not found", id)
	}
	nextDesired := workloadregistry.NormalizeDesired(desired)
	if item.Spec.Desired != workloadregistry.DesiredRunning && nextDesired == workloadregistry.DesiredRunning {
		item.RestartCount = 0
		item.NeedsOperatorAction = false
		item.Reason = ""
	}
	item.Spec.Desired = nextDesired
	s.items[id] = NormalizeStatus(item)
	return s.saveLocked()
}

func (s *Service) Reconcile(ctx context.Context) error {
	release, err := s.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()

	s.mu.Lock()
	workCopies, comparisonSnapshots := s.captureStatusesLocked()
	admission := s.admission
	s.mu.Unlock()
	now := time.Now().UTC()
	nextItems := make(map[string]Status, len(workCopies))
	keepItems := make(map[string]bool, len(workCopies))
	admissionStatuses := SnapshotStatuses(workCopies)
	for id, item := range workCopies {
		next, keep, err := s.reconcileLocked(ctx, item, now, admission, admissionStatuses)
		if err != nil {
			return err
		}
		nextItems[id] = next
		keepItems[id] = keep
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range workCopies {
		current, found := s.items[id]
		if !found || !reflect.DeepEqual(current, comparisonSnapshots[id]) {
			continue
		}
		if keepItems[id] {
			s.items[id] = nextItems[id]
		} else {
			delete(s.items, id)
		}
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) acquireOperation(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case s.operationGate <- struct{}{}:
		return func() { <-s.operationGate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) List() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.items))
	for id := range s.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]Status, 0, len(ids))
	for _, id := range ids {
		out = append(out, CloneStatus(s.items[id]))
	}
	return out
}

func (s *Service) Published() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Status, 0)
	for _, item := range s.items {
		if item.Observed != ObservedRunning || len(item.PublishedServices) == 0 {
			continue
		}
		out = append(out, CloneStatus(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Spec.ID < out[j].Spec.ID
	})
	return out
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) Desired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.items {
		switch item.Spec.Desired {
		case workloadregistry.DesiredPresent, workloadregistry.DesiredRunning, workloadregistry.DesiredStopped, workloadregistry.DesiredDisabled:
			count++
		}
	}
	return count
}

func (s *Service) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.items {
		if item.Observed == ObservedRunning {
			count++
		}
	}
	return count
}
